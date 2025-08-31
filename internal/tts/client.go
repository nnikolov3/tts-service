// Package tts provides TTS (Text-to-Speech) functionality.
//
// This package implements TTS functionality that was previously
// handled by Python utilities, following Go coding standards and design
// principles for explicit behavior and maintainable code.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// API endpoints and paths.
const (
	apiGenerateSpeech = "/v1/generate/speech"
	apiHealth         = "/health"
)

// HTTP headers.
const (
	headerContentType = "Content-Type"
	headerAccept      = "Accept"
	contentTypeJSON   = "application/json"
	contentTypeWAV    = "audio/wav"
)

// Default values.
const (
	defaultTemperature = 0.75
	defaultLanguage    = "en"
)

// Error messages.
const (
	errTextCannotBeEmpty       = "text cannot be empty"
	errUnexpectedContentType   = "unexpected content type: expected audio/wav, got %s"
	errReceivedEmptyAudio      = "received empty audio data"
	errFmtServiceErrorWithCode = "TTS service error (%s): %s (code: %s)"
	errFmtServiceNonOKStatus   = "TTS service returned non-OK status: %s, body: %s"
)

// HTTPClient represents a client for the standalone TTS HTTP service.
// It encapsulates the HTTP configuration and provides methods for
// speech generation and health monitoring.
type HTTPClient struct {
	httpClient *http.Client
	baseURL    string
}

// TTSRequest defines the JSON payload structure for TTS generation requests.
// All fields follow the explicit API contract defined in the service blueprint.
type TTSRequest struct {
	// Text contains the input text to convert to speech.
	// Must be non-empty and within reasonable length limits.
	Text string `json:"text"`

	// SpeakerRefPath optionally specifies a server-side path to a speaker
	// reference file for voice cloning. If empty, default speaker is used.
	SpeakerRefPath string `json:"speaker_ref_path,omitempty"`

	// Language specifies the target language code (e.g., "en", "es").
	// Defaults to "en" if not specified.
	Language string `json:"language"`

	// Temperature controls randomness in speech generation.
	// Valid range: 0.0 (deterministic) to 2.0 (highly random).
	Temperature float64 `json:"temperature"`
}

// TTSErrorResponse represents a structured error response from the TTS service.
// This provides actionable diagnostics when requests fail.
type TTSErrorResponse struct {
	// Detail contains a human-readable error description.
	Detail string `json:"detail"`

	// ErrorCode provides a machine-readable error classification.
	ErrorCode string `json:"error_code,omitempty"`
}

// NewHTTPClient creates and configures an HTTP client for the TTS service.
// The baseURL should include the protocol and port (e.g., "http://localhost:8000").
// The timeout applies to all HTTP requests made by this client.
func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// GenerateSpeech sends a TTS generation request and returns the raw audio data.
// This method validates input parameters, constructs the HTTP request according
// to the API contract, and handles both successful responses and error conditions.
//
// The returned audio data is in WAV format as specified by the service contract.
// Callers are responsible for writing this data to files or streaming it as needed.
func (c *HTTPClient) GenerateSpeech(ctx context.Context, req TTSRequest) ([]byte, error) {
	// Validate required input at the boundary
	if req.Text == "" {
		return nil, errors.New(errTextCannotBeEmpty)
	}

	// Set defaults for optional parameters
	if req.Temperature == 0 {
		req.Temperature = defaultTemperature
	}

	if req.Language == "" {
		req.Language = defaultLanguage
	}

	// Marshal request to JSON according to API contract
	requestBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Construct HTTP request with explicit headers
	url := c.baseURL + apiGenerateSpeech

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set explicit headers as per API contract
	httpReq.Header.Set(headerContentType, contentTypeJSON)
	httpReq.Header.Set(headerAccept, contentTypeWAV)

	// Send request with configured timeout
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to send request to TTS service at %s: %w",
			c.baseURL,
			err,
		)
	}
	defer resp.Body.Close()

	// Handle non-success status codes with structured error parsing
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrorResponse(resp)
	}

	// Validate response content type matches expected format
	contentType := resp.Header.Get("Content-Type")
	if contentType != contentTypeWAV {
		return nil, fmt.Errorf(
			errUnexpectedContentType,
			contentType,
		)
	}

	// Read and validate audio response data
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	if len(audioData) == 0 {
		return nil, errors.New(errReceivedEmptyAudio)
	}

	return audioData, nil
}

// HealthCheck verifies that the TTS service is running and operational.
// This method performs a lightweight check against the service health endpoint
// and returns an error if the service is unavailable or reports unhealthy status.
//
// Health checks should be performed before processing large workloads to fail fast
// and provide clear diagnostics when the service is unavailable.
func (c *HTTPClient) HealthCheck(ctx context.Context) error {
	url := c.baseURL + apiHealth

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"health check failed for service at %s: %w",
			c.baseURL,
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %s", resp.Status)
	}

	return nil
}

// parseErrorResponse attempts to decode a structured JSON error from the service.
// If structured parsing fails, it falls back to returning the raw response body
// to ensure diagnostic information is preserved.
func (c *HTTPClient) parseErrorResponse(resp *http.Response) error {
	var errorResp TTSErrorResponse

	err := json.NewDecoder(resp.Body).Decode(&errorResp)
	if err == nil {
		return fmt.Errorf(errFmtServiceErrorWithCode,
			resp.Status, errorResp.Detail, errorResp.ErrorCode)
	}

	// Fallback to raw response for non-JSON errors
	body, _ := io.ReadAll(resp.Body)

	return fmt.Errorf(
		errFmtServiceNonOKStatus,
		resp.Status,
		string(body),
	)
}
