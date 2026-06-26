package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kvstore/store"
)

// ============================================================================
// WHY TEST AUTHENTICATION?
// ============================================================================
// Authentication bugs are security issues. We need to test:
// 1. Valid API key allows access
// 2. Missing API key denies access
// 3. Invalid API key denies access
// 4. Health endpoint is always public
// ============================================================================

// TestAuthWithValidKey verifies that valid API key grants access.
func TestAuthWithValidKey(t *testing.T) {
	s := store.NewStore()
	server := NewServerWithAuth(s, "test-api-key")

	// Create a test request with valid API key
	req := httptest.NewRequest("POST", "/api/keys/test", bytes.NewReader([]byte(`{"value":"hello"}`)))
	req.Header.Set("X-API-Key", "test-api-key")
	req.Header.Set("Content-Type", "application/json")

	// Check authentication
	if !server.CheckAuth(req) {
		t.Fatal("Authentication should pass with valid API key")
	}
}

// TestAuthWithInvalidKey verifies that invalid API key denies access.
func TestAuthWithInvalidKey(t *testing.T) {
	s := store.NewStore()
	server := NewServerWithAuth(s, "test-api-key")

	// Create a test request with invalid API key
	req := httptest.NewRequest("POST", "/api/keys/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")

	// Check authentication
	if server.CheckAuth(req) {
		t.Fatal("Authentication should fail with invalid API key")
	}
}

// TestAuthWithMissingKey verifies that missing API key denies access.
func TestAuthWithMissingKey(t *testing.T) {
	s := store.NewStore()
	server := NewServerWithAuth(s, "test-api-key")

	// Create a test request WITHOUT API key
	req := httptest.NewRequest("POST", "/api/keys/test", nil)
	// No X-API-Key header

	// Check authentication
	if server.CheckAuth(req) {
		t.Fatal("Authentication should fail with missing API key")
	}
}

// TestAuthDisabled verifies that empty API key disables authentication.
func TestAuthDisabled(t *testing.T) {
	s := store.NewStore()
	server := NewServer(s) // No API key = auth disabled

	// Create a test request WITHOUT API key
	req := httptest.NewRequest("POST", "/api/keys/test", nil)

	// Check authentication (should pass since auth is disabled)
	if !server.CheckAuth(req) {
		t.Fatal("Authentication should pass when disabled (empty API key)")
	}
}

// TestSetHandlerWithAuth verifies that SetHandler requires authentication.
func TestSetHandlerWithAuth(t *testing.T) {
	s := store.NewStore()
	s.Set("existing", "value")

	server := NewServerWithAuth(s, "secret-key")

	// Create a real mux so PathValue works
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Test 1: Request WITHOUT API key should be rejected
	body := SetRequest{Value: "hello", TTL: 0}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/keys/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	// NO X-API-Key header

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}

	// Test 2: Request WITH valid API key should succeed
	req = httptest.NewRequest("POST", "/api/keys/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret-key")

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d, body: %s", w.Code, w.Body.String())
	}
}

// TestHealthCheckIsPublic verifies that health endpoint doesn't require auth.
func TestHealthCheckIsPublic(t *testing.T) {
	s := store.NewStore()
	server := NewServerWithAuth(s, "secret-key")

	// Health check request WITHOUT API key should still work
	req := httptest.NewRequest("GET", "/health", nil)
	// NO X-API-Key header

	w := httptest.NewRecorder()
	server.HealthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Health check should return 200, got %d", w.Code)
	}
}

// TestGetHandlerWithAuth verifies that GetHandler requires authentication.
func TestGetHandlerWithAuth(t *testing.T) {
	s := store.NewStore()
	s.Set("mykey", "myvalue")

	server := NewServerWithAuth(s, "secret-key")

	// Create a real mux so PathValue works
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Test 1: Request WITHOUT API key should be rejected
	req := httptest.NewRequest("GET", "/api/keys/mykey", nil)
	// NO X-API-Key header

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}

	// Test 2: Request WITH valid API key should succeed
	req = httptest.NewRequest("GET", "/api/keys/mykey", nil)
	req.Header.Set("X-API-Key", "secret-key")

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d, body: %s", w.Code, w.Body.String())
	}
}

// TestDeleteHandlerWithAuth verifies that DeleteHandler requires authentication.
func TestDeleteHandlerWithAuth(t *testing.T) {
	s := store.NewStore()
	s.Set("toDelete", "value")

	server := NewServerWithAuth(s, "secret-key")

	// Test 1: Request WITHOUT API key should be rejected
	req := httptest.NewRequest("DELETE", "/api/keys/toDelete", nil)
	// NO X-API-Key header

	w := httptest.NewRecorder()
	server.AuthMiddleware(server.DeleteHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}

	// Test 2: Request WITH valid API key should succeed
	req = httptest.NewRequest("DELETE", "/api/keys/toDelete", nil)
	req.Header.Set("X-API-Key", "secret-key")

	w = httptest.NewRecorder()
	server.AuthMiddleware(server.DeleteHandler)(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d", w.Code)
	}
}

// TestListHandlerWithAuth verifies that ListHandler requires authentication.
func TestListHandlerWithAuth(t *testing.T) {
	s := store.NewStore()
	s.Set("key1", "value1")

	server := NewServerWithAuth(s, "secret-key")

	// Test 1: Request WITHOUT API key should be rejected
	req := httptest.NewRequest("GET", "/api/keys", nil)
	// NO X-API-Key header

	w := httptest.NewRecorder()
	server.AuthMiddleware(server.ListHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}

	// Test 2: Request WITH valid API key should succeed
	req = httptest.NewRequest("GET", "/api/keys", nil)
	req.Header.Set("X-API-Key", "secret-key")

	w = httptest.NewRecorder()
	server.AuthMiddleware(server.ListHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

// TestStatsHandlerWithAuth verifies that StatsHandler requires authentication.
func TestStatsHandlerWithAuth(t *testing.T) {
	s := store.NewStore()
	server := NewServerWithAuth(s, "secret-key")

	// Test 1: Request WITHOUT API key should be rejected
	req := httptest.NewRequest("GET", "/api/stats", nil)
	// NO X-API-Key header

	w := httptest.NewRecorder()
	server.AuthMiddleware(server.StatsHandler)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}

	// Test 2: Request WITH valid API key should succeed
	req = httptest.NewRequest("GET", "/api/stats", nil)
	req.Header.Set("X-API-Key", "secret-key")

	w = httptest.NewRecorder()
	server.AuthMiddleware(server.StatsHandler)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}
