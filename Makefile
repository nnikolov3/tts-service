# TTS Microservice Project Makefile

.PHONY: build build-client build-service install clean test test-go test-python lint fmt help

# Build configuration
CLIENT_BINARY := tts-client
SERVICE_BINARY := tts-service
BUILD_DIR := build
INSTALL_DIR := $(HOME)/bin

# Go build flags
LDFLAGS := -w -s
BUILD_FLAGS := -ldflags="$(LDFLAGS)"

# Default target
all: build test lint

# Build both client and service
build: build-client

# Build the Go TTS client
build-client:
	@echo "Building $(CLIENT_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(CLIENT_BINARY) ./cmd/go-client

# Install client binary to ~/bin
install: build-client
	@echo "Installing $(CLIENT_BINARY) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(CLIENT_BINARY) $(INSTALL_DIR)/$(CLIENT_BINARY)
	@echo "✅ $(CLIENT_BINARY) installed to $(INSTALL_DIR)/$(CLIENT_BINARY)"
	@echo "Make sure $(INSTALL_DIR) is in your PATH"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f logs/*.log

# Run all tests
test: test-go test-python

# Run Go tests
test-go:
	@echo "Running Go tests..."
	go test -v ./cmd/go-client ./internal/tts

# Run Python tests
test-python:
	@echo "Running Python tests..."
	cd cmd/tts-service && python -m pytest test_main.py -v

# Run linter on Go code
lint:
	@echo "Running golangci-lint..."
	golangci-lint run --fix

# Format Go code
fmt:
	@echo "Formatting Go code..."
	go fmt ./...

# Build and start Python TTS service for development
start-service:
	@echo "Starting TTS service..."
	cd cmd/tts-service && python main.py $(shell pwd)/models/Llama-OuteTTS-1.0-1B-Q8_0.gguf

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the Go TTS client"
	@echo "  build-client  - Build the Go TTS client"
	@echo "  install       - Install client binary to ~/bin"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run all tests (Go + Python)"
	@echo "  test-go       - Run Go tests only"
	@echo "  test-python   - Run Python tests only"
	@echo "  lint          - Run golangci-lint on Go code"
	@echo "  fmt           - Format Go code"
	@echo "  start-service - Start Python TTS service for development"
	@echo "  help          - Show this help"
