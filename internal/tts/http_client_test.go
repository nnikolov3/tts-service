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
	const testAudioData = "fake-wav-data"

	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				// Verify request method and path
				if request.Method != http.MethodPost {
					t.Errorf("Expected POST, got %s", request.Method)
				}

				if request.URL.Path != "/v1/generate/speech" {
					t.Errorf(
						"Expected /v1/generate/speech, got %s",
						request.URL.Path,
					)
				}

				// Verify request headers
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

				// Verify request body
				var req tts.TTSRequest

				err := json.NewDecoder(request.Body).Decode(&req)
				if err != nil {
					t.Fatalf("Failed to decode request: %v", err)
				}

				expectedReq := tts.TTSRequest{
					Text:        "Hello, world!",
					Language:    "en",
					Temperature: 0.75,
				}

				if req.Text != expectedReq.Text {
					t.Errorf(
						"Expected text %q, got %q",
						expectedReq.Text,
						req.Text,
					)
				}

				if req.Language != expectedReq.Language {
					t.Errorf(
						"Expected language %q, got %q",
						expectedReq.Language,
						req.Language,
					)
				}

				if req.Temperature != expectedReq.Temperature {
					t.Errorf(
						"Expected temperature %f, got %f",
						expectedReq.Temperature,
						req.Temperature,
					)
				}

				// Return mock audio response
				responseWriter.Header().Set("Content-Type", "audio/wav")
				responseWriter.WriteHeader(http.StatusOK)
				responseWriter.Write([]byte(testAudioData))
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	req := tts.TTSRequest{
		Text:        "Hello, world!",
		Language:    "en",
		Temperature: 0.75,
	}

	audioData, err := client.GenerateSpeech(ctx, req)
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

// TestHTTPClient_GenerateSpeech_EmptyText verifies validation of empty text.
func TestHTTPClient_GenerateSpeech_EmptyText(t *testing.T) {
	client := tts.NewHTTPClient("http://localhost:8000", 10*time.Second)
	ctx := context.Background()

	req := tts.TTSRequest{
		Text:        "",
		Language:    "en",
		Temperature: 0.75,
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
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)

			errorResp := tts.TTSErrorResponse{
				Detail:    "Model failed to load",
				ErrorCode: "MODEL_LOAD_ERROR",
			}
			json.NewEncoder(w).Encode(errorResp)
		}),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	req := tts.TTSRequest{
		Text:        "Hello, world!",
		Language:    "en",
		Temperature: 0.75,
	}

	_, err := client.GenerateSpeech(ctx, req)
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
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not audio data"))
		}),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	req := tts.TTSRequest{
		Text:        "Hello, world!",
		Language:    "en",
		Temperature: 0.75,
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
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "audio/wav")
			w.WriteHeader(http.StatusOK)
			// Write no content (empty response)
		}),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	req := tts.TTSRequest{
		Text:        "Hello, world!",
		Language:    "en",
		Temperature: 0.75,
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
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate a slow server that exceeds timeout
			time.Sleep(200 * time.Millisecond)
			w.Header().Set("Content-Type", "audio/wav")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("audio-data"))
		}),
	)
	defer server.Close()

	// Use a very short timeout to trigger timeout quickly
	client := tts.NewHTTPClient(server.URL, 50*time.Millisecond)
	ctx := context.Background()

	req := tts.TTSRequest{
		Text:        "Hello, world!",
		Language:    "en",
		Temperature: 0.75,
	}

	_, err := client.GenerateSpeech(ctx, req)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

// TestHTTPClient_HealthCheck_Success verifies successful health check.
func TestHTTPClient_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet {
					t.Errorf("Expected GET, got %s", request.Method)
				}

				if request.URL.Path != "/health" {
					t.Errorf(
						"Expected /health, got %s",
						request.URL.Path,
					)
				}

				responseWriter.WriteHeader(http.StatusOK)
				json.NewEncoder(responseWriter).Encode(map[string]any{
					"status":       "healthy",
					"model_loaded": true,
				})
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	ctx := context.Background()

	err := client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

// TestHTTPClient_HealthCheck_ServiceDown verifies health check failure handling.
func TestHTTPClient_HealthCheck_ServiceDown(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
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
			func(responseWriter http.ResponseWriter, request *http.Request) {
				responseWriter.Header().Set("Content-Type", "audio/wav")
				responseWriter.WriteHeader(http.StatusOK)
				responseWriter.Write(
					[]byte("mock-audio-data-for-benchmark"),
				)
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 30*time.Second)
	ctx := context.Background()

	req := tts.TTSRequest{
		Text:        "This is benchmark text for TTS generation",
		Language:    "en",
		Temperature: 0.75,
	}

	b.ResetTimer()

	for range b.N {
		_, err := client.GenerateSpeech(ctx, req)
		if err != nil {
			b.Fatalf("GenerateSpeech failed: %v", err)
		}
	}
}
