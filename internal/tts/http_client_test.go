package tts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tts/internal/tts"
)

// TestNewHTTPClient verifies HTTP client creation with proper configuration.
func TestNewHTTPClient(t *testing.T) {
	t.Parallel()

	const (
		testBaseURL = "http://localhost:8000"
		testTimeout = 30 * time.Second
	)

	client := tts.NewHTTPClient(testBaseURL, testTimeout)

	if client == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
}

// TestHTTPClient_GenerateSpeech_Success verifies successful speech generation.
func TestHTTPClient_GenerateSpeech_Success(t *testing.T) {
	t.Parallel()

	const testAudioData = "fake-wav-data"

	server := httptest.NewServer(createSuccessHandler(t, testAudioData))
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	req := createStandardTestRequest()

	audioData, err := client.GenerateSpeech(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateSpeech failed: %v", err)
	}

	if string(audioData) != testAudioData {
		t.Errorf(
			"Expected audio data %q, got %q",
			testAudioData,
			string(audioData),
		)
	}
}

func createSuccessHandler(t *testing.T, testAudioData string) http.HandlerFunc {
	t.Helper()

	return http.HandlerFunc(
		func(responseWriter http.ResponseWriter, request *http.Request) {
			validateHTTPRequestMethod(t, request)
			validateHTTPRequestHeaders(t, request)
			validateHTTPRequestBody(t, request)
			sendSuccessResponse(t, responseWriter, testAudioData)
		},
	)
}

func validateHTTPRequestMethod(t *testing.T, request *http.Request) {
	t.Helper()

	if request.Method != http.MethodPost {
		t.Errorf("Expected POST, got %s", request.Method)
	}

	if request.URL.Path != "/v1/generate/speech" {
		t.Errorf(
			"Expected /v1/generate/speech, got %s",
			request.URL.Path,
		)
	}
}

func validateHTTPRequestHeaders(t *testing.T, request *http.Request) {
	t.Helper()

	if contentType := request.Header.Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"Expected Content-Type application/json, got %s",
			contentType,
		)
	}

	if accept := request.Header.Get("Accept"); accept != "audio/wav" {
		t.Errorf(
			"Expected Accept audio/wav, got %s",
			accept,
		)
	}
}

func validateHTTPRequestBody(t *testing.T, request *http.Request) {
	t.Helper()

	var req tts.Request

	err := json.NewDecoder(request.Body).Decode(&req)
	if err != nil {
		t.Fatalf("Failed to decode request: %v", err)
	}

	expectedReq := createStandardTestRequest()
	validateRequestFields(t, req, expectedReq)
}

func validateRequestFields(t *testing.T, actual, expected tts.Request) {
	t.Helper()

	if actual.Text != expected.Text {
		t.Errorf(
			"Expected text %q, got %q",
			expected.Text,
			actual.Text,
		)
	}

	if actual.Language != expected.Language {
		t.Errorf(
			"Expected language %q, got %q",
			expected.Language,
			actual.Language,
		)
	}

	if actual.Temperature != expected.Temperature {
		t.Errorf(
			"Expected temperature %f, got %f",
			expected.Temperature,
			actual.Temperature,
		)
	}
}

func createStandardTestRequest() tts.Request {
	return tts.Request{
		Text:           "Hello, world!",
		SpeakerRefPath: "",
		Language:       "en",
		Temperature:    0.75,
	}
}

func sendSuccessResponse(
	t *testing.T,
	responseWriter http.ResponseWriter,
	testAudioData string,
) {
	t.Helper()
	responseWriter.Header().Set("Content-Type", "audio/wav")
	responseWriter.WriteHeader(http.StatusOK)

	_, err := responseWriter.Write([]byte(testAudioData))
	if err != nil {
		t.Fatalf("Failed to write mock success response: %v", err)
	}
}

// TestHTTPClient_GenerateSpeech_EmptyText verifies validation of empty text.
func TestHTTPClient_GenerateSpeech_EmptyText(t *testing.T) {
	t.Parallel()

	client := tts.NewHTTPClient("http://localhost:8000", 10*time.Second)
	ctx := context.Background()

	req := tts.Request{
		Text:           "",
		SpeakerRefPath: "",
		Language:       "en",
		Temperature:    0.75,
	}

	_, err := client.GenerateSpeech(ctx, req)
	if err == nil {
		t.Fatal("Expected error for empty text, got nil")
	}

	if !strings.Contains(err.Error(), "text cannot be empty") {
		t.Errorf("Expected 'text cannot be empty' error, got: %v", err)
	}
}

// TestHTTPClient_GenerateSpeech_ServerError verifies proper error handling.
func TestHTTPClient_GenerateSpeech_ServerError(t *testing.T) {
	t.Parallel()

	server := createServerErrorMockServer(t)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	req := tts.Request{
		Text:           "Hello, world!",
		SpeakerRefPath: "",
		Language:       "en",
		Temperature:    0.75,
	}

	_, err := client.GenerateSpeech(ctx, req)
	validateServerErrorResponse(t, err)
}

// createServerErrorMockServer creates a mock server that returns server error.
func createServerErrorMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				writeServerErrorResponse(t, responseWriter)
			},
		),
	)
}

// writeServerErrorResponse writes a server error response.
func writeServerErrorResponse(t *testing.T, responseWriter http.ResponseWriter) {
	t.Helper()
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusInternalServerError)

	errorResp := tts.ErrorResponse{
		Detail:    "Model failed to load",
		ErrorCode: "MODEL_LOAD_ERROR",
	}

	err := json.NewEncoder(responseWriter).Encode(errorResp)
	if err != nil {
		t.Fatalf("Failed to encode mock server error response: %v", err)
	}
}

// validateServerErrorResponse validates the server error response.
func validateServerErrorResponse(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("Expected error for server error, got nil")
	}

	expectedSubstrings := []string{
		"TTS service error",
		"Model failed to load",
		"MODEL_LOAD_ERROR",
	}

	for _, substring := range expectedSubstrings {
		if !strings.Contains(err.Error(), substring) {
			t.Errorf("Expected error to contain %q, got: %v", substring, err)
		}
	}
}

// TestHTTPClient_GenerateSpeech_WrongContentType verifies content type validation.
func TestHTTPClient_GenerateSpeech_WrongContentType(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.Header().Set("Content-Type", "text/plain")
				responseWriter.WriteHeader(http.StatusOK)

				_, err := responseWriter.Write([]byte("not audio data"))
				if err != nil {
					t.Fatalf(
						"Failed to write mock wrong content type response: %v",
						err,
					)
				}
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	req := tts.Request{
		Text:           "Hello, world!",
		Language:       "en",
		Temperature:    0.75,
		SpeakerRefPath: "",
	}

	_, err := client.GenerateSpeech(ctx, req)
	if err == nil {
		t.Fatal("Expected error for wrong content type, got nil")
	}

	if !strings.Contains(err.Error(), "unexpected content type") {
		t.Errorf("Expected 'unexpected content type' error, got: %v", err)
	}
}

// TestHTTPClient_GenerateSpeech_EmptyAudioData verifies empty response handling.
func TestHTTPClient_GenerateSpeech_EmptyAudioData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.Header().Set("Content-Type", "audio/wav")
				responseWriter.WriteHeader(http.StatusOK)
				// Write no content (empty response)
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	req := tts.Request{
		Text:           "Hello, world!",
		Language:       "en",
		Temperature:    0.75,
		SpeakerRefPath: "",
	}

	_, err := client.GenerateSpeech(ctx, req)
	if err == nil {
		t.Fatal("Expected error for empty audio data, got nil")
	}

	if !strings.Contains(err.Error(), "received empty audio data") {
		t.Errorf("Expected 'received empty audio data' error, got: %v", err)
	}
}

// TestHTTPClient_GenerateSpeech_Timeout verifies timeout handling.
func TestHTTPClient_GenerateSpeech_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				// Simulate a slow server that exceeds timeout
				time.Sleep(200 * time.Millisecond)
				responseWriter.Header().Set("Content-Type", "audio/wav")
				responseWriter.WriteHeader(http.StatusOK)

				_, err := responseWriter.Write([]byte("audio-data"))
				if err != nil {
					t.Fatalf(
						"Failed to write mock timeout response: %v",
						err,
					)
				}
			},
		),
	)
	defer server.Close()

	// Use a very short timeout to trigger timeout quickly
	client := tts.NewHTTPClient(server.URL, 50*time.Millisecond)
	ctx := context.Background()

	req := tts.Request{
		Text:           "Hello, world!",
		Language:       "en",
		Temperature:    0.75,
		SpeakerRefPath: "",
	}

	_, err := client.GenerateSpeech(ctx, req)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

// TestHTTPClient_HealthCheck_Success verifies successful health check.
func TestHTTPClient_HealthCheck_Success(t *testing.T) {
	t.Parallel()

	server := createHealthCheckMockServer(t)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

// createHealthCheckMockServer creates a mock server for health check tests.
func createHealthCheckMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				validateHealthCheckRequest(t, request)
				writeHealthCheckResponse(t, responseWriter)
			},
		),
	)
}

// validateHealthCheckRequest validates the health check request.
func validateHealthCheckRequest(t *testing.T, request *http.Request) {
	t.Helper()

	if request.Method != http.MethodGet {
		t.Errorf("Expected GET, got %s", request.Method)
	}

	if request.URL.Path != "/health" {
		t.Errorf("Expected /health, got %s", request.URL.Path)
	}
}

// writeHealthCheckResponse writes a successful health check response.
func writeHealthCheckResponse(t *testing.T, responseWriter http.ResponseWriter) {
	t.Helper()
	responseWriter.WriteHeader(http.StatusOK)

	healthResponse := map[string]any{
		"status":       "healthy",
		"model_loaded": true,
	}

	err := json.NewEncoder(responseWriter).Encode(healthResponse)
	if err != nil {
		t.Fatalf("Failed to encode mock health response: %v", err)
	}
}

// TestHTTPClient_HealthCheck_ServiceDown verifies health check failure handling.
func TestHTTPClient_HealthCheck_ServiceDown(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.WriteHeader(http.StatusServiceUnavailable)
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	if err == nil {
		t.Fatal("Expected error for service unavailable, got nil")
	}

	if !strings.Contains(err.Error(), "health check failed with status") {
		t.Errorf("Expected 'health check failed with status' error, got: %v", err)
	}
}

// TestHTTPClient_HealthCheck_NetworkError verifies network error handling.
func TestHTTPClient_HealthCheck_NetworkError(t *testing.T) {
	t.Parallel()
	// Use an invalid URL to simulate network error
	client := tts.NewHTTPClient("http://invalid-host:9999", 1*time.Second)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	if err == nil {
		t.Fatal("Expected network error, got nil")
	}
}

// BenchmarkHTTPClient_GenerateSpeech benchmarks speech generation performance.
func BenchmarkHTTPClient_GenerateSpeech(b *testing.B) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.Header().Set("Content-Type", "audio/wav")
				responseWriter.WriteHeader(http.StatusOK)

				_, err := responseWriter.Write(
					[]byte("mock-audio-data-for-benchmark"),
				)
				if err != nil {
					b.Fatalf(
						"Failed to write mock benchmark response: %v",
						err,
					)
				}
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 30*time.Second)
	ctx := context.Background()

	req := tts.Request{
		Text:           "This is benchmark text for TTS generation",
		Language:       "en",
		Temperature:    0.75,
		SpeakerRefPath: "",
	}

	b.ResetTimer()

	for range b.N {
		_, err := client.GenerateSpeech(ctx, req)
		if err != nil {
			b.Fatalf("GenerateSpeech failed: %v", err)
		}
	}
}
