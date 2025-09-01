// Package whisper provides Whisper API client functionality for TTS applications.
//
// This package implements Whisper integration that was previously
// handled by Python utilities, following Go coding standards and design
// principles for explicit behavior and maintainable code.
package whisper

import (
	"bytes"
	"context"
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

const (
	// DefaultTimeout defines the default timeout for HTTP client operations.
	DefaultTimeout = 60 * time.Second
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

// Static errors.
var (
	ErrOpenAIAPIKeyNotSet = errors.New(
		"OPENAI_API_KEY environment variable not set",
	)
	ErrAPIRequestFailed      = errors.New("API request failed")
	ErrCouldNotReadErrorBody = errors.New("could not read API error response body")
)

// Helper functions for dynamic error messages.
func newAPIRequestFailedError(statusCode int, body string) error {
	return fmt.Errorf("%w with status %d: %s", ErrAPIRequestFailed, statusCode, body)
}

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
			Transport:     nil,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       DefaultTimeout,
		},
	}
}

// TranscribeFile transcribes an audio file using Whisper API.
func (c *Client) TranscribeFile(
	ctx context.Context,
	audioPath, model, language string,
) (string, error) {
	formData, contentType, formErr := c.createBasicMultipartForm(
		audioPath,
		model,
		language,
	)
	if formErr != nil {
		return "", formErr
	}

	return c.executeBasicTranscriptionRequest(ctx, formData, contentType)
}

// TranscribeFileWithWordTimestamps transcribes an audio file with word-level timestamps.
func (c *Client) TranscribeFileWithWordTimestamps(
	ctx context.Context, audioPath, model, language string,
) (map[string]any, error) {
	formData, contentType, formErr := c.createTimestampMultipartForm(
		audioPath,
		model,
		language,
	)
	if formErr != nil {
		return nil, formErr
	}

	return c.executeTranscriptionRequest(ctx, formData, contentType)
}

func (c *Client) openAndCopyFile(audioPath string, writer *multipart.Writer) error {
	file, fileOpenErr := os.Open(audioPath)
	if fileOpenErr != nil {
		return fmt.Errorf(errFailedToOpenFile, fileOpenErr)
	}

	defer func() {
		fileCloseErr := file.Close()
		if fileCloseErr != nil {
			log.Printf(errFailedToCloseFile, fileCloseErr)
		}
	}()

	part, partErr := writer.CreateFormFile(formFieldFile, filepath.Base(audioPath))
	if partErr != nil {
		return fmt.Errorf(errFailedToCreateFormFile, partErr)
	}

	_, copyErr := io.Copy(part, file)
	if copyErr != nil {
		return fmt.Errorf(errFailedToCopyFileData, copyErr)
	}

	return nil
}

func (c *Client) addBasicFormFields(
	writer *multipart.Writer,
	model, language string,
) error {
	modelErr := writer.WriteField(formFieldModel, model)
	if modelErr != nil {
		return fmt.Errorf(errFailedToWriteModelField, modelErr)
	}

	if language != "" {
		langErr := writer.WriteField(formFieldLanguage, language)
		if langErr != nil {
			return fmt.Errorf(errFailedToWriteLangField, langErr)
		}
	}

	return nil
}

func (c *Client) createBasicMultipartForm(
	audioPath, model, language string,
) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)

	fileErr := c.openAndCopyFile(audioPath, writer)
	if fileErr != nil {
		return nil, "", fileErr
	}

	fieldsErr := c.addBasicFormFields(writer, model, language)
	if fieldsErr != nil {
		return nil, "", fieldsErr
	}

	writerCloseErr := writer.Close()
	if writerCloseErr != nil {
		return nil, "", fmt.Errorf(errFailedToCloseWriter, writerCloseErr)
	}

	return &buf, writer.FormDataContentType(), nil
}

func (c *Client) createHTTPRequest(
	ctx context.Context,
	formData *bytes.Buffer,
	contentType string,
) (*http.Request, error) {
	req, reqErr := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL,
		formData,
	)
	if reqErr != nil {
		return nil, fmt.Errorf(errFailedToCreateRequest, reqErr)
	}

	req.Header.Set(headerAuthorization, "Bearer "+c.apiKey)
	req.Header.Set(headerContentType, contentType)

	return req, nil
}

func (c *Client) handleBasicResponse(resp *http.Response) (string, error) {
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrCouldNotReadErrorBody, err)
		}

		return "", newAPIRequestFailedError(resp.StatusCode, string(body))
	}

	var whisperResp Response

	decodeErr := json.NewDecoder(resp.Body).Decode(&whisperResp)
	if decodeErr != nil {
		return "", fmt.Errorf(errFailedToDecodeResponse, decodeErr)
	}

	return whisperResp.Text, nil
}

func (c *Client) executeBasicTranscriptionRequest(
	ctx context.Context,
	formData *bytes.Buffer,
	contentType string,
) (string, error) {
	req, reqErr := c.createHTTPRequest(ctx, formData, contentType)
	if reqErr != nil {
		return "", reqErr
	}

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		return "", fmt.Errorf(errFailedToMakeRequest, doErr)
	}

	defer func() {
		respCloseErr := resp.Body.Close()
		if respCloseErr != nil {
			log.Printf(errFailedToCloseRespBody, respCloseErr)
		}
	}()

	return c.handleBasicResponse(resp)
}

func (c *Client) addTimestampFormFields(
	writer *multipart.Writer,
	model, language string,
) error {
	modelErr := writer.WriteField(formFieldModel, model)
	if modelErr != nil {
		return fmt.Errorf(errFailedToWriteModelField, modelErr)
	}

	formatErr := writer.WriteField(formFieldResponseFormat, "verbose_json")
	if formatErr != nil {
		return fmt.Errorf(errFailedToWriteRespFormat, formatErr)
	}

	if language != "" {
		langErr := writer.WriteField(formFieldLanguage, language)
		if langErr != nil {
			return fmt.Errorf(errFailedToWriteLangField, langErr)
		}
	}

	return nil
}

func (c *Client) createTimestampMultipartForm(
	audioPath, model, language string,
) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)

	fileErr := c.openAndCopyFile(audioPath, writer)
	if fileErr != nil {
		return nil, "", fileErr
	}

	fieldsErr := c.addTimestampFormFields(writer, model, language)
	if fieldsErr != nil {
		return nil, "", fieldsErr
	}

	writerCloseErr := writer.Close()
	if writerCloseErr != nil {
		return nil, "", fmt.Errorf(errFailedToCloseWriter, writerCloseErr)
	}

	return &buf, writer.FormDataContentType(), nil
}

func (c *Client) handleTimestampResponse(resp *http.Response) (map[string]any, error) {
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCouldNotReadErrorBody, err)
		}

		return nil, newAPIRequestFailedError(resp.StatusCode, string(body))
	}

	var result map[string]any

	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if decodeErr != nil {
		return nil, fmt.Errorf(errFailedToDecodeResponse, decodeErr)
	}

	return result, nil
}

func (c *Client) executeTranscriptionRequest(
	ctx context.Context,
	formData *bytes.Buffer,
	contentType string,
) (map[string]any, error) {
	req, reqErr := c.createHTTPRequest(ctx, formData, contentType)
	if reqErr != nil {
		return nil, reqErr
	}

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		return nil, fmt.Errorf(errFailedToMakeRequest, doErr)
	}

	defer func() {
		respCloseErr := resp.Body.Close()
		if respCloseErr != nil {
			log.Printf(errFailedToCloseRespBody, respCloseErr)
		}
	}()

	return c.handleTimestampResponse(resp)
}

// TranscribeOnce is a convenience function for single transcription.
func TranscribeOnce(audioPath, model, language string) (string, error) {
	apiKey := os.Getenv(envOpenAIAPIKey)
	if apiKey == "" {
		return "", ErrOpenAIAPIKeyNotSet
	}

	client := NewClient(apiKey)

	return client.TranscribeFile(context.Background(), audioPath, model, language)
}

// TranscribeOnceWithWordTimestamps is a convenience function for single transcription
// with word timestamps.
func TranscribeOnceWithWordTimestamps(
	audioPath, model, language string,
) (map[string]any, error) {
	apiKey := os.Getenv(envOpenAIAPIKey)
	if apiKey == "" {
		return nil, ErrOpenAIAPIKeyNotSet
	}

	client := NewClient(apiKey)

	return client.TranscribeFileWithWordTimestamps(
		context.Background(),
		audioPath,
		model,
		language,
	)
}
