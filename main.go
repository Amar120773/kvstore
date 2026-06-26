package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kvstore/api"
	"kvstore/store"
)

// ============================================================================
// WHY THIS ARCHITECTURE:
// ============================================================================
// We're demonstrating the SOLID principles in Go:
// - Single Responsibility: store.go (data), api/handler.go (HTTP), main.go (setup)
// - Dependency Injection: store injected into Server
// - Open/Closed: Easy to extend with new handlers or store implementations
//
// Concurrency Patterns shown:
// 1. goroutines: HTTP server spawns one per request automatically
// 2. channels: graceful shutdown using signal channel
// 3. context: propagating cancellation (shown in cleanup)
// ============================================================================

const (
	// Server configuration
	ServerPort = ":8080"
	ReadTimeout = 5 * time.Second
	WriteTimeout = 5 * time.Second
	IdleTimeout = 15 * time.Second

	// Cleanup configuration
	CleanupInterval = 30 * time.Second // Run cleanup every 30 seconds

	// LRU Eviction configuration
	// WHY configurable maxSize?
	// - Different use cases need different limits
	// - 0 = unlimited (no eviction)
	// - > 0 = evict LRU key when limit is reached
	MaxStoreSize = 10000 // Keep max 10,000 keys in memory

	// Authentication
	// WHY hardcode API key?
	// - For demo/development purposes
	// - In production, use environment variables or secure config managers
	// - Shows how to enable/disable auth in one place
	DefaultAPIKey = "demo-api-key-12345" // Change this for production!
)

func main() {
	// Create the store
	// WHY create here?
	// - Single instance shared across all HTTP handlers
	// - Each handler accesses the same underlying data
	// - Demonstrates the shared-memory concurrency model
	kvStore := store.NewStore()

	// Configure LRU eviction
	// WHY configure max size?
	// - Prevents unbounded memory growth
	// - When store reaches MaxStoreSize keys, evicts least recently used key
	// - Can be changed dynamically via POST /api/config endpoint
	// - Set to 0 to disable eviction (unlimited memory, use with caution!)
	kvStore.SetMaxSize(MaxStoreSize)
	log.Printf("LRU eviction enabled with maxSize=%d", MaxStoreSize)

	// Create the HTTP server wrapper with authentication enabled
	// WHY pass DefaultAPIKey?
	// - All API endpoints now require X-API-Key header
	// - To disable auth, pass empty string: server := api.NewServer(kvStore)
	// - See handler.go for how middleware works
	server := api.NewServerWithAuth(kvStore, DefaultAPIKey)

	log.Printf("API Key: %s (include in request header: X-API-Key: %s)", DefaultAPIKey, DefaultAPIKey)

	// Set up routing
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Create the HTTP server with timeouts
	// WHY set timeouts?
	// - Prevent slowloris attacks (clients hold connections open)
	// - Prevent zombie goroutines
	// - Ensure server can recover from hung requests
	httpServer := &http.Server{
		Addr:         ServerPort,
		Handler:      mux,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		IdleTimeout:  IdleTimeout,
	}

	// ========================================================================
	// START BACKGROUND CLEANUP GOROUTINE
	// ========================================================================
	// WHY background goroutine for cleanup?
	// - Expires keys periodically (eager cleanup)
	// - Prevents memory from growing unbounded with expired keys
	// - Demonstrates how to manage background tasks in Go
	//
	// CONCURRENCY PATTERN: goroutine + ticker
	// - Runs independently of HTTP server
	// - Demonstrates multi-goroutine application
	// - Need to cancel when server shuts down (shown below)
	go runCleanupRoutine(kvStore)

	// ========================================================================
	// GRACEFUL SHUTDOWN
	// ========================================================================
	// WHY graceful shutdown?
	// - Wait for in-flight requests to complete
	// - Don't abruptly kill goroutines
	// - Prevents data corruption or loss
	//
	// HOW it works:
	// 1. Listen for SIGTERM/SIGINT (Ctrl+C)
	// 2. Stop accepting new connections
	// 3. Wait for existing requests to finish
	// 4. Exit cleanly
	//
	// CONCURRENCY PATTERN: signal channel + context
	// - os/signal.Notify sends OS signals to a channel
	// - Allows main goroutine to react to signals
	// - Context propagates cancellation to handlers
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// ========================================================================
	// START SERVER IN BACKGROUND GOROUTINE
	// ========================================================================
	// WHY run server in a goroutine?
	// - ListenAndServe() blocks forever (until error or shutdown)
	// - Main goroutine waits for shutdown signal
	// - Allows clean shutdown handling
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Starting HTTP server on %s\n", ServerPort)
		// WHY log before listening?
		// - Helps verify server is starting
		// - Easier to debug if server crashes during startup

		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			// ErrServerClosed is expected on shutdown, not an error
			serverErrors <- err
		}
	}()

	// ========================================================================
	// WAIT FOR SHUTDOWN SIGNAL
	// ========================================================================
	// WHY use a select statement?
	// - Block until one of the channels receives data
	// - Handle shutdown signal OR server error
	// - Demonstrates Go's channel-based concurrency
	select {
	case err := <-serverErrors:
		// Server encountered an error during startup
		log.Fatalf("Server error: %v", err)

	case sig := <-shutdown:
		// Received shutdown signal (SIGINT or SIGTERM)
		log.Printf("\nReceived signal: %v. Shutting down gracefully...\n", sig)

		// Create a context with 30-second deadline
		// WHY timeout context?
		// - Don't wait forever for requests to finish
		// - After 30 seconds, force shutdown (might lose data!)
		// - In production, make this configurable
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Gracefully shutdown the HTTP server
		// - Stops accepting new connections
		// - Waits for existing requests to complete (or timeout)
		// - Closes listener
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v\n", err)
		}

		log.Println("Server stopped.")
	}
}

// ============================================================================
// BACKGROUND CLEANUP ROUTINE
// ============================================================================

// runCleanupRoutine periodically removes expired keys from the store.
// WHY separate function?
// - Testable: can test cleanup logic independently
// - Reusable: could use in multiple contexts
// - Clear intent: name describes what it does
func runCleanupRoutine(s *store.Store) {
	// Create a ticker that fires every CleanupInterval
	// WHY time.Ticker?
	// - Repeating events at regular intervals
	// - More efficient than time.Sleep in a loop
	// - Demonstrates Go's timer/ticker patterns
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop() // Ensure ticker is stopped on return

	log.Printf("Cleanup routine started (interval: %v)\n", CleanupInterval)

	for {
		// Block until next tick or until main exits
		// When main calls Shutdown(), the channel will close on next tick
		// (We'd need to use a context to truly stop this, shown in advanced version)
		<-ticker.C

		// Run cleanup
		deleted := s.Cleanup()

		if deleted > 0 {
			// Only log if something was deleted (reduce noise)
			log.Printf("Cleanup: removed %d expired keys\n", deleted)
		}
	}
	// NOTE: In a production version, we'd use a context.Done() channel
	// to cleanly stop this goroutine when the server shuts down.
	// For now, it's an acceptable "fire and forget" pattern.
}

// ============================================================================
// KEY LEARNING POINTS (Concurrency + Memory Management)
// ============================================================================
//
// 1. GOROUTINES:
//    - HTTP server spawns one per request (automatic)
//    - Cleanup routine runs in its own goroutine
//    - Main goroutine handles shutdown signal
//    - Lightweight (can have thousands)
//
// 2. CHANNELS:
//    - signal.Notify sends OS signals to a channel
//    - serverErrors communicates errors from server goroutine
//    - Demonstrates channel-based communication
//
// 3. MEMORY MANAGEMENT:
//    - sync.RWMutex prevents race conditions
//    - TTL + Cleanup prevents unbounded memory growth
//    - Proper cleanup (defer) ensures resource release
//
// 4. GRACEFUL SHUTDOWN:
//    - Don't kill goroutines abruptly
//    - Wait for in-flight requests
//    - Use context for deadline
//
// 5. SEPARATION OF CONCERNS:
//    - store/: Data management (thread-safe)
//    - api/: HTTP handling (request/response)
//    - main.go: Orchestration (server setup, shutdown)
//
// ============================================================================
