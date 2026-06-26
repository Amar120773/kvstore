# 🔐 Authentication Enhancement - Implementation Guide

## What We Added

**API Key Authentication** to secure all endpoints (except health check). This is a realistic security pattern used in production APIs.

---

## How It Works

### 1. **API Key Header**
Every request (except `/health`) must include the `X-API-Key` header:

```
X-API-Key: demo-api-key-12345
```

### 2. **Middleware Pattern**
We implemented authentication as middleware, which:
- Separates authentication logic from business logic
- Can be applied to any handler
- Makes it easy to add/remove authentication
- Demonstrates Go's middleware patterns

### 3. **Code Flow**

```
HTTP Request
    ↓
AuthMiddleware checks X-API-Key header
    ↓
Valid? → Handler executes → Response
Invalid? → 401 Unauthorized → Error response
```

---

## Files Modified

### 1. **api/handler.go**
**Changes:**
- Added `apiKey` field to `Server` struct
- Added `NewServerWithAuth()` constructor
- Added `CheckAuth()` method
- Added `AuthMiddleware()` function
- Updated `RegisterRoutes()` to apply middleware

**Key Code:**
```go
type Server struct {
    store  *store.Store
    apiKey string  // NEW: API key field
}

func NewServerWithAuth(s *store.Store, apiKey string) *Server {
    return &Server{
        store:  s,
        apiKey: apiKey,
    }
}

func (s *Server) CheckAuth(r *http.Request) bool {
    if s.apiKey == "" {
        return true  // Auth disabled
    }
    return r.Header.Get("X-API-Key") == s.apiKey
}

func (s *Server) AuthMiddleware(handler http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !s.CheckAuth(r) {
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(ErrorResponse{...})
            return
        }
        handler(w, r)
    }
}
```

### 2. **api/handler_test.go** (NEW FILE)
**What it tests:**
- Valid API key allows access
- Invalid API key denied (401)
- Missing API key denied (401)
- Auth can be disabled
- All endpoints respect authentication
- Health endpoint is always public

**Example test:**
```go
func TestAuthWithValidKey(t *testing.T) {
    server := NewServerWithAuth(store.NewStore(), "test-api-key")
    req := httptest.NewRequest("POST", "/api/keys/test", ...)
    req.Header.Set("X-API-Key", "test-api-key")
    
    if !server.CheckAuth(req) {
        t.Fatal("Authentication should pass with valid API key")
    }
}
```

### 3. **main.go**
**Changes:**
- Added `DefaultAPIKey` constant
- Changed `NewServer()` to `NewServerWithAuth()` with API key
- Added log message showing the API key on startup

**Code:**
```go
const (
    DefaultAPIKey = "demo-api-key-12345"
)

func main() {
    // ... create store ...
    server := api.NewServerWithAuth(kvStore, DefaultAPIKey)
    log.Printf("API Key: %s (include in request header: X-API-Key: %s)", 
        DefaultAPIKey, DefaultAPIKey)
    // ...
}
```

### 4. **go.mod**
- Changed `go 1.21` → `go 1.22` (already done for path parameters)

### 5. **README.md & SETUP.md**
- Added authentication section
- Updated all API examples to include `X-API-Key` header
- Documented how to disable authentication
- Updated PowerShell test script with authentication

---

## How to Use

### Starting the Server
```powershell
cd kvstore
go build -o kvstore.exe
.\kvstore.exe
```

**Output:**
```
API Key: demo-api-key-12345 (include in request header: X-API-Key: demo-api-key-12345)
Cleanup routine started (interval: 30s)
Starting HTTP server on :8080
```

### API Requests with Authentication
```powershell
# Define API key
$apiKey = "demo-api-key-12345"

# Set a value
$body = @{value="hello"} | ConvertTo-Json
Invoke-WebRequest -Uri http://localhost:8080/api/keys/test `
  -Method POST `
  -Headers @{"Content-Type"="application/json"; "X-API-Key"=$apiKey} `
  -Body $body

# Get a value
Invoke-WebRequest -Uri http://localhost:8080/api/keys/test `
  -Headers @{"X-API-Key"=$apiKey}

# Health check (NO API KEY NEEDED)
Invoke-WebRequest -Uri http://localhost:8080/health
```

### What Happens Without API Key
```powershell
# Try without API key
Invoke-WebRequest -Uri http://localhost:8080/api/keys/test

# Returns:
# StatusCode: 401
# Content: {"error":"unauthorized","code":401,"message":"Missing or invalid X-API-Key header"}
```

---

## Testing

### Run All Tests
```bash
go test -v ./...
```

**You should see:**
```
TestAuthWithValidKey - PASS
TestAuthWithInvalidKey - PASS
TestAuthWithMissingKey - PASS
TestAuthDisabled - PASS
TestSetHandlerWithAuth - PASS
TestHealthCheckIsPublic - PASS
TestGetHandlerWithAuth - PASS
TestDeleteHandlerWithAuth - PASS
TestListHandlerWithAuth - PASS
TestStatsHandlerWithAuth - PASS
```

### Run with Race Detector
```bash
go test -race ./...
```

Ensures no data races in authentication code.

---

## Security Patterns Demonstrated

### 1. **Middleware Pattern**
```go
// Wrap handler with authentication
mux.HandleFunc("POST /api/keys/{key}", s.AuthMiddleware(s.SetHandler))
```

### 2. **Optional Authentication**
```go
// Can disable by passing empty string
server := api.NewServer(kvStore)  // Auth disabled
```

### 3. **Public vs Private Endpoints**
```go
// Private (requires auth)
mux.HandleFunc("POST /api/keys/{key}", s.AuthMiddleware(s.SetHandler))

// Public (no auth required)
mux.HandleFunc("GET /health", s.HealthHandler)
```

### 4. **Proper HTTP Status Codes**
- `401 Unauthorized` - Missing or invalid API key
- `403 Forbidden` - (Reserved for authenticated but not authorized)

### 5. **Error Response Format**
```json
{
    "error": "unauthorized",
    "code": 401,
    "message": "Missing or invalid X-API-Key header"
}
```

---

## Production Considerations

### ⚠️ This is Educational, Not Production-Ready!

For production, you'd want:

1. **Better Key Management**
   - Store keys in secure config (not hardcoded)
   - Use environment variables or secret managers
   - Implement key rotation

2. **Stronger Authentication**
   - JWT (JSON Web Tokens)
   - OAuth2
   - mTLS (mutual TLS)
   - API key with hashing (don't store plaintext)

3. **Rate Limiting**
   - Limit requests per API key
   - Prevent brute force attacks

4. **Logging & Monitoring**
   - Log failed authentication attempts
   - Alert on suspicious patterns
   - Track API key usage

5. **Constant-Time Comparison**
   ```go
   // Current (vulnerable to timing attacks)
   return providedKey == s.apiKey
   
   // Better
   import "crypto/subtle"
   return subtle.ConstantTimeCompare([]byte(providedKey), []byte(s.apiKey)) == 1
   ```

---

## Interview Talking Points

**"Why middleware for authentication?"**
> "Middleware separates cross-cutting concerns (like auth) from business logic. It's reusable, testable, and easy to apply to multiple endpoints without duplicating code. This is a core architectural pattern in web frameworks."

**"How would you improve this for production?"**
> "I'd implement JWT tokens instead of simple API keys, use secure key storage (environment variables or secret managers), add rate limiting, constant-time comparison to prevent timing attacks, and comprehensive logging."

**"What about rate limiting per API key?"**
> "I'd add a rate limiter in the middleware that tracks requests per key using a separate map with timestamps, cleaning up old entries periodically. Go's channels could coordinate cleanup across goroutines."

---

## Summary

We've now added:
- ✅ **API Key Authentication** middleware
- ✅ **Security** to protect endpoints
- ✅ **Optional** authentication (can disable for dev/test)
- ✅ **Public** health endpoint for monitoring
- ✅ **Comprehensive tests** for auth functionality
- ✅ **Production patterns** (middleware, proper HTTP codes)
- ✅ **Error handling** with meaningful messages

This enhancement demonstrates:
1. Middleware patterns in Go
2. Security best practices
3. Separating concerns (auth from business logic)
4. Testing security-critical code
5. Backward compatibility (optional feature)

---

## Next Enhancement Ideas

1. **Persistence** - Save/load KV store from disk
2. **LRU Eviction** - Evict least-recently-used keys
3. **Rate Limiting** - Limit requests per API key
4. **Metrics** - Prometheus metrics endpoint
5. **Clustering** - Multi-node replication

---

