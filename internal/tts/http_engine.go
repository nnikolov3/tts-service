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

	"github.com/nnikolov3/logger"

	"tts/internal/config"
)

const (
	// HealthCheckTimeout defines the timeout for health check operations.
	HealthCheckTimeout = 10 * time.Second

	// File and directory permissions.
	filePermissions = 0o600
	dirPermissions  = 0o750
)

// Static errors.
var (
	ErrChunksPathEmpty = errors.New("chunks path cannot be empty")
	ErrOutputDirEmpty  = errors.New("output directory cannot be empty")
	ErrTextEmpty       = errors.New("text cannot be empty")
	ErrOutputPathEmpty = errors.New("output path cannot be empty")
	ErrNoChunksFound   = errors.New("no chunks found")
)

// Helper functions for dynamic error messages.
func newNoChunksFoundError(path string) error {
	return fmt.Errorf("%w in %s", ErrNoChunksFound, path)
}

const (
	// Log formats and file patterns.
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
func NewHTTPEngine(cfg *config.Config, log *logger.Logger) *HTTPEngine {
	serviceURL := cfg.TTS.GetServiceURL()
	timeout := time.Duration(cfg.TTS.TimeoutSeconds) * time.Second

	client := NewHTTPClient(serviceURL, timeout)

	return &HTTPEngine{
		client: client,
		config: cfg,
		logger: log,
	}
}

// NewHTTPEngineWithClient creates an HTTP-based TTS engine with a custom client.
// This constructor is primarily for testing purposes, allowing injection of
// mock clients while maintaining the same engine behavior.
func NewHTTPEngineWithClient(
	cfg *config.Config,
	log *logger.Logger,
	client *HTTPClient,
) *HTTPEngine {
	return &HTTPEngine{
		client: client,
		config: cfg,
		logger: log,
	}
}

// ProcessChunks processes a JSON file containing text chunks using the HTTP TTS service.
// Each chunk is processed in parallel according to the configured worker count.
// Output files are named sequentially (chunk_0001.wav, chunk_0002.wav, etc.).
//
// The method performs a health check before processing to fail fast if the service
// is unavailable, adhering to the "Make the common case fast" principle.
func (e *HTTPEngine) ProcessChunks(chunksPath, outputDir string) error {
	inputErr := e.validateChunkInputs(chunksPath, outputDir)
	if inputErr != nil {
		return inputErr
	}

	chunks, prepErr := e.prepareChunkProcessing(chunksPath, outputDir)
	if prepErr != nil {
		return prepErr
	}

	healthErr := e.checkServiceHealth()
	if healthErr != nil {
		return healthErr
	}

	e.logger.Info(logFmtServiceHealthy, len(chunks))

	return e.processChunksParallel(chunks, outputDir)
}

// ProcessSingleChunk processes a single text string and saves the generated
// audio to the specified output path. The method handles directory creation,
// TTS request construction, and file writing operations.
//
// This method is suitable for processing individual text inputs or as part
// of a larger batch processing workflow.
func (e *HTTPEngine) ProcessSingleChunk(text, outputPath string) error {
	inputErr := e.validateSingleChunkInputs(text, outputPath)
	if inputErr != nil {
		return inputErr
	}

	prepErr := e.prepareSingleChunkOutput(outputPath)
	if prepErr != nil {
		return prepErr
	}

	audioData, genErr := e.generateSpeechAudio(text)
	if genErr != nil {
		return genErr
	}

	writeErr := os.WriteFile(outputPath, audioData, filePermissions)
	if writeErr != nil {
		return fmt.Errorf("failed to write audio file: %w", writeErr)
	}

	e.logger.Info(logFmtGeneratedAudio, outputPath, len(audioData))

	return nil
}

// Close performs cleanup operations for the HTTP engine.
// Currently a no-op as HTTP clients don't require explicit cleanup,
// but provides interface consistency for future resource management needs.
func (e *HTTPEngine) Close() error {
	return nil
}

func (e *HTTPEngine) validateChunkInputs(chunksPath, outputDir string) error {
	if chunksPath == "" {
		return ErrChunksPathEmpty
	}

	if outputDir == "" {
		return ErrOutputDirEmpty
	}

	return nil
}

func (e *HTTPEngine) prepareChunkProcessing(
	chunksPath, outputDir string,
) ([]string, error) {
	chunks, chunksErr := e.readChunksFile(chunksPath)
	if chunksErr != nil {
		return nil, fmt.Errorf("failed to read chunks: %w", chunksErr)
	}

	dirErr := os.MkdirAll(outputDir, dirPermissions)
	if dirErr != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", dirErr)
	}

	return chunks, nil
}

func (e *HTTPEngine) checkServiceHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), HealthCheckTimeout)
	defer cancel()

	healthErr := e.client.HealthCheck(ctx)
	if healthErr != nil {
		return fmt.Errorf(errFmtHealthCheckFailed, healthErr)
	}

	return nil
}

func (e *HTTPEngine) validateSingleChunkInputs(text, outputPath string) error {
	if text == "" {
		return ErrTextEmpty
	}

	if outputPath == "" {
		return ErrOutputPathEmpty
	}

	return nil
}

func (e *HTTPEngine) prepareSingleChunkOutput(outputPath string) error {
	outputDir := filepath.Dir(outputPath)

	dirErr := os.MkdirAll(outputDir, dirPermissions)
	if dirErr != nil {
		return fmt.Errorf("failed to create output directory: %w", dirErr)
	}

	return nil
}

func (e *HTTPEngine) generateSpeechAudio(text string) ([]byte, error) {
	req := Request{
		Text:           text,
		SpeakerRefPath: "",
		Temperature:    e.config.TTS.Temperature,
		Language:       "en",
	}

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(e.config.TTS.TimeoutSeconds)*time.Second)
	defer cancel()

	audioData, speechErr := e.client.GenerateSpeech(ctx, req)
	if speechErr != nil {
		return nil, fmt.Errorf("failed to generate speech: %w", speechErr)
	}

	return audioData, nil
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

	err = parseJSON(data, &chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chunks JSON: %w", err)
	}

	if len(chunks) == 0 {
		return nil, newNoChunksFoundError(chunksPath)
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
