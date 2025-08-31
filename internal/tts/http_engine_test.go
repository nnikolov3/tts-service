package tts_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"logger"

	"tts/internal/config"
	"tts/internal/tts"
)

// createTestConfig creates a minimal test configuration for HTTP engine testing.
func createTestConfig(serverURL string) *config.Config {
	return &config.Config{
		TTS: config.TTSConfig{
			ServiceHost:    "127.0.0.1",
			ServicePort:    8000,
			UseHTTPService: true,
			Workers:        2,
			TimeoutSeconds: 30,
			Temperature:    0.75,
		},
		Paths: config.PathsConfig{
			OutputDir: "/tmp/tts-test-output",
		},
		Logging: config.LoggingConfig{
			Level:         "info",
			LogDir:        "/tmp/tts-test-logs",
			MaxFileSizeMB: 10,
			MaxFiles:      3,
		},
	}
}

// createTestLogger creates a test logger instance for HTTP engine testing.
func createTestLogger(t *testing.T) *logger.Logger {
	t.Helper()

	lg, err := logger.New("/tmp/tts-test-logs", "test.log")
	if err != nil {
		t.Fatalf("Failed to create test logger: %v", err)
	}

	return lg
}

// createMockTTSServer creates a mock HTTP server that simulates TTS service responses.
func createMockTTSServer(
	t *testing.T,
	responses map[string]func(w http.ResponseWriter, r *http.Request),
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				handler, exists := responses[request.URL.Path]
				if !exists {
					t.Errorf(
						"Unexpected request path: %s",
						request.URL.Path,
					)
					responseWriter.WriteHeader(http.StatusNotFound)

					return
				}

				handler(responseWriter, request)
			},
		),
	)
}

// TestNewHTTPEngine verifies HTTP engine creation with proper dependencies.
func TestNewHTTPEngine(t *testing.T) {
	cfg := createTestConfig("http://localhost:8000")

	logger := createTestLogger(t)
	defer logger.Close()

	engine := tts.NewHTTPEngine(cfg, logger)
	if engine == nil {
		t.Fatal("NewHTTPEngine returned nil")
	}

	err := engine.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

// TestHTTPEngine_ProcessSingleChunk_Success verifies successful single chunk processing.
func TestHTTPEngine_ProcessSingleChunk_Success(t *testing.T) {
	const testAudioData = "mock-wav-audio-data"

	// Create mock server that responds to health and generation requests
	responses := map[string]func(responseWriter http.ResponseWriter, request *http.Request){
		"/health": func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			json.NewEncoder(responseWriter).Encode(map[string]any{
				"status":       "healthy",
				"model_loaded": true,
			})
		},
		"/v1/generate/speech": func(responseWriter http.ResponseWriter, request *http.Request) {
			responseWriter.Header().Set("Content-Type", "audio/wav")
			responseWriter.WriteHeader(http.StatusOK)
			responseWriter.Write([]byte(testAudioData))
		},
	}

	server := createMockTTSServer(t, responses)
	defer server.Close()

	// Create temporary output directory
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test_output.wav")

	// Create engine with mock server URL
	cfg := createTestConfig(server.URL)

	cfg.Paths.OutputDir = tempDir

	logger := createTestLogger(t)
	defer logger.Close()

	// Use test client pointing to mock server
	client := tts.NewHTTPClient(server.URL, 30*time.Second)

	engine := tts.NewHTTPEngineWithClient(cfg, logger, client)
	defer engine.Close()

	// Test single chunk processing
	testText := "Hello, this is a test."

	err := engine.ProcessSingleChunk(testText, outputPath)
	if err != nil {
		t.Fatalf("ProcessSingleChunk failed: %v", err)
	}

	// Verify output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}

	// Verify output file content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != testAudioData {
		t.Errorf(
			"Expected file content %q, got %q",
			testAudioData,
			string(content),
		)
	}
}

// TestHTTPEngine_ProcessSingleChunk_EmptyText verifies validation of empty text input.
func TestHTTPEngine_ProcessSingleChunk_EmptyText(t *testing.T) {
	cfg := createTestConfig("http://localhost:8000")

	logger := createTestLogger(t)
	defer logger.Close()

	engine := tts.NewHTTPEngine(cfg, logger)
	defer engine.Close()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test_output.wav")

	err := engine.ProcessSingleChunk("", outputPath)
	if err == nil {
		t.Fatal("Expected error for empty text, got nil")
	}
}

// TestHTTPEngine_ProcessSingleChunk_EmptyOutputPath verifies validation of empty output
// path.
func TestHTTPEngine_ProcessSingleChunk_EmptyOutputPath(t *testing.T) {
	cfg := createTestConfig("http://localhost:8000")

	logger := createTestLogger(t)
	defer logger.Close()

	engine := tts.NewHTTPEngine(cfg, logger)
	defer engine.Close()

	err := engine.ProcessSingleChunk("test text", "")
	if err == nil {
		t.Fatal("Expected error for empty output path, got nil")
	}
}

// TestHTTPEngine_ProcessChunks_Success verifies successful chunks file processing.
func TestHTTPEngine_ProcessChunks_Success(t *testing.T) {
	const testAudioData = "mock-wav-audio-data"

	// Create mock server
	responses := map[string]func(w http.ResponseWriter, r *http.Request){
		"/health": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"status":       "healthy",
				"model_loaded": true,
			})
		},
		"/v1/generate/speech": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "audio/wav")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testAudioData))
		},
	}

	server := createMockTTSServer(t, responses)
	defer server.Close()

	// Create test chunks file
	tempDir := t.TempDir()
	chunksPath := filepath.Join(tempDir, "chunks.json")
	testChunks := []string{
		"First chunk of text to process.",
		"Second chunk of text to process.",
		"Third chunk of text to process.",
	}

	chunksData, err := json.Marshal(testChunks)
	if err != nil {
		t.Fatalf("Failed to marshal test chunks: %v", err)
	}

	if err := os.WriteFile(chunksPath, chunksData, 0o644); err != nil {
		t.Fatalf("Failed to write chunks file: %v", err)
	}

	// Create engine with mock server URL
	cfg := createTestConfig(server.URL)

	cfg.Paths.OutputDir = tempDir

	logger := createTestLogger(t)
	defer logger.Close()

	// Use test client pointing to mock server
	client := tts.NewHTTPClient(server.URL, 30*time.Second)

	engine := tts.NewHTTPEngineWithClient(cfg, logger, client)
	defer engine.Close()

	// Test chunks processing
	outputDir := filepath.Join(tempDir, "output")

	err = engine.ProcessChunks(chunksPath, outputDir)
	if err != nil {
		t.Fatalf("ProcessChunks failed: %v", err)
	}

	// Verify output files were created
	expectedFiles := []string{
		"chunk_0001.wav",
		"chunk_0002.wav",
		"chunk_0003.wav",
	}

	for _, filename := range expectedFiles {
		outputPath := filepath.Join(outputDir, filename)
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Errorf("Expected output file %s was not created", filename)
		}
	}
}

// TestHTTPEngine_ProcessChunks_InvalidChunksFile verifies handling of invalid chunks
// files.
func TestHTTPEngine_ProcessChunks_InvalidChunksFile(t *testing.T) {
	cfg := createTestConfig("http://localhost:8000")

	logger := createTestLogger(t)
	defer logger.Close()

	engine := tts.NewHTTPEngine(cfg, logger)
	defer engine.Close()

	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")

	// Test with non-existent chunks file
	err := engine.ProcessChunks("/non/existent/chunks.json", outputDir)
	if err == nil {
		t.Fatal("Expected error for non-existent chunks file, got nil")
	}
}

// TestHTTPEngine_ProcessChunks_ServiceUnavailable verifies handling when service is down.
func TestHTTPEngine_ProcessChunks_ServiceUnavailable(t *testing.T) {
	// Create mock server that returns service unavailable
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				responseWriter.WriteHeader(http.StatusServiceUnavailable)
			},
		),
	)
	defer server.Close()

	// Create test chunks file
	tempDir := t.TempDir()
	chunksPath := filepath.Join(tempDir, "chunks.json")
	testChunks := []string{"Test chunk"}

	chunksData, err := json.Marshal(testChunks)
	if err != nil {
		t.Fatalf("Failed to marshal test chunks: %v", err)
	}

	if err := os.WriteFile(chunksPath, chunksData, 0o644); err != nil {
		t.Fatalf("Failed to write chunks file: %v", err)
	}

	// Create engine with unavailable server URL
	cfg := createTestConfig(server.URL)

	logger := createTestLogger(t)
	defer logger.Close()

	engine := tts.NewHTTPEngine(cfg, logger)
	defer engine.Close()

	// Test should fail due to health check failure
	outputDir := filepath.Join(tempDir, "output")

	err = engine.ProcessChunks(chunksPath, outputDir)
	if err == nil {
		t.Fatal("Expected error for unavailable service, got nil")
	}
}

// BenchmarkHTTPEngine_ProcessSingleChunk benchmarks single chunk processing performance.
func BenchmarkHTTPEngine_ProcessSingleChunk(b *testing.B) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/health" {
					responseWriter.WriteHeader(http.StatusOK)
					json.NewEncoder(responseWriter).
						Encode(map[string]any{
							"status":       "healthy",
							"model_loaded": true,
						})

					return
				}

				responseWriter.Header().Set("Content-Type", "audio/wav")
				responseWriter.WriteHeader(http.StatusOK)
				responseWriter.Write([]byte("benchmark-audio-data"))
			},
		),
	)
	defer server.Close()

	tempDir := b.TempDir()
	cfg := createTestConfig(server.URL)

	cfg.Paths.OutputDir = tempDir

	logger, err := logger.New("/tmp/benchmark-logs", "benchmark.log")
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	engine := tts.NewHTTPEngine(cfg, logger)
	defer engine.Close()

	testText := "This is benchmark text for TTS engine performance testing."

	b.ResetTimer()

	for i := range b.N {
		outputPath := filepath.Join(tempDir, fmt.Sprintf("benchmark_%d.wav", i))

		err := engine.ProcessSingleChunk(testText, outputPath)
		if err != nil {
			b.Fatalf("ProcessSingleChunk failed: %v", err)
		}
	}
}
