package job

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/utils/cache"
)

var shareLocks sync.Map
var errLockHeld = errors.New("lock already taken")

func shareInfoFromLabels(labels map[string]string) (string, config.ShareStrategy) {
	if labels == nil {
		return "", ""
	}
	name := strings.TrimSpace(labels[config.LabelShareName])
	if name == "" {
		return "", ""
	}
	rawStrategy := strings.TrimSpace(labels[config.LabelShareStrategy])
	strategy, _ := config.NormalizeShareStrategy(rawStrategy)
	return name, strategy
}

func shareListOptions(name string) metav1.ListOptions {
	selector := labels.Set{config.LabelShareName: name}.String()
	return metav1.ListOptions{LabelSelector: selector}
}

func hasSharedResources(ctx context.Context, name string, listFn func(context.Context, metav1.ListOptions) (int, error)) (bool, error) {
	if name == "" {
		return false, nil
	}
	count, err := listFn(ctx, shareListOptions(name))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func shareLockKey(kind config.ResourceKind, name string) string {
	if kind == "" || name == "" {
		return ""
	}
	return fmt.Sprintf("kubemin-share:%s:%s", kind, name)
}

func acquireShareLock(key string) (func(), error) {
	if key == "" {
		return nil, nil
	}
	lock, err := shareLock(key)
	if err != nil {
		return nil, err
	}
	if err := lock.Lock(); err != nil {
		return nil, err
	}
	return func() {
		if err := lock.Unlock(); err != nil {
			klog.Warningf("failed to release share lock %s: %v", key, err)
		}
	}, nil
}

func shareMutex(key string) *sync.Mutex {
	if lock, ok := shareLocks.Load(key); ok {
		return lock.(*sync.Mutex)
	}
	lock := &sync.Mutex{}
	actual, _ := shareLocks.LoadOrStore(key, lock)
	return actual.(*sync.Mutex)
}

type localLock struct {
	mu *sync.Mutex
}

func (l localLock) Lock() error {
	l.mu.Lock()
	return nil
}

func (l localLock) TryLock() error {
	if l.mu.TryLock() {
		return nil
	}
	return errLockHeld
}

func (l localLock) Unlock() error {
	l.mu.Unlock()
	return nil
}

func shareLock(key string) (cache.Lock, error) {
	lock, _, err := cache.AcquireLock(key)
	if err != nil {
		return nil, err
	}
	if _, ok := lock.(cache.NoopLock); ok {
		return localLock{mu: shareMutex(key)}, nil
	}
	return lock, nil
}

func resolveSharedResource(ctx context.Context, name string, strategy config.ShareStrategy, kind config.ResourceKind, listFn func(context.Context, metav1.ListOptions) (int, error)) (func(), bool, error) {
	if name == "" || strategy == "" {
		return nil, false, nil
	}
	if strategy == config.ShareStrategyIgnore {
		return nil, true, nil
	}
	if strategy != config.ShareStrategyDefault {
		return nil, false, nil
	}
	unlock, err := acquireShareLock(shareLockKey(kind, name))
	if err != nil {
		return nil, false, err
	}
	release := func() {
		if unlock != nil {
			unlock()
		}
	}
	exists, err := hasSharedResources(ctx, name, listFn)
	if err != nil {
		release()
		return nil, false, err
	}
	if exists {
		release()
		return nil, true, nil
	}
	return unlock, false, nil
}
