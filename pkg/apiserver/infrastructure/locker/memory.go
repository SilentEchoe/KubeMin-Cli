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
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryLocker implements Locker using in-memory locks.
// This is useful for single-instance deployments and testing.
// Note: This does NOT provide distributed locking across multiple processes.
type MemoryLocker struct {
	mu     sync.Mutex
	locks  map[string]*memoryLockEntry
	prefix string
	closed bool
}

type memoryLockEntry struct {
	mu        sync.Mutex
	held      bool
	owner     string // unique identifier for the lock holder
	expiresAt time.Time
}

// NewMemoryLocker creates a new in-memory locker.
func NewMemoryLocker(prefix string) *MemoryLocker {
	return &MemoryLocker{
		locks:  make(map[string]*memoryLockEntry),
		prefix: prefix,
	}
}

// NewMutex creates a new in-memory mutex for the given key.
func (l *MemoryLocker) NewMutex(key string, opts ...Option) Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return &degradedMutex{key: l.prefixedKey(key), err: ErrLockerClosed}
	}

	fullKey := l.prefixedKey(key)
	options := ApplyOptions(opts...)

	// Get or create lock entry
	entry, exists := l.locks[fullKey]
	if !exists {
		entry = &memoryLockEntry{}
		l.locks[fullKey] = entry
	}

	return &MemoryMutex{
		key:     fullKey,
		entry:   entry,
		options: options,
		locker:  l,
	}
}

// Close marks the locker as closed.
func (l *MemoryLocker) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.closed = true
	return nil
}

func (l *MemoryLocker) prefixedKey(key string) string {
	if l.prefix == "" {
		return key
	}
	return l.prefix + ":" + key
}

// MemoryMutex implements Mutex using in-memory synchronization.
type MemoryMutex struct {
	key     string
	entry   *memoryLockEntry
	options *Options
	locker  *MemoryLocker
	ownerID string // set when lock is acquired
}

// Lock acquires the lock, blocking until available or ctx is cancelled.
func (m *MemoryMutex) Lock(ctx context.Context) error {
	retryCount := m.options.RetryCount
	attempts := 0

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try to acquire
		if m.tryAcquire() {
			return nil
		}

		// Check retry limit
		if retryCount >= 0 {
			attempts++
			if attempts > retryCount {
				return ErrLockAcquireFailed
			}
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.options.RetryDelay):
		}
	}
}

// TryLock attempts to acquire the lock without blocking.
func (m *MemoryMutex) TryLock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if m.tryAcquire() {
		return nil
	}
	return ErrLockAcquireFailed
}

// Unlock releases the lock.
func (m *MemoryMutex) Unlock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	m.entry.mu.Lock()
	defer m.entry.mu.Unlock()

	if !m.entry.held || m.entry.owner != m.ownerID {
		return ErrLockNotHeld
	}

	m.entry.held = false
	m.entry.owner = ""
	m.entry.expiresAt = time.Time{}
	return nil
}

// Extend extends the lock's TTL.
func (m *MemoryMutex) Extend(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	m.entry.mu.Lock()
	defer m.entry.mu.Unlock()

	if !m.entry.held || m.entry.owner != m.ownerID {
		return ErrLockNotHeld
	}

	m.entry.expiresAt = time.Now().Add(m.options.TTL)
	return nil
}

// Key returns the key of this mutex.
func (m *MemoryMutex) Key() string {
	return m.key
}

// tryAcquire attempts to acquire the lock without blocking.
func (m *MemoryMutex) tryAcquire() bool {
	m.entry.mu.Lock()
	defer m.entry.mu.Unlock()

	now := time.Now()

	// Check if lock is expired
	if m.entry.held && !m.entry.expiresAt.IsZero() && now.After(m.entry.expiresAt) {
		// Lock has expired, release it
		m.entry.held = false
		m.entry.owner = ""
	}

	if m.entry.held {
		return false
	}

	// Acquire the lock
	m.ownerID = generateOwnerID()
	m.entry.held = true
	m.entry.owner = m.ownerID
	m.entry.expiresAt = now.Add(m.options.TTL)
	return true
}

// ownerIDCounter is used to generate unique owner IDs.
var ownerIDCounter int64

// generateOwnerID generates a unique identifier for lock ownership.
func generateOwnerID() string {
	id := atomic.AddInt64(&ownerIDCounter, 1)
	return fmt.Sprintf("mem-%d-%d", time.Now().UnixNano(), id)
}

// Compile-time interface checks
var (
	_ Locker = (*MemoryLocker)(nil)
	_ Mutex  = (*MemoryMutex)(nil)
)
