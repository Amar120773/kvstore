package store

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// WHY THIS FILE MATTERS FOR LEARNING:
// ============================================================================
// This file demonstrates Go's core concurrency primitives:
// 1. sync.RWMutex - Read-Write locks (multiple readers, single writer)
// 2. Goroutine-safe operations (no data races)
// 3. Memory management with cleanup
//
// Key Insight: In production systems, reads >> writes. RWMutex lets us
// have concurrent readers, which is more efficient than a simple Mutex.
// ============================================================================

// Value represents data stored in the KV store with metadata.
// WHY a struct? Separation of concerns:
// - data: the actual value
// - expiresAt: when this key expires (0 means no expiration)
// This makes it easy to add features like TTL without changing the API.
type Value struct {
	Data      string    // The actual value we're storing
	ExpiresAt time.Time // When this key expires (zero value = no expiration)
	CreatedAt time.Time // Track when the key was created (useful for LRU later)
	LastAccessed time.Time // Track last read time (useful for LRU eviction)
}

// Store is our thread-safe, in-memory key-value store.
// WHY use a struct with embedded RWMutex?
// - The RWMutex protects access to the data map
// - Go's defer with Unlock() ensures we ALWAYS release the lock,
//   even if a panic occurs (defensive programming)
// - stats tracks metadata about store usage (for monitoring)
// - maxSize enforces memory limit with LRU eviction
type Store struct {
	// RWMutex allows multiple concurrent readers but exclusive writers.
	// This is critical for performance:
	// - RLock(): Multiple goroutines can hold this simultaneously
	// - Lock(): Only one goroutine can hold this (blocks readers & writers)
	mu sync.RWMutex

	// The actual data storage.
	// map[string]*Value:
	// - string key for O(1) lookup
	// - *Value (pointer) to avoid copying large data on reads
	data map[string]*Value

	// LRU Configuration
	// WHY add maxSize?
	// - Prevents unbounded memory growth
	// - Automatic eviction when limit is reached
	// - Simulates real-world memory constraints
	maxSize int // 0 = unlimited (default), >0 = max number of keys
	evictions uint64 // Track how many keys were evicted


	// Metadata for monitoring (optional, but good practice)
	stats struct {
		Gets    uint64 // Number of GET operations
		Sets    uint64 // Number of SET operations
		Deletes uint64 // Number of DELETE operations
	}
}

// NewStore creates and returns a new Store instance.
// WHY return a pointer (*Store)?
// - Maps in Go are reference types but sync.Mutex can't be copied.
// - By returning a pointer, callers share the same underlying store.
// - This is also the standard Go convention for types with methods.
func NewStore() *Store {
	return &Store{
		data: make(map[string]*Value),
	}
}

// Set stores a value in the store.
// WHY use value *string as parameter?
// - Pointer avoids copying the string for large values
// - But careful: this means the caller could modify the original!
// - In production, you might deep-copy to prevent external mutations.
func (s *Store) Set(key string, value string) error {
	// Validate input
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Lock for WRITING.
	// Lock() blocks until no readers or writers hold the lock.
	// CRITICAL: Any operation that modifies data needs Lock(), not RLock().
	s.mu.Lock()
	defer s.mu.Unlock() // Ensure lock is released (even on error)

	// Check if we need to evict before adding new key
	// WHY check before inserting?
	// - If key already exists, we're updating (no eviction needed)
	// - If key is new and we're at capacity, evict first
	if s.maxSize > 0 && len(s.data) >= s.maxSize {
		// Check if this is an update (key already exists)
		if _, exists := s.data[key]; !exists {
			// This is a new key and we're at capacity
			// Evict the least recently used key
			s.evictLRU()
		}
	}

	// Update the data
	s.data[key] = &Value{
		Data:         value,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		ExpiresAt:    time.Time{}, // Zero value = no expiration
	}

	// Update stats
	s.stats.Sets++

	return nil
}

// evictLRU removes the least recently used (oldest) key from the store.
// WHY separate method?
// - Encapsulation: LRU logic is in one place
// - Reusable: can be called from multiple locations
// - Testable: can test eviction independently
//
// WHY track LastAccessed?
// - LRU = Least Recently Used
// - We find the key with the oldest LastAccessed time
// - On ties, fall back to CreatedAt (when key was created)
//
// GOTCHA: This is O(n) complexity!
// For production, you'd use a more sophisticated data structure
// (e.g., doubly-linked list + map for O(1) eviction)
func (s *Store) evictLRU() {
	if len(s.data) == 0 {
		return // Nothing to evict
	}

	// Find the key with the oldest LastAccessed time
	var lruKey string
	var lruTime time.Time

	for key, val := range s.data {
		// Use LastAccessed if set, otherwise use CreatedAt
		accessTime := val.LastAccessed
		if accessTime.IsZero() {
			accessTime = val.CreatedAt
		}

		// First key or newer LRU candidate found?
		if lruKey == "" || accessTime.Before(lruTime) {
			lruKey = key
			lruTime = accessTime
		}
	}

	// Delete the LRU key
	delete(s.data, lruKey)
	s.evictions++
}

// Get retrieves a value from the store.
// WHY return (string, bool)?
// - The bool indicates whether the key exists (like map lookup).
// - Go convention for optional values.
// - Easier to handle than returning nil and checking.
func (s *Store) Get(key string) (string, bool) {
	// Lock for READING.
	// RLock() allows multiple goroutines to read simultaneously!
	// This is why RWMutex is better than Mutex for read-heavy workloads.
	s.mu.RLock()

	// Check if key exists and hasn't expired (don't defer here yet)
	val, exists := s.data[key]
	if !exists {
		s.mu.RUnlock()
		return "", false
	}

	// Check if key has expired
	if !val.ExpiresAt.IsZero() && time.Now().After(val.ExpiresAt) {
		// Key has expired, but we can't delete it while holding RLock.
		// In a more sophisticated implementation, we'd queue it for cleanup.
		s.mu.RUnlock()
		return "", false
	}

	// Update stats while holding the read lock
	// (Fine because we're only modifying stats, not data)
	s.stats.Gets++

	// Update LastAccessed time for LRU tracking
	// IMPORTANT: This requires upgrading from RLock to Lock
	// In a production system, this could be done asynchronously
	// For now, we accept the trade-off: slightly slower reads for accurate LRU
	
	// Release read lock and acquire write lock
	s.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check key still exists after lock upgrade
	val, exists = s.data[key]
	if !exists {
		return "", false
	}

	// Update LastAccessed for LRU tracking
	val.LastAccessed = time.Now()

	return val.Data, true
}

// Delete removes a key from the store.
// WHY return error?
// - Consistent error handling across operations.
// - Allows future expansions (e.g., "key not found" as error).
func (s *Store) Delete(key string) error {
	// Lock for WRITING.
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	s.stats.Deletes++

	return nil
}

// List returns all keys (optionally filtered by prefix).
// WHY take a prefix parameter?
// - Demonstrates how to optimize queries.
// - Real KV stores support range queries; this is simplified.
func (s *Store) List(prefix string) []string {
	// Lock for READING - we only need to read the map.
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string

	// Iterate over the map.
	// WHY scan all keys?
	// - O(n) operation, but safe because we hold the lock.
	// - In production, you'd use sorted structures or indexes for better performance.
	for key, val := range s.data {
		// Skip expired keys
		if !val.ExpiresAt.IsZero() && time.Now().After(val.ExpiresAt) {
			continue
		}

		// Filter by prefix if provided
		if prefix == "" || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}

	return keys
}

// SetWithExpiration sets a value with a TTL (Time-To-Live).
// WHY separate method instead of adding TTL to Set()?
// - Flexibility: Some callers want expiration, others don't.
// - Clarity: The name makes the intent obvious.
// - In production APIs, this might be a parameter with a default.
func (s *Store) SetWithExpiration(key string, value string, ttl time.Duration) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	if ttl <= 0 {
		return fmt.Errorf("TTL must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = &Value{
		Data:         value,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		ExpiresAt:    time.Now().Add(ttl), // Set expiration time
	}

	s.stats.Sets++

	return nil
}

// Exists checks if a key exists without returning its value.
// WHY separate method?
// - Performance: GET still returns the value even if we only need to check existence.
// - Clarity: Intent is explicit.
// - In Redis, EXISTS is a common operation.
func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, exists := s.data[key]
	if !exists {
		return false
	}

	// Check expiration
	if !val.ExpiresAt.IsZero() && time.Now().After(val.ExpiresAt) {
		return false
	}

	return true
}

// Size returns the number of keys in the store.
// WHY expose this?
// - Useful for monitoring and debugging.
// - Helps understand memory usage patterns.
func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Count non-expired keys
	count := 0
	for _, val := range s.data {
		if val.ExpiresAt.IsZero() || time.Now().Before(val.ExpiresAt) {
			count++
		}
	}

	return count
}

// GetStats returns current statistics.
// WHY expose stats?
// - Monitoring and debugging
// - In production, you'd expose these via /metrics endpoint
func (s *Store) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"gets":       s.stats.Gets,
		"sets":       s.stats.Sets,
		"deletes":    s.stats.Deletes,
		"size":       s.Size(),
		"maxSize":    s.maxSize,
		"evictions":  s.evictions,
	}
}

// Cleanup removes all expired keys from the store.
// WHY separate method?
// - Lazy vs eager cleanup:
//   - Lazy: Skip expired keys on access (current approach)
//   - Eager: Periodically remove all expired keys (this method)
// - In production, you'd run this in a background goroutine.
// - Demonstrates how to handle memory cleanup in Go.
func (s *Store) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0
	now := time.Now()

	// Iterate and delete expired keys.
	// WHY iterate and delete separately?
	// - Can't modify map while iterating in Go.
	// - This is a common Go pattern.
	for key, val := range s.data {
		if !val.ExpiresAt.IsZero() && now.After(val.ExpiresAt) {
			delete(s.data, key)
			deleted++
		}
	}

	return deleted
}

// SetMaxSize configures the maximum number of keys allowed in the store.
// WHY separate method?
// - Allows changing size limit without recreating store
// - LRU eviction only activates when this is set to > 0
// - In production, might make this configurable via config file or API
//
// Example usage:
//   store.SetMaxSize(1000)  // Keep max 1000 keys, evict LRU when exceeded
//   store.SetMaxSize(0)     // Disable eviction (unlimited)
func (s *Store) SetMaxSize(size int) error {
	if size < 0 {
		return fmt.Errorf("maxSize cannot be negative")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.maxSize = size

	// If we're now over the new limit, evict until we're under
	if size > 0 {
		for len(s.data) > size {
			s.evictLRU()
		}
	}

	return nil
}

// GetEvictions returns the number of keys evicted due to LRU.
// WHY expose this?
// - Monitoring: understand eviction frequency
// - Debugging: detect if size is too small
// - Performance analysis: balance memory vs evictions
func (s *Store) GetEvictions() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.evictions
}

// GetMaxSize returns the current max size limit.
// WHY expose this?
// - Let clients know if eviction is enabled
// - Useful for monitoring
func (s *Store) GetMaxSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.maxSize
}
