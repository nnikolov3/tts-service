# TTS Microservice Project Makefile

.PHONY: all build test lint clean fmt help

# Build configuration
SERVICE_BINARY := tts-service
BUILD_DIR := bin

# Go build flags
LDFLAGS := -w -s
BUILD_FLAGS := -ldflags="$(LDFLAGS)"

# Default target
all: build

# Build the service
build:
	@echo "Building $(SERVICE_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(SERVICE_BINARY) ./cmd/tts-service

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)

# Run Go tests
test:
	@echo "Running Go tests..."
	go test -v ./...

# Run linter on Go code
lint:
	@echo "Running linter and formatter..."
	@gofmt -w -s .
	@go vet ./...
	@golangci-lint run --fix ./...
	@echo "Cleaning caches..."
	@golangci-lint cache clean
	@go clean -cache

# Format Go code
fmt:
	@echo "Formatting Go code..."
	gofmt -w -s .

# Show help
help:
	@echo "Available targets:"
	@echo "  all           - Build the service"
	@echo "  build         - Build the Go TTS service"
	@echo "  test          - Run Go tests"
	@echo "  lint          - Run linter on Go code"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format Go code"
	@echo "  help          - Show this help"