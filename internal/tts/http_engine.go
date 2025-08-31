// Package tts provides HTTP-based TTS processing functionality that orchestrates
// text-to-speech generation by communicating with a standalone TTS service.
package tts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"logger"

	"tts/internal/config"
)

const (
	// Error messages, log formats, and file patterns.
	errChunksPathEmpty          = "chunks path cannot be empty"
	errFmtHealthCheckFailed     = "TTS service health check failed: %w"
	logFmtServiceHealthy        = "TTS service is healthy, processing %d chunks"
	logFmtGeneratedAudio        = "Generated audio: %s (%d bytes)"
	outputFileFormat            = "chunk_%04d.wav"
	errFmtChunkFailed           = "chunk %d failed: %w"
	logFmtChunkProcessingFailed = "Failed to process chunk %d: %v"
	logFmtChunkProcessed        = "Processed chunk %d/%d"
)

// HTTPEngine orchestrates text-to-speech processing by communicating with
// a standalone TTS HTTP service. It manages parallel processing, error handling,
// and file I/O operations while maintaining separation from the underlying
// audio generation logic.
type HTTPEngine struct {
	client *HTTPClient
	config *config.Config
	logger *logger.Logger
}

// NewHTTPEngine creates an HTTP-based TTS engine with the provided configuration.
// The engine will communicate with the TTS service at the configured URL and
// use the specified timeout for all HTTP operations.
func NewHTTPEngine(cfg *config.Config, logger *logger.Logger) *HTTPEngine {
	serviceURL := cfg.TTS.GetServiceURL()
	timeout := time.Duration(cfg.TTS.TimeoutSeconds) * time.Second

	client := NewHTTPClient(serviceURL, timeout)

	return &HTTPEngine{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// NewHTTPEngineWithClient creates an HTTP-based TTS engine with a custom client.
// This constructor is primarily for testing purposes, allowing injection of
// mock clients while maintaining the same engine behavior.
func NewHTTPEngineWithClient(
	cfg *config.Config,
	logger *logger.Logger,
	client *HTTPClient,
) *HTTPEngine {
	return &HTTPEngine{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// ProcessChunks processes a JSON file containing text chunks using the HTTP TTS service.
// Each chunk is processed in parallel according to the configured worker count.
// Output files are named sequentially (chunk_0001.wav, chunk_0002.wav, etc.).
//
// The method performs a health check before processing to fail fast if the service
// is unavailable, adhering to the "Make the common case fast" principle.
func (e *HTTPEngine) ProcessChunks(chunksPath, outputDir string) error {
	// Validate inputs at the boundary
	if chunksPath == "" {
		return errors.New(errChunksPathEmpty)
	}

	if outputDir == "" {
		return errors.New("output directory cannot be empty")
	}

	// Read chunks file
	chunks, err := e.readChunksFile(chunksPath)
	if err != nil {
		return fmt.Errorf("failed to read chunks: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check service health first
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.client.HealthCheck(ctx); err != nil {
		return fmt.Errorf(errFmtHealthCheckFailed, err)
	}

	e.logger.Info(logFmtServiceHealthy, len(chunks))

	// Process chunks in parallel
	return e.processChunksParallel(chunks, outputDir)
}

// ProcessSingleChunk processes a single text string and saves the generated
// audio to the specified output path. The method handles directory creation,
// TTS request construction, and file writing operations.
//
// This method is suitable for processing individual text inputs or as part
// of a larger batch processing workflow.
func (e *HTTPEngine) ProcessSingleChunk(text, outputPath string) error {
	// Validate inputs at the boundary
	if text == "" {
		return errors.New("text cannot be empty")
	}

	if outputPath == "" {
		return errors.New("output path cannot be empty")
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Construct TTS request using configuration defaults
	req := TTSRequest{
		Text:        text,
		Temperature: e.config.TTS.Temperature,
		Language:    "en",
	}

	// Generate speech with configured timeout
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(e.config.TTS.TimeoutSeconds)*time.Second)
	defer cancel()

	audioData, err := e.client.GenerateSpeech(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to generate speech: %w", err)
	}

	// Write audio data to output file with appropriate permissions
	if err := os.WriteFile(outputPath, audioData, 0o644); err != nil {
		return fmt.Errorf("failed to write audio file: %w", err)
	}

	e.logger.Info(logFmtGeneratedAudio, outputPath, len(audioData))

	return nil
}

// readChunksFile reads and parses a JSON file containing an array of text chunks.
// The file must contain a valid JSON array of strings, with each string representing
// a text chunk to be processed for speech generation.
func (e *HTTPEngine) readChunksFile(chunksPath string) ([]string, error) {
	data, err := os.ReadFile(chunksPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON chunks as array of strings
	var chunks []string
	if err := parseJSON(data, &chunks); err != nil {
		return nil, fmt.Errorf("failed to parse chunks JSON: %w", err)
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks found in %s", chunksPath)
	}

	return chunks, nil
}

// processChunksParallel processes chunks concurrently using a worker pool pattern.
// The number of concurrent workers is controlled by the configuration to prevent
// overwhelming the TTS service while maximizing throughput.
//
// Errors from individual chunks are captured and reported, but processing continues
// for remaining chunks to maximize the amount of work completed.
func (e *HTTPEngine) processChunksParallel(chunks []string, outputDir string) error {
	var (
		waitGroup sync.WaitGroup
		mutex     sync.Mutex
		lastError error
	)

	// Create worker pool to control concurrency
	workerPool := make(chan struct{}, e.config.TTS.Workers)

	for chunkIndex, chunk := range chunks {
		waitGroup.Add(1)

		go func(index int, text string) {
			defer waitGroup.Done()

			// Acquire worker slot to control concurrency
			workerPool <- struct{}{}

			defer func() { <-workerPool }()

			// Generate sequential output filename
			outputPath := filepath.Join(
				outputDir,
				fmt.Sprintf(outputFileFormat, index+1),
			)

			// Process individual chunk
			err := e.ProcessSingleChunk(text, outputPath)
			if err != nil {
				// Capture error while allowing other chunks to continue
				mutex.Lock()

				lastError = fmt.Errorf(
					errFmtChunkFailed,
					index+1,
					err,
				)

				mutex.Unlock()
				e.logger.Error(
					logFmtChunkProcessingFailed,
					index+1,
					err,
				)

				return
			}

			e.logger.Info(logFmtChunkProcessed, index+1, len(chunks))
		}(chunkIndex, chunk)
	}

	waitGroup.Wait()
	close(workerPool)

	return lastError
}

// Close performs cleanup operations for the HTTP engine.
// Currently a no-op as HTTP clients don't require explicit cleanup,
// but provides interface consistency for future resource management needs.
func (e *HTTPEngine) Close() error {
	return nil
}
