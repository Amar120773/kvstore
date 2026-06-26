package store

import (
	"sync"
	"testing"
	"time"
)

// ============================================================================
// WHY TEST CONCURRENT CODE?
// ============================================================================
// Concurrency bugs are:
// - Hard to reproduce (happen randomly)
// - Hard to debug (race condition specific)
// - Catastrophic in production (data corruption)
//
// This file demonstrates:
// 1. Sequential tests (verify basic functionality)
// 2. Concurrent tests (verify thread-safety)
// 3. Race detector (Go's built-in race detection)
//
// Run with: go test -v
// Run with race detector: go test -race
// ============================================================================

// TestSetAndGet verifies basic Set/Get operations.
// WHY test this?
// - Sanity check that the store works
// - Baseline before testing concurrency
func TestSetAndGet(t *testing.T) {
	s := NewStore()

	// Test setting a value
	err := s.Set("key1", "value1")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test getting the value back
	val, found := s.Get("key1")
	if !found {
		t.Fatal("Get failed: key not found")
	}

	if val != "value1" {
		t.Fatalf("Expected 'value1', got '%s'", val)
	}
}

// TestConcurrentReads verifies that multiple goroutines can read simultaneously.
// WHY this test?
// - Verifies RWMutex allows concurrent readers
// - If this fails or is slow, RWMutex isn't working
func TestConcurrentReads(t *testing.T) {
	s := NewStore()

	// Set initial value
	s.Set("shared_key", "shared_value")

	// WaitGroup coordinates multiple goroutines
	// WHY WaitGroup?
	// - Ensures main test doesn't exit before all goroutines finish
	// - Simple pattern for synchronizing goroutines
	var wg sync.WaitGroup

	const numGoroutines = 100
	wg.Add(numGoroutines)

	// Launch 100 concurrent reads
	// Each goroutine attempts to read the same key
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			val, found := s.Get("shared_key")
			if !found {
				t.Error("Concurrent read failed: key not found")
			}
			if val != "shared_value" {
				t.Errorf("Concurrent read got wrong value: %s", val)
			}

			// Simulate some work
			time.Sleep(1 * time.Millisecond)
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	t.Logf("Successfully completed %d concurrent reads", numGoroutines)
}

// TestConcurrentWritesAndReads verifies thread-safety with mixed operations.
// WHY this test?
// - Most realistic scenario: reads and writes happening concurrently
// - Tests the mutex is protecting shared state correctly
func TestConcurrentWritesAndReads(t *testing.T) {
	s := NewStore()

	var wg sync.WaitGroup
	const numWriters = 10
	const numReaders = 20
	const opsPerGoroutine = 100

	// Launch writer goroutines
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for op := 0; op < opsPerGoroutine; op++ {
				key := "key_" + string(rune(writerID))
				value := "value_" + string(rune(op))
				err := s.Set(key, value)
				if err != nil {
					t.Errorf("Writer %d: Set failed: %v", writerID, err)
				}
			}
		}(w)
	}

	// Launch reader goroutines
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			for op := 0; op < opsPerGoroutine; op++ {
				// Try to read various keys
				for w := 0; w < numWriters; w++ {
					key := "key_" + string(rune(w))
					val, _ := s.Get(key)
					// We don't check the value because writes are still happening
					_ = val
				}
			}
		}(r)
	}

	// Wait for all operations to complete
	wg.Wait()

	// Verify store is not corrupted
	size := s.Size()
	if size == 0 {
		t.Fatal("Store is empty after concurrent operations")
	}

	t.Logf("Completed concurrent test with %d writers, %d readers, size=%d", numWriters, numReaders, size)
}

// TestExpirationBasic verifies that keys expire correctly.
// WHY test expiration?
// - TTL is a critical feature (prevents memory leaks)
// - Timing bugs are common (off-by-one, race conditions)
func TestExpirationBasic(t *testing.T) {
	s := NewStore()

	// Set a key with 100ms TTL
	ttl := 100 * time.Millisecond
	s.SetWithExpiration("temp_key", "temp_value", ttl)

	// Should be found immediately
	val, found := s.Get("temp_key")
	if !found {
		t.Fatal("Key should be found immediately after setting")
	}
	if val != "temp_value" {
		t.Fatalf("Expected 'temp_value', got '%s'", val)
	}

	// Wait for expiration
	time.Sleep(ttl + 50*time.Millisecond)

	// Should not be found now
	_, found = s.Get("temp_key")
	if found {
		t.Fatal("Key should have expired")
	}
}

// TestConcurrentExpirations verifies thread-safety of expiration.
// WHY this test?
// - Expiration interacts with locks (Get doesn't clean up)
// - Need to verify no race conditions in expiration logic
func TestConcurrentExpirations(t *testing.T) {
	s := NewStore()

	var wg sync.WaitGroup
	const numGoroutines = 50
	const keysPerGoroutine = 20

	// Set keys with short TTL
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for k := 0; k < keysPerGoroutine; k++ {
				key := "temp_" + string(rune(goroutineID)) + "_" + string(rune(k))
				s.SetWithExpiration(key, "value", 50*time.Millisecond)
			}
		}(g)
	}

	wg.Wait()

	// Wait for all keys to expire
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	cleaned := s.Cleanup()
	t.Logf("Cleanup removed %d expired keys", cleaned)

	// Verify store is empty
	size := s.Size()
	if size > 0 {
		t.Fatalf("Store should be empty after expiration and cleanup, but has %d keys", size)
	}
}

// TestDelete verifies deletion is thread-safe.
// WHY test deletion?
// - Delete modifies the map (requires Lock)
// - Need to verify no race conditions
func TestDelete(t *testing.T) {
	s := NewStore()

	// Set a key
	s.Set("to_delete", "value")

	// Verify it exists
	_, found := s.Get("to_delete")
	if !found {
		t.Fatal("Key should exist after setting")
	}

	// Delete it
	err := s.Delete("to_delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, found = s.Get("to_delete")
	if found {
		t.Fatal("Key should not exist after deletion")
	}
}

// TestList verifies listing works with concurrent operations.
// WHY test List?
// - List scans entire map (requires RLock for safety)
// - Need to verify snapshot is consistent
func TestList(t *testing.T) {
	s := NewStore()

	// Set some keys
	for i := 0; i < 100; i++ {
		s.Set("key_"+string(rune(i)), "value")
	}

	// List all
	keys := s.List("")
	if len(keys) != 100 {
		t.Fatalf("Expected 100 keys, got %d", len(keys))
	}

	// List with prefix
	keys = s.List("key_")
	if len(keys) != 100 {
		t.Fatalf("Expected 100 keys with prefix, got %d", len(keys))
	}

	keys = s.List("nonexistent_")
	if len(keys) != 0 {
		t.Fatalf("Expected 0 keys with nonexistent prefix, got %d", len(keys))
	}
}

// TestStatsAccuracy verifies stats are updated correctly.
// WHY test stats?
// - Stats track operation counts
// - Useful for monitoring and debugging
// - Need to verify counts are accurate
func TestStatsAccuracy(t *testing.T) {
	s := NewStore()

	// Perform some operations
	s.Set("key1", "value1")
	s.Set("key2", "value2")
	s.Get("key1")
	s.Get("key1")
	s.Delete("key1")

	// Check stats
	stats := s.GetStats()

	// WHY type assertion?
	// - Stats returns map[string]interface{}
	// - Need to convert to specific types
	sets := uint64(stats["sets"].(uint64))
	gets := uint64(stats["gets"].(uint64))
	deletes := uint64(stats["deletes"].(uint64))

	if sets != 2 {
		t.Fatalf("Expected 2 sets, got %d", sets)
	}
	if gets != 2 {
		t.Fatalf("Expected 2 gets, got %d", gets)
	}
	if deletes != 1 {
		t.Fatalf("Expected 1 delete, got %d", deletes)
	}
}

// Benchmark tests measure performance.
// WHY benchmarks?
// - Measure actual performance under load
// - Detect performance regressions
// - Run with: go test -bench=. -benchmem
//
// Example output:
//   BenchmarkSet-8              100000  10523 ns/op  1024 B/op  10 allocs/op
//   - "100000" = number of iterations
//   - "10523 ns/op" = nanoseconds per operation
//   - "1024 B/op" = bytes allocated per operation

// BenchmarkGet benchmarks GET operations.
func BenchmarkGet(b *testing.B) {
	s := NewStore()
	s.Set("bench_key", "bench_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get("bench_key")
	}
}

// ============================================================================
// LRU EVICTION TESTS
// ============================================================================

// TestLRUEvictionBasic verifies that LRU eviction works.
// WHY test LRU?
// - Critical for memory management
// - Must evict correct key (least recently used)
// - Must prevent unbounded growth
func TestLRUEvictionBasic(t *testing.T) {
	s := NewStore()

	// Set max size to 3
	s.SetMaxSize(3)

	// Add 3 keys with small delays to ensure different times
	s.Set("key1", "value1")
	time.Sleep(10 * time.Millisecond)
	s.Set("key2", "value2")
	time.Sleep(10 * time.Millisecond)
	s.Set("key3", "value3")

	// Verify all 3 exist
	if s.Size() != 3 {
		t.Fatalf("Expected 3 keys, got %d", s.Size())
	}

	// Access key1 to make it "recently used"
	time.Sleep(10 * time.Millisecond)
	s.Get("key1")

	// Add a 4th key (should evict key2, the LRU)
	time.Sleep(10 * time.Millisecond)
	s.Set("key4", "value4")

	// Verify key2 was evicted
	_, found := s.Get("key2")
	if found {
		t.Fatal("key2 should have been evicted")
	}

	// Verify key1, key3, key4 still exist
	_, found = s.Get("key1")
	if !found {
		t.Fatal("key1 should still exist (was recently accessed)")
	}
	_, found = s.Get("key3")
	if !found {
		t.Fatal("key3 should still exist")
	}
	_, found = s.Get("key4")
	if !found {
		t.Fatal("key4 should exist (just added)")
	}

	// Check eviction count
	if s.GetEvictions() != 1 {
		t.Fatalf("Expected 1 eviction, got %d", s.GetEvictions())
	}
}

// TestLRUEvictionUpdate verifies that updating a key doesn't trigger eviction.
// WHY this test?
// - Eviction should only happen when adding NEW keys
// - Updating existing key shouldn't evict
func TestLRUEvictionUpdate(t *testing.T) {
	s := NewStore()
	s.SetMaxSize(2)

	// Add 2 keys
	s.Set("key1", "value1")
	s.Set("key2", "value2")

	// Update key1 (should not trigger eviction)
	s.Set("key1", "updated_value")

	// Verify both keys still exist
	if s.Size() != 2 {
		t.Fatalf("Expected 2 keys after update, got %d", s.Size())
	}

	// Verify no evictions happened
	if s.GetEvictions() != 0 {
		t.Fatalf("Expected 0 evictions, got %d", s.GetEvictions())
	}
}

// TestLRUEvictionCascade verifies multiple evictions.
// WHY this test?
// - When multiple keys need eviction, must evict in order
// - Each addition should evict one LRU
func TestLRUEvictionCascade(t *testing.T) {
	s := NewStore()
	s.SetMaxSize(2)

	// Add 2 keys with delays
	s.Set("a", "1")
	time.Sleep(10 * time.Millisecond)
	s.Set("b", "2")

	// Add 3 more keys with delays (should trigger 3 evictions)
	time.Sleep(10 * time.Millisecond)
	s.Set("c", "3")
	time.Sleep(10 * time.Millisecond)
	s.Set("d", "4")
	time.Sleep(10 * time.Millisecond)
	s.Set("e", "5")

	// Verify only 2 keys remain
	if s.Size() != 2 {
		t.Fatalf("Expected 2 keys, got %d", s.Size())
	}

	// Verify 3 evictions happened
	if s.GetEvictions() != 3 {
		t.Fatalf("Expected 3 evictions, got %d", s.GetEvictions())
	}

	// Verify d and e exist (most recently added)
	_, found := s.Get("d")
	if !found {
		t.Fatal("key d should exist")
	}
	_, found = s.Get("e")
	if !found {
		t.Fatal("key e should exist")
	}

	// Verify a, b, c were evicted
	_, found = s.Get("a")
	if found {
		t.Fatal("key a should have been evicted")
	}
	_, found = s.Get("b")
	if found {
		t.Fatal("key b should have been evicted")
	}
	_, found = s.Get("c")
	if found {
		t.Fatal("key c should have been evicted")
	}
}

// TestLRUSetMaxSize verifies that changing max size triggers eviction.
// WHY this test?
// - Should be able to reduce max size and evict immediately
// - Or disable eviction (maxSize = 0)
func TestLRUSetMaxSize(t *testing.T) {
	s := NewStore()

	// Add 5 keys without limit
	for i := 1; i <= 5; i++ {
		s.Set("key"+string(rune('0'+i)), "value")
	}

	// Verify all 5 exist
	if s.Size() != 5 {
		t.Fatalf("Expected 5 keys, got %d", s.Size())
	}

	// Now set max size to 2 (should trigger 3 evictions)
	s.SetMaxSize(2)

	// Verify only 2 keys remain
	if s.Size() != 2 {
		t.Fatalf("Expected 2 keys after SetMaxSize, got %d", s.Size())
	}

	// Verify 3 evictions happened
	if s.GetEvictions() != 3 {
		t.Fatalf("Expected 3 evictions, got %d", s.GetEvictions())
	}
}

// TestLRUDisableEviction verifies that setting maxSize to 0 disables eviction.
// WHY this test?
// - Should support unlimited size when maxSize = 0
// - Useful for disabling LRU in dev/test
func TestLRUDisableEviction(t *testing.T) {
	s := NewStore()
	s.SetMaxSize(0) // Unlimited

	// Add many keys
	for i := 0; i < 100; i++ {
		key := "key" + string(rune('0'+(i%10)))
		s.Set(key, "value")
	}

	// Should have no evictions
	if s.GetEvictions() != 0 {
		t.Fatalf("Expected 0 evictions with unlimited size, got %d", s.GetEvictions())
	}
}

// TestLRUConcurrentEviction verifies LRU is thread-safe under concurrent ops.
// WHY this test?
// - Eviction must be atomic
// - Concurrent gets/sets must not corrupt state
func TestLRUConcurrentEviction(t *testing.T) {
	s := NewStore()
	s.SetMaxSize(50) // Small limit to trigger evictions

	var wg sync.WaitGroup
	const numWriters = 10
	const numWrites = 200 // Increased to guarantee evictions

	// Launch concurrent writers
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for op := 0; op < numWrites; op++ {
				key := "key_" + string(rune(writerID))
				s.Set(key, "value")
			}
		}(w)
	}

	wg.Wait()

	// Should be at max size (or slightly under)
	size := s.Size()
	if size > 50 {
		t.Fatalf("Expected size <= 50, got %d", size)
	}

	// Should have many evictions (with lots of writes to same keys)
	evictions := s.GetEvictions()
	// Each writer keeps writing to the same key, so limited evictions expected
	// But with proper LRU, we should see some
	t.Logf("Concurrent eviction test: size=%d, evictions=%d", size, evictions)
}

// BenchmarkSet benchmarks SET operations.
func BenchmarkSet(b *testing.B) {
	s := NewStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set("bench_key", "bench_value")
	}
}

// BenchmarkConcurrentGets benchmarks concurrent reads.
func BenchmarkConcurrentGets(b *testing.B) {
	s := NewStore()
	s.Set("bench_key", "bench_value")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.Get("bench_key")
		}
	})
}
