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

import "time"

// Default values for lock options.
const (
	// DefaultTTL is the default lock expiration time.
	DefaultTTL = 30 * time.Second

	// DefaultRetryDelay is the default delay between retry attempts.
	DefaultRetryDelay = 500 * time.Millisecond

	// DefaultRetryCount is the default number of retry attempts.
	// -1 means infinite retries (until context is cancelled).
	DefaultRetryCount = -1
)

// Options holds configuration for a distributed mutex.
type Options struct {
	// TTL is the lock expiration time. After this duration, the lock will
	// automatically be released if not extended or unlocked.
	TTL time.Duration

	// RetryDelay is the duration to wait between retry attempts when
	// the lock is held by another process.
	RetryDelay time.Duration

	// RetryCount is the maximum number of retry attempts.
	// Set to -1 for infinite retries (until context cancellation).
	// Set to 0 for no retries (equivalent to TryLock behavior in Lock).
	RetryCount int

	// Metadata is optional key-value data associated with the lock.
	// Some backends may use this for debugging or ownership tracking.
	Metadata map[string]string
}

// Option is a functional option for configuring a mutex.
type Option func(*Options)

// DefaultOptions returns Options with default values.
func DefaultOptions() *Options {
	return &Options{
		TTL:        DefaultTTL,
		RetryDelay: DefaultRetryDelay,
		RetryCount: DefaultRetryCount,
		Metadata:   make(map[string]string),
	}
}

// ApplyOptions applies functional options to the default options and returns the result.
func ApplyOptions(opts ...Option) *Options {
	o := DefaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithTTL sets the lock expiration time.
func WithTTL(ttl time.Duration) Option {
	return func(o *Options) {
		if ttl > 0 {
			o.TTL = ttl
		}
	}
}

// WithRetryDelay sets the delay between retry attempts.
func WithRetryDelay(delay time.Duration) Option {
	return func(o *Options) {
		if delay > 0 {
			o.RetryDelay = delay
		}
	}
}

// WithRetryCount sets the maximum number of retry attempts.
// Use -1 for infinite retries, 0 for no retries.
func WithRetryCount(count int) Option {
	return func(o *Options) {
		o.RetryCount = count
	}
}

// WithMetadata sets metadata key-value pairs for the lock.
func WithMetadata(key, value string) Option {
	return func(o *Options) {
		if o.Metadata == nil {
			o.Metadata = make(map[string]string)
		}
		o.Metadata[key] = value
	}
}

// Config holds configuration for creating a Locker instance.
type Config struct {
	// Type specifies the backend type (redis, memory, noop, etcd).
	Type Type

	// RedisClient is the Redis client for TypeRedis.
	// Required when Type is TypeRedis.
	RedisClient interface{}

	// EtcdEndpoints are etcd server endpoints for TypeEtcd.
	// Required when Type is TypeEtcd.
	EtcdEndpoints []string

	// Prefix is an optional key prefix for all locks created by this locker.
	// Useful for namespacing locks in shared backends.
	Prefix string
}
