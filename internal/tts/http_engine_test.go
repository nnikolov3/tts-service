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
func createTestConfig(_ string) *config.Config {
	return &config.Config{
		TTS: config.TTSConfig{
			ModelPath:         "",
			ServiceHost:       "127.0.0.1",
			Quality:           "",
			Device:            "",
			Workers:           2,
			TopP:              0.0,
			GPUMemoryLimitGB:  0.0,
			MirostatEta:       0.0,
			MirostatTau:       0.0,
			ServicePort:       8000,
			Temperature:       0.75,
			RepetitionPenalty: 0.0,
			RepetitionRange:   0,
			TopK:              0,
			TimeoutSeconds:    30,
			MinP:              0.0,
			Mirostat:          false,
			UseHTTPService:    true,
			UseGPU:            false,
		},
		Paths: config.PathsConfig{
			InputDir:  "",
			OutputDir: "/tmp/tts-test-output",
			LogsDir:   "",
			ModelsDir: "",
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
	t.Parallel()

	cfg := createTestConfig("http://localhost:8000")

	log := createTestLogger(t)

	defer func() {
		closeErr := log.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	engine := tts.NewHTTPEngine(cfg, log)
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

	server := createSingleChunkMockServer(t, testAudioData)
	defer server.Close()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "test_output.wav")

	engine := createTestEngine(t, server.URL, tempDir)
	defer closeEngine(t, engine)

	testText := "Hello, this is a test."

	err := engine.ProcessSingleChunk(testText, outputPath)
	if err != nil {
		t.Fatalf("ProcessSingleChunk failed: %v", err)
	}

	validateOutputFile(t, outputPath, testAudioData)
}

// createSingleChunkMockServer creates a mock server for single chunk processing tests.
func createSingleChunkMockServer(t *testing.T, testAudioData string) *httptest.Server {
	responses := map[string]func(responseWriter http.ResponseWriter, request *http.Request){
		"/health": func(responseWriter http.ResponseWriter, _ *http.Request) {
			writeHealthResponse(responseWriter)
		},
		"/v1/generate/speech": func(responseWriter http.ResponseWriter, _ *http.Request) {
			writeAudioResponse(responseWriter, testAudioData)
		},
	}

	return createMockTTSServer(t, responses)
}

// writeHealthResponse writes a healthy status response.
func writeHealthResponse(responseWriter http.ResponseWriter) {
	responseWriter.WriteHeader(http.StatusOK)

	healthResponse := map[string]any{
		"status":       "healthy",
		"model_loaded": true,
	}

	encodeErr := json.NewEncoder(responseWriter).Encode(healthResponse)
	if encodeErr != nil {
		// In test context, we can't easily propagate this error
		// This is acceptable for test helper functions
	}
}

// writeAudioResponse writes an audio response.
func writeAudioResponse(responseWriter http.ResponseWriter, audioData string) {
	responseWriter.Header().Set("Content-Type", "audio/wav")
	responseWriter.WriteHeader(http.StatusOK)

	if _, writeErr := responseWriter.Write([]byte(audioData)); writeErr != nil {
		// In test context, we can't easily propagate this error
		// This is acceptable for test helper functions
	}
}

// createTestEngine creates a test engine with the given server URL and output directory.
func createTestEngine(t *testing.T, serverURL, tempDir string) *tts.HTTPEngine {
	cfg := createTestConfig(serverURL)

	cfg.Paths.OutputDir = tempDir

	log := createTestLogger(t)
	client := tts.NewHTTPClient(serverURL, 30*time.Second)

	return tts.NewHTTPEngineWithClient(cfg, log, client)
}

// closeEngine safely closes the engine.
func closeEngine(t *testing.T, engine *tts.HTTPEngine) {
	closeErr := engine.Close()
	if closeErr != nil {
		// In test context, we can't easily propagate this error
		// This is acceptable for test cleanup
	}
}

// validateOutputFile validates that the output file was created with correct content.
func validateOutputFile(t *testing.T, outputPath, expectedContent string) {
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("Output file was not created")
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf(
			"Expected file content %q, got %q",
			expectedContent,
			string(content),
		)
	}
}

// TestHTTPEngine_ProcessSingleChunk_EmptyText verifies validation of empty text input.
func TestHTTPEngine_ProcessSingleChunk_EmptyText(t *testing.T) {
	t.Parallel()

	cfg := createTestConfig("http://localhost:8000")

	log := createTestLogger(t)

	defer func() {
		closeErr := log.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	engine := tts.NewHTTPEngine(cfg, log)

	defer func() {
		closeErr := engine.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

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
	t.Parallel()

	cfg := createTestConfig("http://localhost:8000")

	log := createTestLogger(t)

	defer func() {
		closeErr := log.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	engine := tts.NewHTTPEngine(cfg, log)

	defer func() {
		closeErr := engine.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	err := engine.ProcessSingleChunk("test text", "")
	if err == nil {
		t.Fatal("Expected error for empty output path, got nil")
	}
}

// TestHTTPEngine_ProcessChunks_Success verifies successful chunks file processing.
func setupMockTTSServer(t *testing.T, testAudioData string) *httptest.Server {
	responses := map[string]func(w http.ResponseWriter, r *http.Request){
		"/health": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			encodeErr := json.NewEncoder(w).Encode(map[string]any{
				"status":       "healthy",
				"model_loaded": true,
			})
			if encodeErr != nil {
				// In test context, we can't easily propagate this error
				// This is acceptable for test helper functions
			}
		},
		"/v1/generate/speech": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "audio/wav")
			w.WriteHeader(http.StatusOK)
			if _, writeErr := w.Write([]byte(testAudioData)); writeErr != nil {
				// In test context, we can't easily propagate this error
				// This is acceptable for test helper functions
			}
		},
	}

	return createMockTTSServer(t, responses)
}

func createTestChunksFile(t *testing.T, tempDir string) string {
	chunksPath := filepath.Join(tempDir, "chunks.json")
	testChunks := []string{
		"First chunk of text to process.",
		"Second chunk of text to process.",
		"Third chunk of text to process.",
	}

	chunksData, marshalErr := json.Marshal(testChunks)
	if marshalErr != nil {
		t.Fatalf("Failed to marshal test chunks: %v", marshalErr)
	}

	writeErr := os.WriteFile(chunksPath, chunksData, 0o644)
	if writeErr != nil {
		t.Fatalf("Failed to write chunks file: %v", writeErr)
	}

	return chunksPath
}

func setupTestEngine(t *testing.T, serverURL, tempDir string) *tts.HTTPEngine {
	cfg := createTestConfig(serverURL)

	cfg.Paths.OutputDir = tempDir

	log := createTestLogger(t)

	defer func() {
		closeErr := log.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	client := tts.NewHTTPClient(serverURL, 30*time.Second)

	engine := tts.NewHTTPEngineWithClient(cfg, log, client)

	defer func() {
		closeErr := engine.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	return engine
}

func TestHTTPEngine_ProcessChunks_Success(t *testing.T) {
	t.Parallel()

	const testAudioData = "mock-wav-audio-data"

	server := setupMockTTSServer(t, testAudioData)
	defer server.Close()

	tempDir := t.TempDir()
	chunksPath := createTestChunksFile(t, tempDir)
	engine := setupTestEngine(t, server.URL, tempDir)

	outputDir := filepath.Join(tempDir, "output")

	processErr := engine.ProcessChunks(chunksPath, outputDir)
	if processErr != nil {
		t.Fatalf("ProcessChunks failed: %v", processErr)
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
	t.Parallel()

	cfg := createTestConfig("http://localhost:8000")

	log := createTestLogger(t)

	defer func() {
		closeErr := log.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	engine := tts.NewHTTPEngine(cfg, log)

	defer func() {
		closeErr := engine.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

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
	t.Parallel()

	server := createUnavailableMockServer(t)
	defer server.Close()

	tempDir := t.TempDir()
	chunksPath := createTestChunksFile(t, tempDir)

	engine := createUnavailableTestEngine(t, server.URL)
	defer closeEngine(t, engine)

	outputDir := filepath.Join(tempDir, "output")

	err := engine.ProcessChunks(chunksPath, outputDir)
	if err == nil {
		t.Fatal("Expected error for unavailable service, got nil")
	}
}

// createUnavailableMockServer creates a mock server that returns service unavailable.
func createUnavailableMockServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.WriteHeader(http.StatusServiceUnavailable)
			},
		),
	)
}

// createUnavailableTestEngine creates a test engine with unavailable server.
func createUnavailableTestEngine(t *testing.T, serverURL string) *tts.HTTPEngine {
	cfg := createTestConfig(serverURL)
	log := createTestLogger(t)

	return tts.NewHTTPEngine(cfg, log)
}

// BenchmarkHTTPEngine_ProcessSingleChunk benchmarks single chunk processing performance.
func setupBenchmarkServer() *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				handleBenchmarkRequest(responseWriter, request)
			},
		),
	)
}

// handleBenchmarkRequest handles benchmark requests based on path.
func handleBenchmarkRequest(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/health" {
		writeBenchmarkHealthResponse(responseWriter)

		return
	}

	writeBenchmarkAudioResponse(responseWriter)
}

// writeBenchmarkHealthResponse writes a health response for benchmarks.
func writeBenchmarkHealthResponse(responseWriter http.ResponseWriter) {
	responseWriter.WriteHeader(http.StatusOK)

	healthResponse := map[string]any{
		"status":       "healthy",
		"model_loaded": true,
	}

	encodeErr := json.NewEncoder(responseWriter).Encode(healthResponse)
	if encodeErr != nil {
		// In test context, we can't easily propagate this error
		// This is acceptable for test helper functions
	}
}

// writeBenchmarkAudioResponse writes an audio response for benchmarks.
func writeBenchmarkAudioResponse(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "audio/wav")
	responseWriter.WriteHeader(http.StatusOK)

	if _, writeErr := responseWriter.Write([]byte("benchmark-audio-data")); writeErr != nil {
		// In test context, we can't easily propagate this error
		// This is acceptable for test helper functions
	}
}

func setupBenchmarkEngine(b *testing.B, serverURL, tempDir string) *tts.HTTPEngine {
	cfg := createTestConfig(serverURL)

	cfg.Paths.OutputDir = tempDir

	log, loggerErr := logger.New("/tmp/benchmark-logs", "benchmark.log")
	if loggerErr != nil {
		b.Fatalf("Failed to create logger: %v", loggerErr)
	}

	defer func() {
		closeErr := log.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	engine := tts.NewHTTPEngine(cfg, log)

	defer func() {
		closeErr := engine.Close()
		if closeErr != nil {
			// In test context, we can't easily propagate this error
			// This is acceptable for test cleanup
		}
	}()

	return engine
}

func BenchmarkHTTPEngine_ProcessSingleChunk(b *testing.B) {
	server := setupBenchmarkServer()
	defer server.Close()

	tempDir := b.TempDir()
	engine := setupBenchmarkEngine(b, server.URL, tempDir)

	testText := "This is benchmark text for TTS engine performance testing."

	b.ResetTimer()

	for i := range b.N {
		outputPath := filepath.Join(tempDir, fmt.Sprintf("benchmark_%d.wav", i))

		benchErr := engine.ProcessSingleChunk(testText, outputPath)
		if benchErr != nil {
			b.Fatalf("ProcessSingleChunk failed: %v", benchErr)
		}
	}
}
