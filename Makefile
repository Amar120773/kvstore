# Makefile for KV Store project
# WHY use a Makefile?
# - Single command to perform common tasks
# - Self-documenting (list all targets with 'make help')
# - No need to remember long command-line flags

.PHONY: help build run test clean race fmt lint

# Default target (shown when you run 'make' without args)
help:
	@echo "KV Store Makefile - Available targets:"
	@echo ""
	@echo "  make build       - Build the executable"
	@echo "  make run         - Run the server"
	@echo "  make test        - Run tests (creates example)"
	@echo "  make race        - Run with race detector"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make fmt         - Format code (gofmt)"
	@echo ""

# Build the executable
build:
	go build -o kvstore

# Run the server
run:
	go run main.go

# Test with basic operations
test: run
	@echo "Server is running. In another terminal, run:"
	@echo "  curl -X POST http://localhost:8080/api/keys/test -H 'Content-Type: application/json' -d '{\"value\":\"hello\"}'"
	@echo "  curl http://localhost:8080/api/keys/test"
	@echo "  curl http://localhost:8080/api/stats"

# Run with race detector (find data races)
race:
	go run -race main.go

# Clean up build artifacts
clean:
	rm -f kvstore

# Format code
fmt:
	go fmt ./...

# Run Go linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Build and run
dev: build run
