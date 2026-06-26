package api

import (
	"encoding/json"
	"net/http"
	"time"

	"kvstore/store"
)

// ============================================================================
// WHY THIS FILE MATTERS:
// ============================================================================
// This demonstrates Go's HTTP server patterns:
// 1. Handler functions that accept (w http.ResponseWriter, r *http.Request)
// 2. Go automatically spawns a goroutine for each request
// 3. JSON marshaling/unmarshaling
// 4. Proper error handling in HTTP context
// ============================================================================

// Server wraps the store and provides HTTP handlers.
// WHY a wrapper struct?
// - Dependency injection: handlers have access to the store
// - Can extend with config, logger, metrics, etc. later
// - Makes it testable: inject a mock store in tests
// - Now includes apiKey for authentication
type Server struct {
	store  *store.Store
	apiKey string // API key for authentication (can be empty to disable auth)
}

// NewServer creates a new HTTP server without authentication.
func NewServer(s *store.Store) *Server {
	return &Server{
		store:  s,
		apiKey: "", // No authentication by default
	}
}

// NewServerWithAuth creates a new HTTP server with API key authentication.
// WHY separate constructor?
// - Optional feature (backward compatible)
// - Explicit intent: "I want auth enabled"
// - Dependency injection: auth key is injected
//
// Example usage:
//   server := api.NewServerWithAuth(kvStore, "secret-api-key-12345")
func NewServerWithAuth(s *store.Store, apiKey string) *Server {
	return &Server{
		store:  s,
		apiKey: apiKey,
	}
}

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

// SetRequest represents a SET request body.
// WHY use structs for JSON?
// - Type safety: compiler checks fields
// - Validation: can add struct tags for validation
// - Serialization: json package handles parsing
type SetRequest struct {
	Value string `json:"value"` // The value to store
	TTL   int    `json:"ttl"`   // Optional TTL in seconds
}

// GetResponse represents a GET response body.
type GetResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Found bool   `json:"found"`
}

// ErrorResponse represents any error response.
// WHY explicit error type?
// - Consistent error format across all endpoints
// - Easier for clients to parse errors
// - Can extend with error codes, details, etc.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ListResponse represents a LIST response body.
type ListResponse struct {
	Keys  []string `json:"keys"`
	Count int      `json:"count"`
}

// StatsResponse represents a STATS response body.
type StatsResponse struct {
	Gets      uint64 `json:"gets"`
	Sets      uint64 `json:"sets"`
	Deletes   uint64 `json:"deletes"`
	Size      int    `json:"size"`
	MaxSize   int    `json:"maxSize"`
	Evictions uint64 `json:"evictions"`
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

// SetHandler handles POST /api/keys/:key requests.
// WHY pass key as URL param instead of JSON?
// - REST best practice: the resource identifier goes in the URL
// - JSON body is for the data you're sending (the value)
func (s *Server) SetHandler(w http.ResponseWriter, r *http.Request) {
	// Extract key from URL path.
	// WHY use URL params?
	// - RESTful convention: /api/keys/{id} identifies the resource
	// - Easier to route and cache than query params
	key := r.PathValue("key")

	// Read request body
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// WHY check error?
		// - JSON parsing can fail (malformed input)
		// - Return 400 Bad Request to client
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "invalid_request",
			Code:    http.StatusBadRequest,
			Message: "Failed to parse request body: " + err.Error(),
		})
		return
	}

	// Store the value
	var err error
	if req.TTL > 0 {
		// WHY check TTL?
		// - If client provided TTL, use SetWithExpiration
		// - Demonstrates different methods for different use cases
		err = s.store.SetWithExpiration(key, req.Value, time.Duration(req.TTL)*time.Second)
	} else {
		err = s.store.Set(key, req.Value)
	}

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "set_failed",
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	// Success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"key":    key,
	})
}

// GetHandler handles GET /api/keys/:key requests.
// WHY is GET idempotent?
// - Doesn't modify state (except internal LastAccessed, which we ignore)
// - Can be called multiple times with same result
// - Safe to retry on network failures
func (s *Server) GetHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	// Get value from store
	value, found := s.store.Get(key)

	w.Header().Set("Content-Type", "application/json")

	if !found {
		// WHY 404 Not Found instead of 400?
		// - 404: resource doesn't exist (correct semantic)
		// - 400: request was malformed (incorrect)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(GetResponse{
			Key:   key,
			Found: false,
		})
		return
	}

	// Success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(GetResponse{
		Key:   key,
		Value: value,
		Found: true,
	})
}

// DeleteHandler handles DELETE /api/keys/:key requests.
// WHY DELETE is idempotent?
// - Deleting twice has same effect as deleting once
// - Safe to retry without side effects
// - Return 204 No Content (no response body needed)
func (s *Server) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	if err := s.store.Delete(key); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "delete_failed",
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	// 204 No Content - successful deletion with no response body
	w.WriteHeader(http.StatusNoContent)
}

// ListHandler handles GET /api/keys requests with optional prefix query param.
// WHY query param for prefix?
// - Filtering data (not identifying a resource)
// - GET request (idempotent)
// - Query params: ?prefix=user_
func (s *Server) ListHandler(w http.ResponseWriter, r *http.Request) {
	// WHY parse query params?
	// - Optional filtering (not required)
	// - r.URL.Query() gives access to all query params
	prefix := r.URL.Query().Get("prefix")

	keys := s.store.List(prefix)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ListResponse{
		Keys:  keys,
		Count: len(keys),
	})
}

// ExistsHandler handles GET /api/keys/:key/exists requests.
// WHY separate endpoint?
// - Some clients only need to check existence, not fetch the value
// - Faster response (no need to return large values)
// - Demonstrates specialized endpoints for different use cases
func (s *Server) ExistsHandler(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	exists := s.store.Exists(key)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"key":    key,
		"exists": exists,
	})
}

// StatsHandler handles GET /api/stats requests.
// WHY expose stats?
// - Monitoring: understand store usage patterns
// - Debugging: detect issues (e.g., lots of DELETEs but no SETs)
// - In production: feed into alerting/dashboards
func (s *Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	// Call store.GetStats() which uses RLock
	// Demonstrates that even monitoring doesn't need exclusive access
	stats := s.store.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}

// HealthHandler handles GET /health requests.
// WHY health check endpoint?
// - Load balancers use this to detect if server is alive
// - Kubernetes liveness probes use this
// - Simple check: if we can respond, we're healthy
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// LRU EVICTION HANDLERS
// ============================================================================

// ConfigRequest represents a config change request.
type ConfigRequest struct {
	MaxSize int `json:"maxSize"` // Maximum number of keys allowed
}

// ConfigHandler handles GET /api/config to retrieve current config.
// WHY expose config?
// - Clients need to know current settings
// - Useful for monitoring/debugging
func (s *Server) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	maxSize := s.store.GetMaxSize()
	evictions := s.store.GetEvictions()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"maxSize":    maxSize,
		"evictions":  evictions,
		"lruEnabled": maxSize > 0,
	})
}

// SetConfigHandler handles POST /api/config to change settings.
// WHY allow changing config?
// - In production, config might come from environment or config files
// - For this app, allows dynamic adjustment without restarting
// - Useful for testing different memory limits
func (s *Server) SetConfigHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request
	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "invalid_request",
			Code:    http.StatusBadRequest,
			Message: "Failed to parse request body: " + err.Error(),
		})
		return
	}

	// Set max size
	if err := s.store.SetMaxSize(req.MaxSize); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "config_error",
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	// Success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "configured",
		"maxSize": req.MaxSize,
	})
}

// ============================================================================
// AUTHENTICATION & MIDDLEWARE
// ============================================================================

// CheckAuth verifies the API key from request headers.
// WHY separate method?
// - Reusable across multiple handlers
// - Clear separation of concerns
// - Easy to test
//
// HOW it works:
// 1. Extract "X-API-Key" header
// 2. Compare with server's API key
// 3. Return true if matches, false otherwise
//
// SECURITY NOTE: Real apps use stronger methods (JWT, OAuth2, etc.)
// This is simplified for learning purposes.
func (s *Server) CheckAuth(r *http.Request) bool {
	// WHY check if apiKey is empty?
	// - Allows disabling auth (useful for dev/test)
	// - If apiKey is "", auth is disabled
	if s.apiKey == "" {
		return true // Auth disabled
	}

	// Extract API key from request header
	providedKey := r.Header.Get("X-API-Key")

	// Compare (constant-time comparison would be better in production)
	return providedKey == s.apiKey
}

// AuthMiddleware wraps a handler to enforce authentication.
// WHY middleware pattern?
// - Reusable: apply to any handler
// - Separates concerns: auth logic is separate from handler logic
// - Easy to test: can test auth independently
//
// Example:
//   authedHandler := s.AuthMiddleware(s.SetHandler)
//   mux.HandleFunc("POST /api/keys/{key}", authedHandler)
func (s *Server) AuthMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if authentication is required and if it passes
		if !s.CheckAuth(r) {
			// WHY 401 vs 403?
			// - 401 Unauthorized: client failed to authenticate (no/wrong API key)
			// - 403 Forbidden: client authenticated but not allowed to access resource
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:   "unauthorized",
				Code:    http.StatusUnauthorized,
				Message: "Missing or invalid X-API-Key header",
			})
			return
		}

		// Authentication passed, call the wrapped handler
		handler(w, r)
	}
}

// ============================================================================
// ROUTE SETUP
// ============================================================================

// RegisterRoutes registers all HTTP handlers with a mux.
// WHY centralize route registration?
// - Single place to see all endpoints
// - Easy to add middleware later
// - Demonstrates how to structure a server
//
// NOTE: With authentication enabled, all data endpoints require X-API-Key header
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Key-value operations (RESTful) - REQUIRE AUTHENTICATION
	mux.HandleFunc("POST /api/keys/{key}", s.AuthMiddleware(s.SetHandler))      // Create/Update
	mux.HandleFunc("GET /api/keys/{key}", s.AuthMiddleware(s.GetHandler))       // Read
	mux.HandleFunc("DELETE /api/keys/{key}", s.AuthMiddleware(s.DeleteHandler)) // Delete

	// Other operations - REQUIRE AUTHENTICATION
	mux.HandleFunc("GET /api/keys", s.AuthMiddleware(s.ListHandler))            // List all keys
	mux.HandleFunc("GET /api/keys/{key}/exists", s.AuthMiddleware(s.ExistsHandler)) // Check existence

	// Admin operations - REQUIRE AUTHENTICATION
	mux.HandleFunc("GET /api/stats", s.AuthMiddleware(s.StatsHandler))   // Stats and monitoring
	mux.HandleFunc("GET /api/config", s.AuthMiddleware(s.ConfigHandler)) // Get current config
	mux.HandleFunc("POST /api/config", s.AuthMiddleware(s.SetConfigHandler)) // Set config (e.g., maxSize)

	// Health check - PUBLIC (no authentication required)
	// WHY public?
	// - Load balancers need to check health without API key
	// - Kubernetes liveness probes need this
	// - General best practice to have public health endpoint
	mux.HandleFunc("GET /health", s.HealthHandler)

	// WHY these specific routes?
	// - POST /api/keys/{key}: Create or update (idempotent PUT would work too)
	// - GET /api/keys/{key}: Retrieve
	// - DELETE /api/keys/{key}: Delete
	// - GET /api/keys: List (with optional ?prefix=)
	// - GET /health: Standard health check (public)
	// - GET /api/stats: Monitoring (now protected)
	//
	// These follow REST conventions:
	// - HTTP methods (POST/GET/DELETE) indicate operation type
	// - URL structure indicates resource hierarchy
	// - Query params for filtering/options
	// - Authentication via X-API-Key header
}
