# 📚 Guided Learning Path: From Beginner to Portfolio-Ready

This document provides a structured path to learn from this KV store project, with specific milestones and learning objectives.

---

## Phase 1: Understanding the Basics (Day 1-2)

### 1.1 Read and Understand the Code
**Time:** 2-3 hours

**Steps:**
1. Read `store/store.go` thoroughly
   - Pay special attention to comments marked "WHY"
   - Understand the `sync.RWMutex` usage
   - Note the `Value` struct and why it's designed that way

2. Read `api/handler.go`
   - Understand RESTful conventions
   - Note HTTP status codes and when to use them
   - See how dependency injection works (Server struct)

3. Read `main.go`
   - Understand graceful shutdown pattern
   - Note the goroutine for cleanup
   - See channel-based signal handling

**Key Questions to Answer:**
- Why is RWMutex better than Mutex?
- What does `defer mu.RUnlock()` do?
- How does the HTTP server handle concurrent requests?

---

### 1.2 Set Up and Run the Project
**Time:** 30 minutes

**Steps:**
1. Follow `SETUP.md` to install Go
2. Run the server: `go run main.go`
3. Test basic endpoints (see `README.md` for examples)
4. Stop the server with Ctrl+C and observe graceful shutdown

**Success Criteria:**
- Server starts without errors
- Can GET/SET/DELETE keys via HTTP
- Can see stats endpoint working
- Graceful shutdown without crashes

---

### 1.3 Run the Tests
**Time:** 1 hour

**Steps:**
```bash
# Run all tests
go test -v ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...
```

**What to observe:**
- All tests pass
- No race conditions detected
- Benchmark results show performance numbers

**Key Questions:**
- What does the race detector do?
- Why do concurrent tests exist?
- How fast is the store? (Look at benchmark results)

---

## Phase 2: Hands-On Experimentation (Day 3-4)

### 2.1 Modify the Code
**Time:** 2-3 hours

**Experiments:**
1. **Add a new method to Store**: Implement `GetAll()` that returns all keys with values
   ```go
   func (s *Store) GetAll() map[string]string
   ```
   - Must be thread-safe (use RLock)
   - Add HTTP handler: `GET /api/all`
   - Test with concurrent requests

2. **Change TTL default**: Make expired keys immediately deleted instead of lazy
   - Modify `Get()` to delete expired keys
   - Run tests to ensure nothing breaks
   - Discuss trade-offs (performance vs memory)

3. **Add max size limit**: Store only keeps last 1000 keys
   - Implement `MaxKeys` field
   - Add eviction policy (simplest: remove oldest)
   - Add test for eviction

**Learning Goals:**
- Understand how to modify concurrent code safely
- See how to add new HTTP endpoints
- Learn to reason about trade-offs

---

### 2.2 Write a Test
**Time:** 1-2 hours

**Challenge:** Write a test that finds a bug in your modifications

```go
// Example: Test that concurrent GetAll() and Set() don't race
func TestConcurrentGetAllAndSet(t *testing.T) {
    s := NewStore()
    
    var wg sync.WaitGroup
    
    // GetAll in multiple goroutines
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            all := s.GetAll()
            _ = all
        }()
    }
    
    // Set in multiple goroutines
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            s.Set("key_"+string(rune(id)), "value")
        }(i)
    }
    
    wg.Wait()
}
```

**Learning Goals:**
- Understand concurrent test patterns
- Learn to verify thread-safety
- Gain confidence in modifying concurrent code

---

### 2.3 Profile the Code
**Time:** 1 hour

**Steps:**
1. Create a load test script
2. Run with CPU profiling
3. Analyze results

```bash
# Create load test (use Apache Bench or similar)
ab -n 10000 -c 100 http://localhost:8080/api/keys/test

# Run with profiling
go test -cpuprofile=cpu.prof -benchmem ./store -bench=.
go tool pprof cpu.prof

# Analyze memory
go test -memprofile=mem.prof -benchmem ./store -bench=.
go tool pprof mem.prof
```

**Learning Goals:**
- Understand performance characteristics
- Learn to identify bottlenecks
- See how Go's runtime works

---

## Phase 3: Building Portfolio Enhancements (Day 5-7)

### 3.1 Add Persistence (Level: Intermediate)
**Time:** 3-4 hours

**Objective:** Save the store to disk and reload on startup

```go
// Add to store.go
func (s *Store) SaveToFile(filename string) error {
    // Use encoding/json
    // Marshal store data to JSON
    // Write to file
}

func (s *Store) LoadFromFile(filename string) error {
    // Read from file
    // Unmarshal JSON
    // Populate store
}
```

**Implementation Steps:**
1. Use `encoding/json` to serialize
2. Add file I/O operations
3. Handle errors (file not found on first run)
4. Test with concurrent operations

**Learning:**
- JSON encoding/decoding in Go
- File I/O patterns
- Error handling best practices

---

### 3.2 Add Authentication (Level: Intermediate)
**Time:** 2-3 hours

**Objective:** Require API key for operations

```go
// Add API key middleware
func (s *Server) AuthMiddleware(handler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        if apiKey != "secret-key-123" {
            w.WriteHeader(http.StatusUnauthorized)
            return
        }
        handler.ServeHTTP(w, r)
    })
}
```

**Implementation Steps:**
1. Add middleware function
2. Wrap handlers with authentication
3. Add tests for auth success/failure
4. Update API documentation

**Learning:**
- HTTP header parsing
- Middleware patterns
- Security best practices

---

### 3.3 Add Metrics/Monitoring (Level: Advanced)
**Time:** 4-5 hours

**Objective:** Expose Prometheus metrics

```go
// Add metric collectors
var (
    getCount = prometheus.NewCounter(...)
    setCount = prometheus.NewCounter(...)
    latency = prometheus.NewHistogram(...)
)

// Expose /metrics endpoint
func (s *Server) MetricsHandler(w http.ResponseWriter, r *http.Request) {
    // Use promhttp to serialize metrics
}
```

**Implementation Steps:**
1. Add `github.com/prometheus/client_golang` dependency
2. Create metric collectors
3. Update handlers to track metrics
4. Expose `/metrics` endpoint
5. Test with curl and Prometheus

**Learning:**
- Observability in production systems
- Prometheus metrics format
- How monitoring works in real systems

---

### 3.4 Implement Clustering (Level: Very Advanced)
**Time:** 6+ hours

**Objective:** Replicate data across multiple nodes

**Concepts Needed:**
- Distributed consensus (Raft)
- Node discovery
- Replication protocol

**Not for Phase 1!** This is a stretch goal once you're confident with basics.

---

## Phase 4: Interview Preparation (Day 8+)

### 4.1 Understand Trade-offs

**Be able to discuss:**

1. **RWMutex vs Mutex**
   - When to use each
   - Performance implications
   - Trade-offs

2. **Lazy vs Eager Expiration**
   - Memory usage
   - CPU usage
   - When to use each

3. **Concurrency Patterns**
   - Goroutines vs OS threads
   - Channels vs shared memory
   - When to use each

4. **Data Structures**
   - Why HashMap (map) for O(1) lookup
   - Why not sorted structure
   - Trade-offs with different data structures

---

### 4.2 Prepare Demo

**What to show:**
1. Code tour (2-3 minutes)
2. Running the server (1 minute)
3. Testing endpoints (1-2 minutes)
4. Showing concurrent load test (1 minute)
5. Discussing architecture (3-5 minutes)

**Script:**
```
"This is a single-node in-memory KV store, similar to Redis, 
built to demonstrate Go's concurrency patterns.

Key features:
- Thread-safe operations using sync.RWMutex
- TTL/expiration support
- RESTful HTTP API
- Graceful shutdown
- Comprehensive monitoring

The store uses RWMutex to allow concurrent reads while ensuring 
exclusive writes. This is critical for performance in read-heavy workloads.

Background cleanup runs every 30 seconds to remove expired keys, 
preventing unbounded memory growth.

Let me show you some examples..."
```

---

### 4.3 Prepare Interview Talking Points

**Questions You Should Be Ready For:**

1. **"Walk me through how the store handles concurrent requests."**
   - Explain RWMutex semantics
   - Show Go scheduler at work
   - Discuss graceful shutdown

2. **"What if two goroutines write to the same key simultaneously?"**
   - Explain mutual exclusion
   - Show code with Lock()
   - Discuss atomicity

3. **"How does memory management work?"**
   - Explain TTL + cleanup
   - Discuss garbage collection
   - Show lazy vs eager trade-offs

4. **"How would you scale this to multiple nodes?"**
   - Discuss replication
   - Mention distributed consensus (Raft)
   - Discuss consistency models

5. **"What are potential bottlenecks?"**
   - Single node limitation
   - All data in memory
   - Network I/O
   - GC pauses

---

## Checklist for Portfolio Readiness

- [ ] Code is well-commented explaining "why" not just "what"
- [ ] All tests pass (`go test -v -race ./...`)
- [ ] No race conditions detected
- [ ] Server starts and runs correctly
- [ ] API endpoints respond correctly
- [ ] Graceful shutdown works
- [ ] README explains how to use and build
- [ ] CONCURRENCY_GUIDE explains concepts
- [ ] At least one enhancement implemented
- [ ] Code is properly formatted (`go fmt ./...`)
- [ ] No TODOs or hacks in code
- [ ] 2-minute demo prepared
- [ ] Can answer all interview questions above

---

## Time Estimate

| Phase | Time | Description |
|-------|------|-------------|
| Phase 1 | 4-5 hours | Understanding & running |
| Phase 2 | 4-5 hours | Experimentation & testing |
| Phase 3 | 8-10 hours | Enhancements (choose 2-3) |
| Phase 4 | 3-4 hours | Interview prep |
| **Total** | **20-25 hours** | Complete portfolio project |

---

## Resources

### Go Documentation
- [Effective Go](https://golang.org/doc/effective_go)
- [sync Package](https://pkg.go.dev/sync)
- [context Package](https://pkg.go.dev/context)
- [net/http Package](https://pkg.go.dev/net/http)

### Concurrency Resources
- [Go Memory Model](https://golang.org/ref/mem)
- [Concurrency is Not Parallelism](https://www.youtube.com/watch?v=cN_DpYBzKLs)
- [Advanced Go Concurrency Patterns](https://www.youtube.com/watch?v=QDDwwePbDtw)

### Performance & Profiling
- [Profiling Go Programs](https://go.dev/blog/profiling-go-programs)
- [Go Diagnostics Guide](https://golang.org/doc/diagnostics)

---

## Final Thoughts

This project teaches you:
1. **Go fundamentals** - syntax, packages, error handling
2. **Concurrency patterns** - goroutines, channels, mutexes
3. **Systems design** - API design, graceful shutdown, monitoring
4. **Testing** - concurrent tests, benchmarks, race detection
5. **Portfolio skills** - clear code, documentation, communication

**Key differentiator:** Most junior developers build something that works. You're building something that's **correct, thread-safe, and well-explained**. That's what stands out.

---

**Good luck! 🚀 You've got this.**
