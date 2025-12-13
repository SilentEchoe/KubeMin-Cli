/*
Copyright 2024 The KubeMin Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package locker

import (
	"context"
)

// NoopLocker implements Locker with no-op behavior.
// All lock operations succeed immediately without actually locking.
// This is useful as a fallback when no distributed lock backend is available.
type NoopLocker struct {
	prefix string
}

// NewNoopLocker creates a new no-op locker.
func NewNoopLocker(prefix string) *NoopLocker {
	return &NoopLocker{prefix: prefix}
}

// NewMutex creates a new no-op mutex for the given key.
func (l *NoopLocker) NewMutex(key string, opts ...Option) Mutex {
	fullKey := key
	if l.prefix != "" {
		fullKey = l.prefix + ":" + key
	}
	return &NoopMutex{key: fullKey}
}

// Close is a no-op.
func (l *NoopLocker) Close() error {
	return nil
}

// NoopMutex implements Mutex with no-op behavior.
// All operations succeed immediately.
type NoopMutex struct {
	key string
}

// Lock always succeeds immediately.
func (m *NoopMutex) Lock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// TryLock always succeeds immediately.
func (m *NoopMutex) TryLock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Unlock always succeeds.
func (m *NoopMutex) Unlock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Extend always succeeds.
func (m *NoopMutex) Extend(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Key returns the key of this mutex.
func (m *NoopMutex) Key() string {
	return m.key
}

// Compile-time interface checks
var (
	_ Locker = (*NoopLocker)(nil)
	_ Mutex  = (*NoopMutex)(nil)
)
