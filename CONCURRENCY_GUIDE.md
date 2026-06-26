# 🧠 Deep Dive: Go Concurrency & Memory Management

A comprehensive guide to understanding the concurrency patterns used in this KV store project.

---

## Table of Contents
1. [Goroutines](#goroutines)
2. [Channels](#channels)
3. [Mutexes & RWMutex](#mutexes--rwmutex)
4. [Memory Management](#memory-management)
5. [Common Pitfalls](#common-pitfalls)
6. [Interview Questions](#interview-questions)

---

## Goroutines

### What is a Goroutine?

A **goroutine** is a lightweight thread managed by the Go runtime. Unlike OS threads (1000s per process), you can have **millions of goroutines** in a single program.

```go
// This spawns a new goroutine
go doSomething()

// The program continues without waiting for doSomething() to finish
```

### Why Goroutines are Cheap

| Aspect | OS Thread | Goroutine |
|--------|-----------|-----------|
| Memory | ~2MB | ~2KB |
| Creation | Slow (kernel call) | Fast |
| Context Switch | Heavy | Light |
| Scalability | 1,000s | 100,000s+ |

```go
// You can safely create many goroutines
for i := 0; i < 100_000; i++ {
    go handleRequest(i)
}
// This works in Go; would crash with OS threads
```

### HTTP Server Concurrency

In this project, the HTTP server automatically spawns a goroutine for each request:

```go
// In main.go
go func() {
    log.Printf("Starting HTTP server on %s\n", ServerPort)
    err := httpServer.ListenAndServe() // Blocks, handling requests concurrently
}()

// Each incoming request runs in its own goroutine automatically
```

**Behind the scenes:**
```
Client 1 ──→ Request 1 ──→ Goroutine 1 ──→ Store.Get()
Client 2 ──→ Request 2 ──→ Goroutine 2 ──→ Store.Get()
Client 3 ──→ Request 3 ──→ Goroutine 3 ──→ Store.Set()
     ↓
All goroutines coordinate via sync.RWMutex to access shared Store
```

### Goroutine Lifecycle

```go
// When a goroutine completes, it exits
go func() {
    fmt.Println("I run concurrently")
    // When this function returns, goroutine exits
    // Memory is reclaimed
}()

// Main program can exit before goroutine finishes!
// Use sync.WaitGroup to wait
```

---

## Channels

### What are Channels?

**Channels** are Go's primary way for goroutines to communicate. They're like pipes: one goroutine sends data, another receives it.

```go
// Create a channel
ch := make(chan string)

// Send data
go func() {
    ch <- "Hello"  // Block until receiver is ready
}()

// Receive data
msg := <-ch  // Block until sender has data
fmt.Println(msg)  // Prints: "Hello"
```

### Channels in This Project

We use channels for **signal handling**:

```go
// In main.go
shutdown := make(chan os.Signal, 1)
signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

// Block until OS sends SIGINT (Ctrl+C)
sig := <-shutdown
fmt.Println("Got signal:", sig)
```

**Why this pattern?**
- Clean way to react to OS signals
- All goroutines can listen to the same signal channel
- Enables graceful shutdown

### Buffered Channels

```go
// Unbuffered: sender blocks until receiver is ready
ch := make(chan string)
ch <- "hello"  // Blocks here until someone receives

// Buffered: sender doesn't block if buffer has space
ch := make(chan string, 1)
ch <- "hello"  // Doesn't block; stored in buffer
ch <- "world"  // Blocks; buffer is full
```

In this project:
```go
serverErrors := make(chan error, 1)  // Buffered
go func() {
    serverErrors <- err  // Won't block even if no receiver
}()
```

### Select Statement

```go
select {
case msg1 := <-ch1:
    fmt.Println("Got from ch1:", msg1)
case msg2 := <-ch2:
    fmt.Println("Got from ch2:", msg2)
}
// Waits for first channel with data, then executes that case
```

In this project (main.go):
```go
select {
case err := <-serverErrors:
    log.Fatalf("Server error: %v", err)
case sig := <-shutdown:
    log.Printf("Received signal: %v\n", sig)
}
// Reacts to either server error OR shutdown signal
```

---

## Mutexes & RWMutex

### The Problem: Race Conditions

Without synchronization, concurrent access to shared data causes races:

```go
// ❌ WRONG: Race condition!
var counter int

go func() {
    counter++ // Goroutine 1
}()

go func() {
    counter++ // Goroutine 2
}()
```

What happens:
1. Both goroutines read counter value (e.g., 0)
2. Both increment it to 1
3. Both write 1 back
4. **Expected:** counter = 2, **Actual:** counter = 1

This is a **race condition**: result depends on timing.

### Mutex: Mutual Exclusion

A **Mutex** ensures only one goroutine can access a section at a time:

```go
// ✅ CORRECT: Thread-safe
var counter int
var mu sync.Mutex

go func() {
    mu.Lock()
    defer mu.Unlock()
    counter++  // Only one goroutine can be here
}()

go func() {
    mu.Lock()
    defer mu.Unlock()
    counter++  // Waits for first goroutine to release lock
}()
```

**Guarantee:** `counter = 2`

### Defer + Unlock Pattern

```go
mu.Lock()
defer mu.Unlock()
// ... do stuff ...
// Even if panic occurs, Unlock() is called!
```

**Why defer?**
- Ensures lock is released even if function panics
- Prevents deadlocks from forgotten Unlock()
- Go best practice

### RWMutex: Read-Write Locks

In the KV store, many clients want to **read** (GET) concurrently, but **writes** must be exclusive.

```go
// ❌ WRONG: All operations are serialized
var mu sync.Mutex
var data map[string]string

// GET operation (many clients need this!)
mu.Lock()
value := data[key]
mu.Unlock()

// GET operation (must wait for previous GET, even though reads don't conflict!)
mu.Lock()
value := data[key]
mu.Unlock()
```

This serializes reads unnecessarily.

```go
// ✅ CORRECT: Concurrent reads, exclusive writes
var mu sync.RWMutex
var data map[string]string

// GET operation
mu.RLock()  // Can be held by multiple goroutines
value := data[key]
mu.RUnlock()

// GET operation (runs concurrently!)
mu.RLock()
value := data[key]
mu.RUnlock()

// SET operation
mu.Lock()  // Only one goroutine can hold this
data[key] = value
mu.Unlock()
```

### RWMutex Semantics

| Lock | Who Can Hold | Use Case |
|------|--------------|----------|
| `RLock()` | Multiple readers | Read-only operations (GET) |
| `Lock()` | Single writer (blocks all readers) | Modifications (SET, DELETE) |

**Real-world analogy:**
- **RLock**: Library (many people reading books simultaneously)
- **Lock**: Librarian (exclusive access to catalog; patrons must wait)

### In This Project

```go
// GET operation (read-only)
func (s *Store) Get(key string) (string, bool) {
    s.mu.RLock()           // Multiple GETs can run simultaneously
    defer s.mu.RUnlock()
    val, exists := s.data[key]
    return val, exists
}

// SET operation (write)
func (s *Store) Set(key string, value string) error {
    s.mu.Lock()            // Exclusive: blocks all readers/writers
    defer s.mu.Unlock()
    s.data[key] = &Value{Data: value, ...}
    return nil
}
```

**Performance benefit:**
- 100 concurrent GETs run in parallel
- 1 SET blocks all GETs (but only while modifying map)

---

## Memory Management

### Go's Garbage Collection

Go's runtime automatically frees memory when objects are no longer referenced:

```go
func example() {
    // x is allocated on the heap
    x := &struct{ data string }{"hello"}
    
    // x is used...
    
    // After this function returns, x is unreachable
    // Go's GC will reclaim the memory
}
```

### Why TTL Matters

In a KV store, expired keys waste memory:

```go
// ❌ WRONG: Memory grows forever
store.Set("key1", "value1")  // 1MB
// ... after 1 year ...
store.Set("key1", "value1")  // 2MB
store.Set("key2", "value2")  // 3MB
// Keys are never deleted, memory keeps growing!
```

### Lazy vs Eager Expiration

**Lazy expiration** (this project):
```go
func (s *Store) Get(key string) (string, bool) {
    val, exists := s.data[key]
    if exists && time.Now().After(val.ExpiresAt) {
        return "", false  // Treat as expired, but don't delete
    }
    return val, exists
}
```

**Pros:** Fast (just a check)
**Cons:** Expired keys waste memory until cleanup runs

**Eager expiration** (background cleanup):
```go
func (s *Store) Cleanup() int {
    for key, val := range s.data {
        if time.Now().After(val.ExpiresAt) {
            delete(s.data, key)  // Actually remove from map
        }
    }
}
```

**Pros:** Freed memory immediately
**Cons:** Uses CPU/goroutine resources

### Memory Leak Pattern (Avoid!)

```go
// ❌ MEMORY LEAK: channel never drained
ch := make(chan string)
go func() {
    ch <- "data"  // Sends data to channel
}()
// Main function exits without receiving data
// The goroutine blocks forever, holding the channel
// Memory is never freed until program exits!
```

**Fix:**
```go
// ✅ CORRECT: Either receive the data or close the channel
ch := make(chan string)
go func() {
    ch <- "data"
}()
data := <-ch  // Receive it
```

---

## Common Pitfalls

### Pitfall 1: Forgotten Mutex Lock

```go
// ❌ RACE CONDITION: No lock!
func (s *Store) UnsafeGet(key string) string {
    return s.data[key]  // Multiple goroutines can race here!
}

// ✅ CORRECT: Lock first
func (s *Store) Get(key string) string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.data[key]
}
```

**Detects with:** `go run -race main.go`

### Pitfall 2: Modifying Map While Iterating

```go
// ❌ WRONG: Runtime panic!
for key, val := range s.data {
    delete(s.data, key)  // Can't modify while iterating
}

// ✅ CORRECT: Collect keys first, then delete
keys := make([]string, 0)
for key := range s.data {
    keys = append(keys, key)
}
for _, key := range keys {
    delete(s.data, key)
}
```

### Pitfall 3: Deadlock (Nested Locks)

```go
// ❌ DEADLOCK: Same goroutine tries to lock twice
var mu sync.Mutex

func example() {
    mu.Lock()
    defer mu.Unlock()
    
    // Try to lock again while already locked... blocks forever!
    mu.Lock()  // DEADLOCK
    defer mu.Unlock()
}
```

**Go doesn't support recursive mutexes!**

**Fix:** Use `sync.RWMutex` (readers can hold multiple RLocks)

### Pitfall 4: Goroutine Leak

```go
// ❌ GOROUTINE LEAK: Never returns
go func() {
    for {
        data := <-ch  // Channel never closes, blocks forever
    }
}()
```

**Fix:** Close channel or provide exit signal
```go
// ✅ CORRECT: Responds to shutdown
go func() {
    for {
        select {
        case data := <-ch:
            // process
        case <-ctx.Done():
            return  // Exit when context cancels
        }
    }
}()
```

### Pitfall 5: Copying Mutex

```go
// ❌ WRONG: Mutex is copied (separate lock instances!)
type MyStore struct {
    mu   sync.Mutex
    data map[string]string
}

store1 := MyStore{}
store2 := store1  // Copies mu! Now store2 has its own lock!
store1.mu.Lock()
store2.mu.Lock()  // Different locks!

// ✅ CORRECT: Use pointers
store2 := &store1  // Pointers share the same mu
```

---

## Interview Questions

### Q1: "Why RWMutex instead of Mutex?"

**Answer:** RWMutex allows multiple concurrent readers. In read-heavy systems (most KV stores), this dramatically improves throughput:

```go
// With Mutex: Each GET waits for previous GET to finish (serialized)
// With RWMutex: Multiple GETs run in parallel

// Benchmark results (100 GETs, 1 SET):
// - Mutex: ~100ms
// - RWMutex: ~10ms (10x faster!)
```

### Q2: "What's the difference between Mutex.Lock() and RWMutex.RLock()?"

**Answer:**
- `Lock()`: Only one goroutine can hold it (readers + writers blocked)
- `RLock()`: Multiple goroutines can hold it simultaneously

Use `Lock()` when modifying data, `RLock()` when only reading.

### Q3: "How does Go prevent race conditions?"

**Answer:** The `sync` package provides primitives (Mutex, RWMutex, Channels) for synchronization. Additionally:

```bash
go run -race main.go  # Detects races at runtime
```

The race detector instruments code to detect data races.

### Q4: "Why use a background cleanup goroutine?"

**Answer:** To prevent unbounded memory growth. Expired keys should be removed periodically:

```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        deleted := store.Cleanup()
        log.Printf("Cleaned up %d expired keys", deleted)
    }
}()
```

Without cleanup, the map keeps growing even if keys expire.

### Q5: "Explain graceful shutdown."

**Answer:** Instead of killing the process, we:

1. Stop accepting new requests
2. Wait for in-flight requests to complete
3. Close all resources
4. Exit cleanly

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
httpServer.Shutdown(ctx)  // Waits up to 30 seconds for requests
```

This prevents data loss and ensures integrity.

### Q6: "What's a data race and why is it bad?"

**Answer:** A data race occurs when two goroutines access the same memory without synchronization:

```go
// Race: Two goroutines both increment counter
counter++ // Goroutine 1
counter++ // Goroutine 2
// Result is unpredictable (1 or 2, usually 1)
```

**Why bad:**
- Unpredictable behavior
- Hard to reproduce and debug
- Can corrupt data in production
- Can cause panics or crashes

---

## Next Steps

1. **Run with race detector:** `go run -race main.go`
2. **Run tests:** `go test -race ./...`
3. **Read benchmarks:** `go test -bench=. ./store`
4. **Study Effective Go:** [golang.org/doc/effective_go](https://golang.org/doc/effective_go)

---

**Happy learning! 🚀 Master these patterns, and you'll build robust, scalable systems in Go.**
