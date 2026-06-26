# In-Memory Key-Value Store (Go Portfolio Project)

A single-node, in-memory key-value store built in Go to demonstrate concurrency primitives and memory management. Exposed via a RESTful HTTP API.

## 🎯 Learning Objectives Demonstrated

- **Concurrency**: `sync.RWMutex` (read-write locks), goroutines, channels
- **Memory Management**: TTL (Time-To-Live), lazy + eager expiration cleanup
- **HTTP API Design**: RESTful conventions, proper status codes, error handling
- **Go Best Practices**: Dependency injection, graceful shutdown, defer cleanup

---

## 📦 Project Structure

```
kvstore/
├── go.mod               # Go module definition
├── main.go             # Entry point, server setup, graceful shutdown
├── store/
│   └── store.go        # Thread-safe KV store implementation
├── api/
│   └── handler.go      # HTTP handlers and routing
└── README.md           # This file
```

---

## 🚀 Building and Running

### Prerequisites
- Go 1.21 or later (download from [golang.org](https://golang.org))

### Build

```bash
cd kvstore
go build -o kvstore
```

### Run

```bash
./kvstore
```

Or directly:
```bash
go run main.go
```

You should see:
```
Cleanup routine started (interval: 30s)
Starting HTTP server on :8080
```

---

## � Authentication

All API endpoints (except `/health`) require an **X-API-Key** header for security.

**Default API Key:** `demo-api-key-12345` (shown on server startup)

### To Disable Authentication (Development Only)

Edit `main.go` and change:
```go
server := api.NewServerWithAuth(kvStore, DefaultAPIKey)
```

To:
```go
server := api.NewServer(kvStore)  // No authentication
```

---

## �📡 API Endpoints

### Set a Key (Create/Update)
```bash
curl -X POST http://localhost:8080/api/keys/mykey \
  -H "Content-Type: application/json" \
  -H "X-API-Key: demo-api-key-12345" \
  -d '{"value": "hello world", "ttl": 3600}'
```

**Response:**
```json
{"status": "created", "key": "mykey"}
```

**Parameters:**
- `value` (required): The value to store
- `ttl` (optional): Time-to-live in seconds (0 or omitted = no expiration)

---

### Get a Key
```bash
curl http://localhost:8080/api/keys/mykey \
  -H "X-API-Key: demo-api-key-12345"
```

**Response:**
```json
{"key": "mykey", "value": "hello world", "found": true}
```

---

### Check if Key Exists
```bash
curl http://localhost:8080/api/keys/mykey/exists \
  -H "X-API-Key: demo-api-key-12345"
```

**Response:**
```json
{"key": "mykey", "exists": true}
```

---

### List All Keys (with optional prefix filter)
```bash
# Get all keys
curl http://localhost:8080/api/keys \
  -H "X-API-Key: demo-api-key-12345"

# Get keys starting with "user_"
curl http://localhost:8080/api/keys?prefix=user_ \
  -H "X-API-Key: demo-api-key-12345"
```

**Response:**
```json
{"keys": ["mykey", "user_1", "user_2"], "count": 3}
```

---

### Delete a Key
```bash
curl -X DELETE http://localhost:8080/api/keys/mykey \
  -H "X-API-Key: demo-api-key-12345"
```

**Response:** `204 No Content` (no body)

---

### Get Statistics
```bash
curl http://localhost:8080/api/stats \
  -H "X-API-Key: demo-api-key-12345"
```

**Response:**
```json
{
  "gets": 5,
  "sets": 2, 
  "deletes": 1, 
  "size": 10,
  "maxSize": 10000,
  "evictions": 3
}
```

---

### Get Configuration (LRU Settings)
```bash
curl http://localhost:8080/api/config \
  -H "X-API-Key: demo-api-key-12345"
```

**Response:**
```json
{
  "maxSize": 10000,
  "evictions": 3,
  "lruEnabled": true
}
```

---

### Set Configuration (Change LRU Max Size)
```bash
curl -X POST http://localhost:8080/api/config \
  -H "Content-Type: application/json" \
  -H "X-API-Key: demo-api-key-12345" \
  -d '{"maxSize": 5000}'
```

**Response:**
```json
{
  "status": "configured",
  "maxSize": 5000
}
```

**Parameters:**
- `maxSize` (required): Maximum number of keys allowed (0 = unlimited)
  - When store reaches this limit, the least recently used key is evicted
  - See [LRU_IMPLEMENTATION.md](LRU_IMPLEMENTATION.md) for details

---

### Health Check
```bash
curl http://localhost:8080/health
```

**Response:**
```json
{"status": "healthy", "timestamp": "2024-01-15T10:30:45Z"}
```

---

## 🧪 Testing (Manual)

### Test 1: Basic Set/Get Operations

```bash
# Set a value
curl -X POST http://localhost:8080/api/keys/greeting \
  -H "Content-Type: application/json" \
  -d '{"value": "Hello, Go!"}'

# Get it back
curl http://localhost:8080/api/keys/greeting

# Check stats
curl http://localhost:8080/api/stats
```

**Expected:** 1 SET, 1 GET in stats

---

### Test 2: TTL/Expiration

```bash
# Set a key with 2-second TTL
curl -X POST http://localhost:8080/api/keys/temp \
  -H "Content-Type: application/json" \
  -d '{"value": "I will expire", "ttl": 2}'

# Get it immediately
curl http://localhost:8080/api/keys/temp

# Wait 3 seconds
sleep 3

# Try to get it again (should be gone)
curl http://localhost:8080/api/keys/temp
```

**Expected:** First GET returns value, second GET returns `"found": false`

---

### Test 3: Concurrency (Heavy Load)

```bash
# Install Apache Bench (comes with Apache HTTP Server)
# On Windows: download from apachefriends.org or use WSL

ab -n 1000 -c 50 http://localhost:8080/api/keys/stress_test

# Monitor stats endpoint
curl http://localhost:8080/api/stats
```

**Expected:** No race conditions, stats show correct counts

---

### Test 4: Prefix Filtering

```bash
# Set multiple keys
curl -X POST http://localhost:8080/api/keys/user_1 \
  -H "Content-Type: application/json" \
  -d '{"value": "Alice"}'

curl -X POST http://localhost:8080/api/keys/user_2 \
  -H "Content-Type: application/json" \
  -d '{"value": "Bob"}'

curl -X POST http://localhost:8080/api/keys/product_1 \
  -H "Content-Type: application/json" \
  -d '{"value": "Widget"}'

# List all
curl http://localhost:8080/api/keys

# Filter by prefix
curl 'http://localhost:8080/api/keys?prefix=user_'
```

**Expected:** Prefix filter returns only keys starting with "user_"

---

## 🧠 Key Learning Points

### 1. **Why RWMutex instead of Mutex?**

The store uses `sync.RWMutex`, which allows:
- **Multiple concurrent readers** with `RLock()`
- **Exclusive writer access** with `Lock()`

In real systems, reads >> writes. RWMutex maximizes throughput by allowing many GET operations simultaneously while still protecting writes.

```go
// Multiple goroutines can hold this simultaneously
s.mu.RLock()
defer s.mu.RUnlock()
value := s.data[key]

// Only one goroutine can hold this
s.mu.Lock()
defer s.mu.Unlock()
s.data[key] = newValue
```

---

### 2. **Goroutines for Concurrency**

The HTTP server automatically spawns a goroutine for each request. This is safe because:
- All goroutines share the same RWMutex-protected map
- No data races (the mutex prevents simultaneous writes)
- Lightweight: can handle thousands of concurrent connections

```go
// This runs in a separate goroutine for each HTTP request
func (s *Server) GetHandler(w http.ResponseWriter, r *http.Request) {
    s.mu.RLock()          // Safe! Goroutines coordinate via mutex
    defer s.mu.RUnlock()
    // ...
}
```

---

### 3. **Memory Management: TTL + Cleanup**

Instead of growing unbounded, the store:
1. **Lazy Expiration**: On GET, skip expired keys
2. **Eager Cleanup**: Background goroutine removes expired keys every 30 seconds

This demonstrates garbage collection in action—Go's runtime frees the memory when expired entries are deleted.

```go
// Background routine runs every 30 seconds
go runCleanupRoutine(kvStore)

// Cleanup removes expired keys
func (s *Store) Cleanup() int {
    // ... deletes expired keys from map
    // Go's GC reclaims memory when objects are no longer referenced
}
```

---

### 4. **Graceful Shutdown**

When you press Ctrl+C, the server:
1. Stops accepting new connections
2. Waits up to 30 seconds for in-flight requests to complete
3. Closes cleanly

This demonstrates proper resource cleanup in concurrent systems.

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
if err := httpServer.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v\n", err)
}
```

---

## 🔥 Future Enhancements (For Learning)

Once you're comfortable with the basics, consider adding:

1. **Persistence**: Save to disk (JSON, Protocol Buffers)
2. **LRU Eviction**: Evict least-recently-used keys when memory limit is reached
3. **Pub/Sub**: Channels to publish key updates
4. **Clustering**: Sync multiple nodes (introduces distributed consensus challenges)
5. **Authentication**: JWT tokens or basic auth
6. **Rate Limiting**: Demonstrate throttling patterns
7. **Unit Tests**: Test store operations concurrently
8. **Benchmarks**: Measure performance characteristics

---

## 🛑 Stopping the Server

Press `Ctrl+C` to gracefully shut down. The server will:
1. Stop accepting new connections
2. Wait for existing requests to finish
3. Exit cleanly

```
^C
Received signal: interrupt. Shutting down gracefully...
Server stopped.
```

---

## 📚 Further Reading

- [Effective Go - Concurrency](https://golang.org/doc/effective_go#concurrency)
- [Go Memory Model](https://golang.org/ref/mem)
- [sync.RWMutex Documentation](https://pkg.go.dev/sync#RWMutex)
- [Context Package](https://pkg.go.dev/context)
- [HTTP Server Patterns](https://golang.org/doc/net#HTTP)

---

## 💡 Mentorship Notes for Portfolio

**What this project demonstrates:**

1. **Go Fundamentals**
   - Package organization
   - Interfaces (implicit)
   - Error handling
   - Defer and cleanup

2. **Concurrency**
   - Goroutines (HTTP server + cleanup routine)
   - Channels (signal handling)
   - Mutexes (RWMutex for thread safety)
   - Race detection (run with `go run -race main.go`)

3. **Systems Design**
   - RESTful API design
   - Separation of concerns (store/api/main)
   - Dependency injection
   - Graceful shutdown

4. **Production Readiness**
   - Timeouts
   - Health checks
   - Monitoring/stats
   - Proper error handling

**Interview talking points:**
- Explain why RWMutex is used (read-heavy workloads)
- Describe the concurrency model (goroutines + shared memory)
- Walk through graceful shutdown process
- Discuss memory management (TTL, cleanup)
- Mention trade-offs (e.g., lazy vs eager expiration)

---

## 📞 Debugging Tips

### Race Condition Detection
Go has a built-in race detector. Run with:
```bash
go run -race main.go
```

This will flag any potential data races. If you see race reports, it means two goroutines are accessing the same memory without synchronization!

### Verbose Logging
Add more logging to understand what's happening:
```go
// In store operations
log.Printf("Getting key: %s", key)

// In HTTP handlers
log.Printf("GET request for %s from %s", key, r.RemoteAddr)
```

### Profiling
For advanced optimization:
```bash
# CPU profiling
go test -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof ./...
go tool pprof mem.prof
```

---

**Happy learning! 🚀**
