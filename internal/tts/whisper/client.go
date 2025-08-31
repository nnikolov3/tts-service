// Package whisper provides Whisper API client functionality for TTS applications.
//
// This package implements Whisper integration that was previously
// handled by Python utilities, following Go coding standards and design
// principles for explicit behavior and maintainable code.
package whisper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Error messages.
const (
	errFailedToOpenFile        = "failed to open audio file: %w"
	errFailedToCloseFile       = "Warning: failed to close file: %v"
	errFailedToCreateFormFile  = "failed to create form file: %w"
	errFailedToCopyFileData    = "failed to copy file data: %w"
	errFailedToWriteModelField = "failed to write model field: %w"
	errFailedToWriteLangField  = "failed to write language field: %w"
	errFailedToCloseWriter     = "failed to close multipart writer: %w"
	errFailedToCreateRequest   = "failed to create request: %w"
	errFailedToCloseRespBody   = "Warning: failed to close response body: %v"
	errFailedToMakeRequest     = "failed to make request: %w"
	errAPIRequestFailed        = "API request failed with status %d: %s"
	errFailedToDecodeResponse  = "failed to decode response: %w"
	errFailedToWriteRespFormat = "failed to write response format field: %w"
)

// HTTP headers.
const (
	headerAuthorization = "Authorization"
	headerContentType   = "Content-Type"
)

// Form field names.
const (
	formFieldFile           = "file"
	formFieldModel          = "model"
	formFieldLanguage       = "language"
	formFieldResponseFormat = "response_format"
)

// Environment variables.
const (
	envOpenAIAPIKey = "OPENAI_API_KEY"
)

// Environment variable error messages.
const (
	errOpenAIAPIKeyNotSet = "OPENAI_API_KEY environment variable not set"
)

// Client provides Whisper API client functionality.
type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// Response represents the response from Whisper API.
type Response struct {
	Text string `json:"text"`
}

// Request represents a request to Whisper API.
type Request struct {
	File     string `json:"file"`
	Model    string `json:"model"`
	Language string `json:"language,omitempty"`
}

// NewClient creates a new Whisper API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1/audio/transcriptions",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// TranscribeFile transcribes an audio file using Whisper API.
func (c *Client) TranscribeFile(audioPath, model, language string) (string, error) {
	// Open the audio file
	file, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf(errFailedToOpenFile, err)
	}

	defer func() {
		closeErr := file.Close()
		if closeErr != nil {
			// Log the error but don't fail the operation
			log.Printf(errFailedToCloseFile, closeErr)
		}
	}()

	// Create multipart form data
	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)

	// Add file
	part, err := writer.CreateFormFile(formFieldFile, filepath.Base(audioPath))
	if err != nil {
		return "", fmt.Errorf(errFailedToCreateFormFile, err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return "", fmt.Errorf(errFailedToCopyFileData, err)
	}

	// Add model
	err = writer.WriteField(formFieldModel, model)
	if err != nil {
		return "", fmt.Errorf(errFailedToWriteModelField, err)
	}

	// Add language if specified
	if language != "" {
		err = writer.WriteField(formFieldLanguage, language)
		if err != nil {
			return "", fmt.Errorf(errFailedToWriteLangField, err)
		}
	}

	closeErr := writer.Close()
	if closeErr != nil {
		return "", fmt.Errorf(errFailedToCloseWriter, closeErr)
	}

	// Create HTTP request
	req, err := http.NewRequest(http.MethodPost, c.baseURL, &buf)
	if err != nil {
		return "", fmt.Errorf(errFailedToCreateRequest, err)
	}

	req.Header.Set(headerAuthorization, "Bearer "+c.apiKey)
	req.Header.Set(headerContentType, writer.FormDataContentType())

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf(errFailedToMakeRequest, err)
	}

	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			// Log the error but don't fail the operation
			log.Printf(errFailedToCloseRespBody, closeErr)
		}
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return "", fmt.Errorf(
			errAPIRequestFailed,
			resp.StatusCode,
			string(body),
		)
	}

	// Parse response
	var whisperResp Response
	if err := json.NewDecoder(resp.Body).Decode(&whisperResp); err != nil {
		return "", fmt.Errorf(errFailedToDecodeResponse, err)
	}

	return whisperResp.Text, nil
}

// TranscribeFileWithWordTimestamps transcribes an audio file with word-level timestamps.
func (c *Client) TranscribeFileWithWordTimestamps(
	audioPath, model, language string,
) (map[string]any, error) {
	// Open the audio file
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf(errFailedToOpenFile, err)
	}

	defer func() {
		closeErr := file.Close()
		if closeErr != nil {
			// Log the error but don't fail the operation
			log.Printf(errFailedToCloseFile, closeErr)
		}
	}()

	// Create multipart form data
	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)

	// Add file
	part, err := writer.CreateFormFile(formFieldFile, filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf(errFailedToCreateFormFile, err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf(errFailedToCopyFileData, err)
	}

	// Add model
	err = writer.WriteField(formFieldModel, model)
	if err != nil {
		return nil, fmt.Errorf(errFailedToWriteModelField, err)
	}

	// Add word timestamps flag
	err = writer.WriteField(formFieldResponseFormat, "verbose_json")
	if err != nil {
		return nil, fmt.Errorf(errFailedToWriteRespFormat, err)
	}

	// Add language if specified
	if language != "" {
		err = writer.WriteField(formFieldLanguage, language)
		if err != nil {
			return nil, fmt.Errorf(errFailedToWriteLangField, err)
		}
	}

	closeErr := writer.Close()
	if closeErr != nil {
		return nil, fmt.Errorf(errFailedToCloseWriter, closeErr)
	}

	// Create HTTP request
	req, err := http.NewRequest(http.MethodPost, c.baseURL, &buf)
	if err != nil {
		return nil, fmt.Errorf(errFailedToCreateRequest, err)
	}

	req.Header.Set(headerAuthorization, "Bearer "+c.apiKey)
	req.Header.Set(headerContentType, writer.FormDataContentType())

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(errFailedToMakeRequest, err)
	}

	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			// Log the error but don't fail the operation
			log.Printf(errFailedToCloseRespBody, closeErr)
		}
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf(
			errAPIRequestFailed,
			resp.StatusCode,
			string(body),
		)
	}

	// Parse response
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf(errFailedToDecodeResponse, err)
	}

	return result, nil
}

// TranscribeOnce is a convenience function for single transcription.
func TranscribeOnce(audioPath, model, language string) (string, error) {
	apiKey := os.Getenv(envOpenAIAPIKey)
	if apiKey == "" {
		return "", errors.New(errOpenAIAPIKeyNotSet)
	}

	client := NewClient(apiKey)

	return client.TranscribeFile(audioPath, model, language)
}

// TranscribeOnceWithWordTimestamps is a convenience function for single transcription
// with word timestamps.
func TranscribeOnceWithWordTimestamps(
	audioPath, model, language string,
) (map[string]any, error) {
	apiKey := os.Getenv(envOpenAIAPIKey)
	if apiKey == "" {
		return nil, errors.New(errOpenAIAPIKeyNotSet)
	}

	client := NewClient(apiKey)

	return client.TranscribeFileWithWordTimestamps(audioPath, model, language)
}
