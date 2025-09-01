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

// Test constants.
const (
	TestHelloWorld                     = "Hello, world!"
	TestWAVHeaderMinimal               = "RIFF....WAVE"
	TestWAVPrefix                      = "RIFF"
	TestErrMsgInvalidSpeakerPath       = "Invalid speaker reference path"
	TestErrCodeInvalidSpeakerPath      = "INVALID_SPEAKER_PATH"
	TestErrExpectedPostRequest         = "Expected POST request, got %s"
	TestErrExpectedGeneratePath        = "Expected /v1/generate/speech path, got %s"
	TestErrExpectedJSONContentType     = "Expected application/json content type"
	TestErrExpectedWAVAccept           = "Expected audio/wav accept type"
	TestErrFailedToDecodeRequest       = "Failed to decode request: %v"
	TestErrExpectedHelloWorld          = "Expected 'Hello, world!', got '%s'"
	TestErrExpectedTemperature         = "Expected temperature 0.8, got %f"
	TestErrExpectedLanguage            = "Expected language 'en', got '%s'"
	TestErrGenerateSpeechFailed        = "GenerateSpeech failed: %v"
	TestErrExpectedNonEmptyAudio       = "Expected non-empty audio data"
	TestErrExpectedWAVFormat           = "Expected WAV format audio data"
	TestErrExpectedForEmptyText        = "Expected error for empty text"
	TestErrExpectedEmptyTextError      = "Expected 'text cannot be empty' error, got: %v"
	TestErrExpectedForInvalidSpeaker   = "Expected error for invalid speaker path"
	TestErrExpectedSpecificError       = "Expected specific error message, got: %v"
	TestErrExpectedErrorCode           = "Expected error code in message, got: %v"
	TestErrExpectedForWrongContentType = "Expected error for wrong content type"
	TestErrExpectedContentTypeError    = "Expected content type error, got: %v"
	TestErrExpectedHealthPath          = "Expected /health path, got %s"
	TestErrExpectedGetRequest          = "Expected GET request, got %s"
	TestErrHealthCheckFailed           = "HealthCheck failed: %v"
	TestErrExpectedForUnreachable      = "Expected error for unreachable service"
	TestErrExpectedTimeout             = "Expected timeout error"
)

func validateRequestMethod(t *testing.T, request *http.Request) {
	if request.Method != http.MethodPost {
		t.Errorf(TestErrExpectedPostRequest, request.Method)
	}

	if request.URL.Path != "/v1/generate/speech" {
		t.Errorf(TestErrExpectedGeneratePath, request.URL.Path)
	}
}

func validateRequestHeaders(t *testing.T, request *http.Request) {
	if request.Header.Get("Content-Type") != "application/json" {
		t.Error(TestErrExpectedJSONContentType)
	}

	if request.Header.Get("Accept") != "audio/wav" {
		t.Error(TestErrExpectedWAVAccept)
	}
}

func validateRequestBody(t *testing.T, request *http.Request) {
	var req tts.Request

	decodeErr := json.NewDecoder(request.Body).Decode(&req)
	if decodeErr != nil {
		t.Fatalf(TestErrFailedToDecodeRequest, decodeErr)
	}

	if req.Text != TestHelloWorld {
		t.Errorf(TestErrExpectedHelloWorld, req.Text)
	}

	if req.Temperature != 0.8 {
		t.Errorf(TestErrExpectedTemperature, req.Temperature)
	}

	if req.Language != "en" {
		t.Errorf(TestErrExpectedLanguage, req.Language)
	}
}

func sendMockWAVResponse(t *testing.T, responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "audio/wav")
	responseWriter.WriteHeader(http.StatusOK)

	_, err := responseWriter.Write([]byte(TestWAVHeaderMinimal))
	if err != nil {
		t.Fatalf("Failed to write mock WAV response: %v", err)
	}
}

func createMockTTSHandler(t *testing.T) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		validateRequestMethod(t, request)
		validateRequestHeaders(t, request)
		validateRequestBody(t, request)
		sendMockWAVResponse(t, responseWriter)
	}
}

func validateGeneratedAudio(t *testing.T, audioData []byte) {
	if len(audioData) == 0 {
		t.Error(TestErrExpectedNonEmptyAudio)
	}

	if !strings.HasPrefix(string(audioData), TestWAVPrefix) {
		t.Error(TestErrExpectedWAVFormat)
	}
}

func TestClient_GenerateSpeech_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(createMockTTSHandler(t))
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)
	req := createTestRequest()

	audioData, generateErr := client.GenerateSpeech(context.Background(), req)
	if generateErr != nil {
		t.Errorf(TestErrGenerateSpeechFailed, generateErr)
	}

	validateGeneratedAudio(t, audioData)
}

func createTestRequest() tts.Request {
	return tts.Request{
		Text:           TestHelloWorld,
		SpeakerRefPath: "",
		Temperature:    0.8,
		Language:       "en",
	}
}

func TestClient_GenerateSpeech_EmptyText(t *testing.T) {
	t.Parallel()

	client := tts.NewHTTPClient("http://localhost:8000", 10*time.Second)

	req := tts.Request{
		Text:           "",
		SpeakerRefPath: "",
		Temperature:    0.75,
		Language:       "en",
	}

	_, err := client.GenerateSpeech(context.Background(), req)
	if err == nil {
		t.Error(TestErrExpectedForEmptyText)
	}

	if !strings.Contains(err.Error(), "text cannot be empty") {
		t.Errorf(TestErrExpectedEmptyTextError, err)
	}
}

func TestClient_GenerateSpeech_ServiceError(t *testing.T) {
	t.Parallel()
	// Mock TTS service that returns an error
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.Header().
					Set("Content-Type", "application/json")
				responseWriter.WriteHeader(http.StatusBadRequest)

				errorResp := tts.ErrorResponse{
					Detail:    TestErrMsgInvalidSpeakerPath,
					ErrorCode: TestErrCodeInvalidSpeakerPath,
				}

				err := json.NewEncoder(responseWriter).Encode(errorResp)
				if err != nil {
					t.Fatalf(
						"Failed to encode mock error response: %v",
						err,
					)
				}
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)

	req := tts.Request{
		Text:           TestHelloWorld,
		SpeakerRefPath: "/invalid/path.wav",
		Temperature:    0.75,
		Language:       "en",
	}

	_, err := client.GenerateSpeech(context.Background(), req)
	if err == nil {
		t.Error(TestErrExpectedForInvalidSpeaker)
	}

	if !strings.Contains(err.Error(), TestErrMsgInvalidSpeakerPath) {
		t.Errorf(TestErrExpectedSpecificError, err)
	}

	if !strings.Contains(err.Error(), TestErrCodeInvalidSpeakerPath) {
		t.Errorf(TestErrExpectedErrorCode, err)
	}
}

func TestClient_GenerateSpeech_WrongContentType(t *testing.T) {
	t.Parallel()
	// Mock service that returns wrong content type
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.Header().
					Set("Content-Type", "text/plain")
				responseWriter.WriteHeader(http.StatusOK)

				if _, err := responseWriter.Write([]byte("Not audio data")); err != nil {
					t.Fatalf("Failed to write mock response: %v", err)
				}
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)

	req := tts.Request{
		Text:           TestHelloWorld,
		SpeakerRefPath: "",
		Temperature:    0.75,
		Language:       "en",
	}

	_, err := client.GenerateSpeech(context.Background(), req)
	if err == nil {
		t.Error(TestErrExpectedForWrongContentType)
	}

	if !strings.Contains(err.Error(), "unexpected content type") {
		t.Errorf(TestErrExpectedContentTypeError, err)
	}
}

func TestClient_HealthCheck_Success(t *testing.T) {
	t.Parallel()

	server := createHealthyMockServer(t)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 10*time.Second)

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf(TestErrHealthCheckFailed, err)
	}
}

// createHealthyMockServer creates a mock server that responds with healthy status.
func createHealthyMockServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				validateHealthRequest(t, request)
				writeHealthyResponse(t, responseWriter)
			},
		),
	)
}

// validateHealthRequest validates the incoming health check request.
func validateHealthRequest(t *testing.T, request *http.Request) {
	if request.URL.Path != "/health" {
		t.Errorf(TestErrExpectedHealthPath, request.URL.Path)
	}

	if request.Method != http.MethodGet {
		t.Errorf(TestErrExpectedGetRequest, request.Method)
	}
}

// writeHealthyResponse writes a healthy status response.
func writeHealthyResponse(t *testing.T, responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)

	healthResponse := map[string]any{
		"status":       "healthy",
		"model_loaded": true,
		"service":      "TTS",
	}

	err := json.NewEncoder(responseWriter).Encode(healthResponse)
	if err != nil {
		t.Fatalf("Failed to encode healthy response: %v", err)
	}
}

func TestClient_HealthCheck_ServiceDown(t *testing.T) {
	t.Parallel()
	// Use a non-existent server URL
	client := tts.NewHTTPClient("http://localhost:99999", 1*time.Second)

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error(TestErrExpectedForUnreachable)
	}
}

func TestClient_GenerateSpeech_Timeout(t *testing.T) {
	t.Parallel()
	// Mock service that takes too long to respond
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				time.Sleep(2 * time.Second) // Longer than client timeout
				responseWriter.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	client := tts.NewHTTPClient(server.URL, 100*time.Millisecond) // Short timeout

	req := tts.Request{
		Text:           TestHelloWorld,
		SpeakerRefPath: "",
		Temperature:    0.75,
		Language:       "en",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := client.GenerateSpeech(ctx, req)
	if err == nil {
		t.Error(TestErrExpectedTimeout)
	}
}
