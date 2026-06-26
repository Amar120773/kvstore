# 📦 Project Summary: In-Memory KV Store

## What We've Built

A **production-quality** single-node in-memory key-value store in Go, demonstrating:
- **Concurrency Patterns**: `sync.RWMutex`, goroutines, channels
- **Memory Management**: TTL, background cleanup, garbage collection
- **API Design**: RESTful HTTP endpoints, proper error handling
- **Go Best Practices**: Dependency injection, graceful shutdown, testing

---

## Complete File Structure

```
kvstore/
├── go.mod                      # Module definition
├── main.go                     # Entry point, server setup, graceful shutdown
├── Makefile                    # Build targets (build, run, test, race, clean)
│
├── store/
│   ├── store.go               # Core thread-safe KV store (300+ lines, heavily commented)
│   └── store_test.go          # Comprehensive tests (concurrent, benchmarks, edge cases)
│
├── api/
│   └── handler.go             # HTTP handlers and routing (200+ lines, heavily commented)
│
└── Documentation/
    ├── README.md              # How to build, run, and test with examples
    ├── SETUP.md               # Installation and setup guide
    ├── CONCURRENCY_GUIDE.md   # Deep dive into Go concurrency patterns
    └── LEARNING_PATH.md       # Structured learning path with milestones
```

---

## Files at a Glance

### Core Implementation

| File | Lines | Purpose | Key Concepts |
|------|-------|---------|--------------|
| `store/store.go` | ~350 | Thread-safe data storage | RWMutex, TTL, cleanup |
| `api/handler.go` | ~250 | HTTP endpoints | Handlers, JSON, REST |
| `main.go` | ~150 | Server orchestration | Goroutines, signals, shutdown |

### Testing & Experimentation

| File | Lines | Purpose |
|------|-------|---------|
| `store/store_test.go` | ~400 | Tests for concurrency, TTL, edge cases |

### Documentation

| File | Purpose |
|------|---------|
| `README.md` | Usage guide with curl examples |
| `SETUP.md` | Installation instructions |
| `CONCURRENCY_GUIDE.md` | Explanation of concurrency patterns |
| `LEARNING_PATH.md` | Structured learning progression |

---

## Key Features

### 1. Core Store Operations
```go
Set(key, value)              // Create/update
Get(key)                     // Retrieve with expiration check
Delete(key)                  // Remove key
List(prefix)                 // Get all keys with optional filter
Exists(key)                  // Check existence
SetWithExpiration(key, ttl)  // Set with Time-To-Live
```

### 2. Thread Safety
- **RWMutex** allows multiple concurrent readers
- **Exclusive writes** ensure data integrity
- **No race conditions** (verified with `-race` flag)

### 3. Memory Management
- **Lazy expiration**: Skip expired keys on GET
- **Eager cleanup**: Background goroutine removes expired keys
- **Memory metrics**: Track store size and operation counts

### 4. HTTP API
```
POST   /api/keys/{key}          Set key with optional TTL
GET    /api/keys/{key}          Get key value
DELETE /api/keys/{key}          Delete key
GET    /api/keys               List all keys (with ?prefix filter)
GET    /api/keys/{key}/exists  Check if key exists
GET    /api/stats              Get operation statistics
GET    /health                 Health check
```

### 5. Production Patterns
- **Graceful shutdown**: Wait for in-flight requests before exit
- **Timeouts**: Server timeouts prevent slowloris attacks
- **Health checks**: For load balancers/Kubernetes
- **Monitoring**: Stats endpoint for observability

---

## Concurrency Patterns Demonstrated

### Pattern 1: RWMutex for Thread Safety
```go
// Multiple goroutines can read simultaneously
mu.RLock()
value := data[key]
mu.RUnlock()

// Only one goroutine can write
mu.Lock()
data[key] = newValue
mu.Unlock()
```

### Pattern 2: Goroutines for Concurrent Request Handling
```go
// HTTP server automatically spawns goroutine per request
httpServer.ListenAndServe()  // Handles multiple requests concurrently
```

### Pattern 3: Background Tasks
```go
// Cleanup runs in separate goroutine
go func() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        s.Cleanup()  // Remove expired keys
    }
}()
```

### Pattern 4: Channels for Signal Handling
```go
shutdown := make(chan os.Signal, 1)
signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
sig := <-shutdown  // Block until Ctrl+C
```

### Pattern 5: Graceful Shutdown
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
httpServer.Shutdown(ctx)  // Wait for requests to finish
```

---

## Learning Outcomes

After studying this project, you'll understand:

### Go Fundamentals
- Package structure and organization
- Error handling patterns
- Defer for resource cleanup
- Interface usage (implicit)

### Concurrency
- When and how to use goroutines
- RWMutex vs Mutex trade-offs
- Channel-based communication
- Race condition detection

### Systems Design
- RESTful API design
- Separation of concerns (store/api/main)
- Dependency injection
- Configuration management

### Production Readiness
- Timeouts and deadline handling
- Health checks
- Graceful shutdown
- Observability/monitoring

### Testing
- Concurrent testing patterns
- Benchmark testing
- Race detection (`-race` flag)

---

## Getting Started

### 1. Install Go
Follow `SETUP.md` for installation instructions

### 2. Build
```bash
cd kvstore
go build -o kvstore
```

### 3. Run
```bash
./kvstore
# Server listens on :8080
```

### 4. Test
```bash
# In another terminal
curl -X POST http://localhost:8080/api/keys/hello \
  -H "Content-Type: application/json" \
  -d '{"value":"world"}'

curl http://localhost:8080/api/keys/hello
```

### 5. Run Tests
```bash
go test -v -race ./...
```

---

## Code Quality Metrics

- ✅ **No race conditions** (verified with `go test -race`)
- ✅ **Full test coverage** of critical paths
- ✅ **Clean error handling** (no ignored errors)
- ✅ **Well-commented** (explains why, not just what)
- ✅ **Follows Go idioms** (defer, error returns, etc.)
- ✅ **Proper resource cleanup** (goroutines, mutexes)



---


## Resources

### Documentation Included
- `README.md` - Usage and examples
- `SETUP.md` - Installation
- `CONCURRENCY_GUIDE.md` - Deep dive into patterns
- `LEARNING_PATH.md` - Structured learning


## Summary

This project provides:
1. **Educational code** with explanations of concurrency patterns
2. **Production patterns** like graceful shutdown and monitoring
3. **Test examples** showing how to verify concurrent correctness
4. **Documentation** explaining design decisions
5. **Portfolio material** demonstrating Go expertise

It's designed to be:
- **Readable** - Clear code with extensive comments
- **Testable** - Comprehensive test suite
- **Learnable** - Structured learning path with resources
- **Reproducible** - Simple setup and execution
- **Extensible** - Easy to add features and improvements

---

