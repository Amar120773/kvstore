# LRU (Least Recently Used) Eviction Implementation

## Overview

This document explains the LRU eviction mechanism added to the KV store to prevent unbounded memory growth. The implementation follows Go best practices for concurrency and memory management.

## What is LRU Eviction?

**LRU (Least Recently Used)** is a cache eviction policy that removes the key that hasn't been accessed for the longest time when the store reaches its size limit.

### Example:
```
Store with maxSize=3:
1. Set("key1", "val1")      -> Store: {key1}
2. Set("key2", "val2")      -> Store: {key1, key2}
3. Set("key3", "val3")      -> Store: {key1, key2, key3}  [at capacity]
4. Get("key1")              -> key1 now has recent access time
5. Set("key4", "val4")      -> Store: {key1, key3, key4}  [key2 evicted - was LRU]
```

## Configuration

### Enable LRU in main.go

```go
// In main():
kvStore := store.NewStore()
kvStore.SetMaxSize(10000)  // Keep max 10,000 keys
```

### Disable LRU

```go
kvStore.SetMaxSize(0)  // Unlimited (no eviction)
```

### Change Size at Runtime (via API)

```bash
curl -X POST http://localhost:8080/api/config \
  -H "X-API-Key: demo-api-key-12345" \
  -H "Content-Type: application/json" \
  -d '{"maxSize": 5000}'
```

## API Endpoints for LRU

### Get Current Configuration
```bash
curl -X GET http://localhost:8080/api/config \
  -H "X-API-Key: demo-api-key-12345"
```

Response:
```json
{
  "maxSize": 10000,
  "evictions": 42,
  "lruEnabled": true
}
```

### Set New Max Size
```bash
curl -X POST http://localhost:8080/api/config \
  -H "X-API-Key: demo-api-key-12345" \
  -H "Content-Type: application/json" \
  -d '{"maxSize": 5000}'
```

### View Stats (includes eviction count)
```bash
curl -X GET http://localhost:8080/api/stats \
  -H "X-API-Key: demo-api-key-12345"
```

Response:
```json
{
  "gets": 1250,
  "sets": 320,
  "deletes": 5,
  "size": 10000,
  "maxSize": 10000,
  "evictions": 42
}
```

## Implementation Details

### Data Structures

```go
type Value struct {
    Data         string    // The actual value
    CreatedAt    time.Time // When key was created
    LastAccessed time.Time // Last time this key was accessed (for LRU)
    ExpiresAt    time.Time // TTL expiration time
}

type Store struct {
    mu         sync.RWMutex           // Thread-safe access
    data       map[string]*Value      // The actual store
    maxSize    int                    // Maximum number of keys (0=unlimited)
    evictions  uint64                 // Total eviction count
    stats struct {
        Gets    uint64
        Sets    uint64
        Deletes uint64
    }
}
```

### Key Methods

#### SetMaxSize(size int)
```go
// Enable or change LRU limit
store.SetMaxSize(1000)  // Keep max 1000 keys

// If new size is smaller than current size, immediately evicts excess keys
store.SetMaxSize(500)   // If store has 600 keys, evicts 100
```

#### Get() - Updates LastAccessed
```go
// When a key is accessed, its LastAccessed time is updated
// This is critical for accurate LRU tracking
val, found := store.Get("key")
// key's LastAccessed is now updated to current time
```

#### evictLRU() - Finds and Removes LRU Key
```go
// Internal method called automatically when at capacity
// Scans all keys to find the one with oldest LastAccessed
// Time complexity: O(n) where n = number of keys
// For production, consider using doubly-linked list + map for O(1) eviction
```

#### GetEvictions()
```go
// Returns total number of keys evicted
evictionCount := store.GetEvictions()
```

## Performance Considerations

### Current Implementation

**Pros:**
- Simple and correct
- Easy to understand
- No extra data structures needed

**Cons:**
- O(n) eviction: scans all keys to find LRU
- O(n) time for Set() when at capacity
- Get() requires lock upgrade (RLock -> Lock) to update LastAccessed

### Time Complexity Analysis

| Operation | Without LRU | With LRU (at capacity) |
|-----------|------------|----------------------|
| Set() | O(1) | O(n) - must find LRU |
| Get() | O(1) | O(1) - but upgrades lock |
| List() | O(n) | O(n) |

### Memory Overhead

For each key, we track:
- `LastAccessed time.Time` (24 bytes on 64-bit systems)
- `CreatedAt time.Time` (24 bytes)
- `ExpiresAt time.Time` (24 bytes)

Total: ~72 bytes per key for timing metadata

## Testing

### Run LRU Tests
```bash
go test -v -run TestLRU ./store
```

### Test Coverage

1. **TestLRUEvictionBasic**: Verify basic eviction
   - Create 3 keys, access one, add 4th
   - Verify correct key was evicted

2. **TestLRUEvictionUpdate**: Verify updates don't trigger eviction
   - Create 2 keys at capacity
   - Update one key
   - Verify no eviction occurs

3. **TestLRUEvictionCascade**: Verify multiple cascading evictions
   - Create 2 keys at maxSize=2
   - Add 3 more keys
   - Verify exactly 3 evictions

4. **TestLRUSetMaxSize**: Verify dynamic size changes
   - Add 5 keys
   - SetMaxSize(2)
   - Verify immediate evictions

5. **TestLRUConcurrentEviction**: Verify thread-safety
   - 10 concurrent writers
   - Large number of writes
   - Verify size limit is respected and no corruption

## Timing and LastAccessed Tracking

### Design Decision: Lock Upgrade in Get()

The `Get()` method updates `LastAccessed`, which requires a write lock. To avoid performance degradation, we use a lock upgrade pattern:

```go
// 1. Acquire RLock (read lock) for concurrent access
s.mu.RLock()
val, exists := s.data[key]
// ... validate key ...

// 2. Release RLock and acquire Lock (write lock)
s.mu.RUnlock()
s.mu.Lock()
defer s.mu.Unlock()

// 3. Double-check key still exists after upgrade
val, exists = s.data[key]
if !exists { return "", false }

// 4. Update LastAccessed
val.LastAccessed = time.Now()
return val.Data, true
```

**Why this approach?**
- Allows multiple concurrent reads initially
- Only one thread needs the write lock at a time
- Prevents stale reads (double-check after lock upgrade)

**Trade-offs:**
- Slightly slower reads (lock upgrade overhead)
- Accurate LRU tracking (vs skipping LastAccessed for performance)
- Balance between concurrency and correctness

### Alternative Approaches (Not Used)

1. **Always use Lock()**: Thread-safe but much slower reads
2. **Skip updating LastAccessed**: Fast reads but inaccurate LRU
3. **Async update (channel)**: Complex, eventual consistency
4. **Doubly-linked list + Map**: O(1) eviction but complex code

## Monitoring Evictions

### In Application Code
```go
// Get current stats
stats := store.GetStats()
log.Printf("Evictions: %d, Size: %d, MaxSize: %d", 
    stats["evictions"], stats["size"], stats["maxSize"])

// Monitor eviction rate
currentEvictions := store.GetEvictions()
// ... later ...
newEvictions := store.GetEvictions()
evictionRate := newEvictions - currentEvictions
```

### Via HTTP API
```bash
# Check stats endpoint
curl -X GET http://localhost:8080/api/stats \
  -H "X-API-Key: demo-api-key-12345" | jq '.evictions'
```

## When to Use LRU Eviction

**Good Use Cases:**
- Cache systems with bounded memory
- Session stores
- Rate limiting counters
- Last-N-items tracking
- Real-time metrics with sliding window

**Not Ideal For:**
- Data that must never be lost (use persistence)
- Systems requiring guaranteed retention
- Hot data that needs guaranteed availability

## Production Recommendations

1. **Set appropriate MaxSize**
   ```go
   // Based on available memory and key size
   // With 1KB average value: 10,000 keys = ~10MB
   MaxStoreSize := 10000
   ```

2. **Monitor evictions**
   - High eviction rate = size too small
   - Zero evictions = size too large (wasting memory)

3. **Log eviction events**
   ```go
   prevEvictions := store.GetEvictions()
   // ... later ...
   if store.GetEvictions() > prevEvictions {
       log.Printf("LRU eviction occurred: %d", store.GetEvictions())
   }
   ```

4. **Combine with TTL**
   - Use both TTL and LRU
   - TTL: removes stale data
   - LRU: removes inactive data on memory pressure

5. **For Production Scale**
   - Replace O(n) eviction with heap or linked-list (O(log n) or O(1))
   - Consider distributed caching (Redis, Memcached)
   - Implement persistence for critical data


