package job

import (
	"context"
	"fmt"
	"sync"

	"kubemin-cli/pkg/apiserver/config"
)

// cleanupContextKey is the private key used to stash cleanup tracker data in the context.
type cleanupContextKey struct{}

// resourceRef stores the namespace/name of a resource along with whether the job
// created it during the current execution.
type resourceRef struct {
	Kind      config.ResourceKind
	Namespace string
	Name      string
	Created   bool
}

// cleanupTracker accumulates resources that should be cleaned up when a job fails.
type cleanupTracker struct {
	mu        sync.Mutex
	resources []resourceRef
}

func (t *cleanupTracker) add(ref resourceRef) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Avoid duplicate entries for the same resource/kind.
	for _, existing := range t.resources {
		if existing.Kind == ref.Kind && existing.Namespace == ref.Namespace && existing.Name == ref.Name {
			return
		}
	}
	t.resources = append(t.resources, ref)
}

func (t *cleanupTracker) list(kind config.ResourceKind) []resourceRef {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []resourceRef
	for _, ref := range t.resources {
		if kind == "" || ref.Kind == kind {
			out = append(out, ref)
		}
	}
	return out
}

// WithCleanupTracker ensures the provided context carries a cleanup tracker so that
// resource creation can be recorded and later cleaned up.
func WithCleanupTracker(ctx context.Context) context.Context {
	if ctx.Value(cleanupContextKey{}) != nil {
		return ctx
	}
	return context.WithValue(ctx, cleanupContextKey{}, &cleanupTracker{})
}

func trackerFromContext(ctx context.Context) (*cleanupTracker, error) {
	raw := ctx.Value(cleanupContextKey{})
	if raw == nil {
		return nil, fmt.Errorf("cleanup tracker missing from context")
	}
	tracker, ok := raw.(*cleanupTracker)
	if !ok {
		return nil, fmt.Errorf("cleanup tracker has unexpected type %T", raw)
	}
	return tracker, nil
}

// MarkResourceCreated records that the job created the specified resource so that
// it can be deleted if execution fails.
func MarkResourceCreated(ctx context.Context, kind config.ResourceKind, namespace, name string) {
	tracker, err := trackerFromContext(ctx)
	if err != nil {
		return
	}
	tracker.add(resourceRef{Kind: kind, Namespace: namespace, Name: name, Created: true})
}

// markResourceObserved records that the job interacted with the specified resource
// without claiming ownership of its lifecycle.
func markResourceObserved(ctx context.Context, kind config.ResourceKind, namespace, name string) {
	tracker, err := trackerFromContext(ctx)
	if err != nil {
		return
	}
	tracker.add(resourceRef{Kind: kind, Namespace: namespace, Name: name, Created: false})
}

// resourcesForCleanup returns the tracked resources. When kind is empty, all resources
// are returned. Only resources marked as Created should be deleted by cleanup routines.
func resourcesForCleanup(ctx context.Context, kind config.ResourceKind) []resourceRef {
	tracker, err := trackerFromContext(ctx)
	if err != nil {
		return nil
	}
	return tracker.list(kind)
}
