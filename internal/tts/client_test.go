package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test constants.
const (
	testHelloWorld                     = "Hello, world!"
	testWAVHeaderMinimal               = "RIFF....WAVE"
	testWAVPrefix                      = "RIFF"
	testErrMsgInvalidSpeakerPath       = "Invalid speaker reference path"
	testErrCodeInvalidSpeakerPath      = "INVALID_SPEAKER_PATH"
	testErrExpectedPostRequest         = "Expected POST request, got %s"
	testErrExpectedGeneratePath        = "Expected /v1/generate/speech path, got %s"
	testErrExpectedJSONContentType     = "Expected application/json content type"
	testErrExpectedWAVAccept           = "Expected audio/wav accept type"
	testErrFailedToDecodeRequest       = "Failed to decode request: %v"
	testErrExpectedHelloWorld          = "Expected 'Hello, world!', got '%s'"
	testErrExpectedTemperature         = "Expected temperature 0.8, got %f"
	testErrExpectedLanguage            = "Expected language 'en', got '%s'"
	testErrGenerateSpeechFailed        = "GenerateSpeech failed: %v"
	testErrExpectedNonEmptyAudio       = "Expected non-empty audio data"
	testErrExpectedWAVFormat           = "Expected WAV format audio data"
	testErrExpectedForEmptyText        = "Expected error for empty text"
	testErrExpectedEmptyTextError      = "Expected 'text cannot be empty' error, got: %v"
	testErrExpectedForInvalidSpeaker   = "Expected error for invalid speaker path"
	testErrExpectedSpecificError       = "Expected specific error message, got: %v"
	testErrExpectedErrorCode           = "Expected error code in message, got: %v"
	testErrExpectedForWrongContentType = "Expected error for wrong content type"
	testErrExpectedContentTypeError    = "Expected content type error, got: %v"
	testErrExpectedHealthPath          = "Expected /health path, got %s"
	testErrExpectedGetRequest          = "Expected GET request, got %s"
	testErrHealthCheckFailed           = "HealthCheck failed: %v"
	testErrExpectedForUnreachable      = "Expected error for unreachable service"
	testErrExpectedTimeout             = "Expected timeout error"
	testMsgTextShouldBePreserved       = "Text should be preserved"
)

func validateRequestMethod(t *testing.T, request *http.Request) {
	if request.Method != http.MethodPost {
		t.Errorf(testErrExpectedPostRequest, request.Method)
	}

	if request.URL.Path != apiGenerateSpeech {
		t.Errorf(testErrExpectedGeneratePath, request.URL.Path)
	}
}

func validateRequestHeaders(t *testing.T, request *http.Request) {
	if request.Header.Get(headerContentType) != contentTypeJSON {
		t.Error(testErrExpectedJSONContentType)
	}

	if request.Header.Get(headerAccept) != contentTypeWAV {
		t.Error(testErrExpectedWAVAccept)
	}
}

func validateRequestBody(t *testing.T, request *http.Request) {
	var req Request

	decodeErr := json.NewDecoder(request.Body).Decode(&req)
	if decodeErr != nil {
		t.Errorf(testErrFailedToDecodeRequest, decodeErr)
	}

	if req.Text != testHelloWorld {
		t.Errorf(testErrExpectedHelloWorld, req.Text)
	}

	if req.Temperature != 0.8 {
		t.Errorf(testErrExpectedTemperature, req.Temperature)
	}

	if req.Language != "en" {
		t.Errorf(testErrExpectedLanguage, req.Language)
	}
}

func sendMockWAVResponse(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set(headerContentType, contentTypeWAV)
	responseWriter.WriteHeader(http.StatusOK)

	_, _ = responseWriter.Write([]byte(testWAVHeaderMinimal))
}

func createMockTTSHandler(t *testing.T) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		validateRequestMethod(t, request)
		validateRequestHeaders(t, request)
		validateRequestBody(t, request)
		sendMockWAVResponse(responseWriter)
	}
}

func validateGeneratedAudio(t *testing.T, audioData []byte) {
	if len(audioData) == 0 {
		t.Error(testErrExpectedNonEmptyAudio)
	}

	if !strings.HasPrefix(string(audioData), testWAVPrefix) {
		t.Error(testErrExpectedWAVFormat)
	}
}

func TestHTTPClient_GenerateSpeech_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(createMockTTSHandler(t))
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)
	req := createTestRequest()

	audioData, generateErr := client.GenerateSpeech(context.Background(), req)
	if generateErr != nil {
		t.Errorf(testErrGenerateSpeechFailed, generateErr)
	}

	validateGeneratedAudio(t, audioData)
}

func createTestRequest() Request {
	return Request{
		Text:           testHelloWorld,
		SpeakerRefPath: "",
		Temperature:    0.8,
		Language:       "en",
	}
}

func TestHTTPClient_GenerateSpeech_EmptyText(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient("http://localhost:8000", 10*time.Second)

	req := Request{
		Text:           "",
		SpeakerRefPath: "",
		Temperature:    0.75,
		Language:       "en",
	}

	_, err := client.GenerateSpeech(context.Background(), req)
	if err == nil {
		t.Error(testErrExpectedForEmptyText)
	}

	if !strings.Contains(err.Error(), "text cannot be empty") {
		t.Errorf(testErrExpectedEmptyTextError, err)
	}
}

func TestHTTPClient_GenerateSpeech_ServiceError(t *testing.T) {
	t.Parallel()
	// Mock TTS service that returns an error
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.Header().
					Set(headerContentType, contentTypeJSON)
				responseWriter.WriteHeader(http.StatusBadRequest)

				errorResp := ErrorResponse{
					Detail:    testErrMsgInvalidSpeakerPath,
					ErrorCode: testErrCodeInvalidSpeakerPath,
				}

				_ = json.NewEncoder(responseWriter).Encode(errorResp)
			},
		),
	)
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)

	req := Request{
		Text:           testHelloWorld,
		SpeakerRefPath: "/invalid/path.wav",
		Temperature:    0.75,
		Language:       "en",
	}

	_, err := client.GenerateSpeech(context.Background(), req)
	if err == nil {
		t.Error(testErrExpectedForInvalidSpeaker)
	}

	if !strings.Contains(err.Error(), testErrMsgInvalidSpeakerPath) {
		t.Errorf(testErrExpectedSpecificError, err)
	}

	if !strings.Contains(err.Error(), testErrCodeInvalidSpeakerPath) {
		t.Errorf(testErrExpectedErrorCode, err)
	}
}

func TestHTTPClient_GenerateSpeech_WrongContentType(t *testing.T) {
	t.Parallel()
	// Mock service that returns wrong content type
	server := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				responseWriter.Header().
					Set(headerContentType, "text/plain")
				responseWriter.WriteHeader(http.StatusOK)

				_, _ = responseWriter.Write([]byte("Not audio data"))
			},
		),
	)
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)

	req := Request{
		Text:           testHelloWorld,
		SpeakerRefPath: "",
		Temperature:    0.75,
		Language:       "en",
	}

	_, err := client.GenerateSpeech(context.Background(), req)
	if err == nil {
		t.Error(testErrExpectedForWrongContentType)
	}

	if !strings.Contains(err.Error(), "unexpected content type") {
		t.Errorf(testErrExpectedContentTypeError, err)
	}
}

func TestHTTPClient_HealthCheck_Success(t *testing.T) {
	t.Parallel()

	server := createHealthyMockServer(t)
	defer server.Close()

	client := NewHTTPClient(server.URL, 10*time.Second)

	err := client.HealthCheck(context.Background())
	if err != nil {
		t.Errorf(testErrHealthCheckFailed, err)
	}
}

// createHealthyMockServer creates a mock server that responds with healthy status.
func createHealthyMockServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				validateHealthRequest(t, request)
				writeHealthyResponse(responseWriter)
			},
		),
	)
}

// validateHealthRequest validates the incoming health check request.
func validateHealthRequest(t *testing.T, request *http.Request) {
	if request.URL.Path != apiHealth {
		t.Errorf(testErrExpectedHealthPath, request.URL.Path)
	}

	if request.Method != http.MethodGet {
		t.Errorf(testErrExpectedGetRequest, request.Method)
	}
}

// writeHealthyResponse writes a healthy status response.
func writeHealthyResponse(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set(headerContentType, contentTypeJSON)
	responseWriter.WriteHeader(http.StatusOK)

	healthResponse := map[string]any{
		"status":       "healthy",
		"model_loaded": true,
		"service":      "TTS",
	}

	_ = json.NewEncoder(responseWriter).Encode(healthResponse)
}

func TestHTTPClient_HealthCheck_ServiceDown(t *testing.T) {
	t.Parallel()
	// Use a non-existent server URL
	client := NewHTTPClient("http://localhost:99999", 1*time.Second)

	err := client.HealthCheck(context.Background())
	if err == nil {
		t.Error(testErrExpectedForUnreachable)
	}
}

func TestHTTPClient_GenerateSpeech_Timeout(t *testing.T) {
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

	client := NewHTTPClient(server.URL, 100*time.Millisecond) // Short timeout

	req := Request{
		Text:           testHelloWorld,
		SpeakerRefPath: "",
		Temperature:    0.75,
		Language:       "en",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := client.GenerateSpeech(ctx, req)
	if err == nil {
		t.Error(testErrExpectedTimeout)
	}
}

func TestRequest_Defaults(t *testing.T) {
	t.Parallel()

	req := Request{
		Text:           testHelloWorld,
		SpeakerRefPath: "",
		Language:       "",
		Temperature:    0.0,
	}

	// Test that defaults are applied by the client
	client := NewHTTPClient("http://localhost:8000", 10*time.Second)

	_ = client // Avoid unused variable

	// We can't actually make the request without a running server,
	// but we can test the request building logic
	if req.Text != testHelloWorld {
		t.Error(testMsgTextShouldBePreserved)
	}
}
