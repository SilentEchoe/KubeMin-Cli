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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryLockerBasic tests basic lock/unlock operations with MemoryLocker.
func TestMemoryLockerBasic(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	mutex := locker.NewMutex("test-key")
	ctx := context.Background()

	// Lock should succeed
	err := mutex.Lock(ctx)
	require.NoError(t, err)

	// Key should match
	assert.Equal(t, "test-key", mutex.Key())

	// Unlock should succeed
	err = mutex.Unlock(ctx)
	require.NoError(t, err)
}

// TestMemoryLockerTryLock tests TryLock behavior.
func TestMemoryLockerTryLock(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	mutex1 := locker.NewMutex("test-key")
	mutex2 := locker.NewMutex("test-key")
	ctx := context.Background()

	// First TryLock should succeed
	err := mutex1.TryLock(ctx)
	require.NoError(t, err)

	// Second TryLock should fail (lock is held)
	err = mutex2.TryLock(ctx)
	assert.ErrorIs(t, err, ErrLockAcquireFailed)

	// After unlock, TryLock should succeed
	err = mutex1.Unlock(ctx)
	require.NoError(t, err)

	err = mutex2.TryLock(ctx)
	require.NoError(t, err)

	err = mutex2.Unlock(ctx)
	require.NoError(t, err)
}

// TestMemoryLockerExtend tests lock extension.
func TestMemoryLockerExtend(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	mutex := locker.NewMutex("test-key", WithTTL(100*time.Millisecond))
	ctx := context.Background()

	// Lock
	err := mutex.Lock(ctx)
	require.NoError(t, err)

	// Extend should succeed
	err = mutex.Extend(ctx)
	require.NoError(t, err)

	// Unlock
	err = mutex.Unlock(ctx)
	require.NoError(t, err)

	// Extend after unlock should fail
	mutex2 := locker.NewMutex("test-key")
	err = mutex2.Extend(ctx)
	assert.ErrorIs(t, err, ErrLockNotHeld)
}

// TestMemoryLockerConcurrent tests concurrent lock acquisition.
func TestMemoryLockerConcurrent(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	ctx := context.Background()
	const goroutines = 10
	var counter int
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			mutex := locker.NewMutex("counter-lock", WithRetryDelay(10*time.Millisecond))
			err := mutex.Lock(ctx)
			if err != nil {
				t.Errorf("failed to acquire lock: %v", err)
				return
			}

			// Critical section
			counter++

			err = mutex.Unlock(ctx)
			if err != nil {
				t.Errorf("failed to release lock: %v", err)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, goroutines, counter)
}

// TestMemoryLockerContextCancellation tests context cancellation.
func TestMemoryLockerContextCancellation(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	mutex1 := locker.NewMutex("test-key")
	mutex2 := locker.NewMutex("test-key", WithRetryDelay(10*time.Millisecond))

	ctx := context.Background()

	// First lock
	err := mutex1.Lock(ctx)
	require.NoError(t, err)

	// Second lock with cancelled context should fail
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = mutex2.Lock(cancelCtx)
	assert.ErrorIs(t, err, context.Canceled)

	// Cleanup
	mutex1.Unlock(ctx)
}

// TestMemoryLockerTimeout tests context timeout.
func TestMemoryLockerTimeout(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	mutex1 := locker.NewMutex("test-key")
	mutex2 := locker.NewMutex("test-key", WithRetryDelay(10*time.Millisecond))

	ctx := context.Background()

	// First lock
	err := mutex1.Lock(ctx)
	require.NoError(t, err)

	// Second lock with timeout should fail
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = mutex2.Lock(timeoutCtx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Cleanup
	mutex1.Unlock(ctx)
}

// TestMemoryLockerPrefix tests key prefixing.
func TestMemoryLockerPrefix(t *testing.T) {
	locker := NewMemoryLocker("myapp")
	defer locker.Close()

	mutex := locker.NewMutex("test-key")
	assert.Equal(t, "myapp:test-key", mutex.Key())
}

// TestMemoryLockerUnlockNotHeld tests unlocking a lock not held.
func TestMemoryLockerUnlockNotHeld(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	mutex := locker.NewMutex("test-key")
	ctx := context.Background()

	// Unlock without holding should fail
	err := mutex.Unlock(ctx)
	assert.ErrorIs(t, err, ErrLockNotHeld)
}

// TestMemoryLockerExpiration tests lock expiration.
func TestMemoryLockerExpiration(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	// Create a lock with very short TTL
	mutex1 := locker.NewMutex("test-key", WithTTL(50*time.Millisecond))
	mutex2 := locker.NewMutex("test-key")
	ctx := context.Background()

	// First lock
	err := mutex1.Lock(ctx)
	require.NoError(t, err)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Second lock should succeed (first lock expired)
	err = mutex2.TryLock(ctx)
	require.NoError(t, err)

	mutex2.Unlock(ctx)
}

// TestNoopLocker tests NoopLocker behavior.
func TestNoopLocker(t *testing.T) {
	locker := NewNoopLocker("")
	defer locker.Close()

	mutex := locker.NewMutex("test-key")
	ctx := context.Background()

	// All operations should succeed
	err := mutex.Lock(ctx)
	assert.NoError(t, err)

	err = mutex.TryLock(ctx)
	assert.NoError(t, err)

	err = mutex.Extend(ctx)
	assert.NoError(t, err)

	err = mutex.Unlock(ctx)
	assert.NoError(t, err)

	assert.Equal(t, "test-key", mutex.Key())
}

// TestNoopLockerPrefix tests NoopLocker key prefixing.
func TestNoopLockerPrefix(t *testing.T) {
	locker := NewNoopLocker("myapp")
	defer locker.Close()

	mutex := locker.NewMutex("test-key")
	assert.Equal(t, "myapp:test-key", mutex.Key())
}

// TestNewFactory tests the New factory function.
func TestNewFactory(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		wantType string
		wantErr  bool
	}{
		{
			name:     "memory locker",
			cfg:      Config{Type: TypeMemory},
			wantType: "*locker.MemoryLocker",
		},
		{
			name:     "noop locker",
			cfg:      Config{Type: TypeNoop},
			wantType: "*locker.NoopLocker",
		},
		{
			name:     "empty type defaults to noop",
			cfg:      Config{Type: ""},
			wantType: "*locker.NoopLocker",
		},
		{
			name:    "redis without client",
			cfg:     Config{Type: TypeRedis},
			wantErr: true,
		},
		{
			name:    "etcd not implemented",
			cfg:     Config{Type: TypeEtcd},
			wantErr: true,
		},
		{
			name:    "unknown type",
			cfg:     Config{Type: "unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locker, err := New(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, locker)
			defer locker.Close()
		})
	}
}

// TestOptionsDefaults tests default option values.
func TestOptionsDefaults(t *testing.T) {
	opts := DefaultOptions()

	assert.Equal(t, DefaultTTL, opts.TTL)
	assert.Equal(t, DefaultRetryDelay, opts.RetryDelay)
	assert.Equal(t, DefaultRetryCount, opts.RetryCount)
	assert.NotNil(t, opts.Metadata)
}

// TestOptionsApply tests applying options.
func TestOptionsApply(t *testing.T) {
	opts := ApplyOptions(
		WithTTL(10*time.Second),
		WithRetryDelay(100*time.Millisecond),
		WithRetryCount(5),
		WithMetadata("owner", "test"),
	)

	assert.Equal(t, 10*time.Second, opts.TTL)
	assert.Equal(t, 100*time.Millisecond, opts.RetryDelay)
	assert.Equal(t, 5, opts.RetryCount)
	assert.Equal(t, "test", opts.Metadata["owner"])
}

// TestOptionsInvalidValues tests that invalid option values are ignored.
func TestOptionsInvalidValues(t *testing.T) {
	opts := ApplyOptions(
		WithTTL(-1),          // Invalid, should keep default
		WithRetryDelay(-100), // Invalid, should keep default
	)

	assert.Equal(t, DefaultTTL, opts.TTL)
	assert.Equal(t, DefaultRetryDelay, opts.RetryDelay)
}

// TestMemoryLockerClose tests locker close behavior.
func TestMemoryLockerClose(t *testing.T) {
	locker := NewMemoryLocker("")

	// Close the locker
	err := locker.Close()
	require.NoError(t, err)

	// New mutex after close should return degraded mutex
	mutex := locker.NewMutex("test-key")
	ctx := context.Background()

	err = mutex.Lock(ctx)
	assert.ErrorIs(t, err, ErrLockerClosed)
}

// TestRetryCount tests retry count behavior.
func TestRetryCount(t *testing.T) {
	locker := NewMemoryLocker("")
	defer locker.Close()

	ctx := context.Background()

	// Lock with first mutex
	mutex1 := locker.NewMutex("test-key")
	err := mutex1.Lock(ctx)
	require.NoError(t, err)

	// Try to lock with limited retries
	mutex2 := locker.NewMutex("test-key",
		WithRetryCount(2),
		WithRetryDelay(10*time.Millisecond),
	)

	start := time.Now()
	err = mutex2.Lock(ctx)
	elapsed := time.Since(start)

	// Should fail after retries
	assert.ErrorIs(t, err, ErrLockAcquireFailed)

	// Should have taken at least 2 retry delays
	assert.GreaterOrEqual(t, elapsed, 20*time.Millisecond)

	mutex1.Unlock(ctx)
}
