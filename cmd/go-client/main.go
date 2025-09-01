// Package main provides the TTS client command-line interface for text-to-speech
// conversion.
// This client supports both single text processing and batch processing from JSON files.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/nnikolov3/logger"

	"tts/internal/config"
	"tts/internal/tts"
)

const (
	// HealthCheckTimeout defines the timeout for health check operations.
	HealthCheckTimeout = 10 * time.Second
	// ClientTimeout defines the timeout for HTTP client operations.
	ClientTimeout = 10 * time.Second
)

// Flag descriptions and messages.
const (
	flagOutputDesc  = "Output file path (.wav)"
	flagChunksDesc  = "JSON file containing text chunks to process"
	flagConfigDesc  = "Path to project.toml (defaults to searching up directory tree)"
	flagVerboseDesc = "Enable verbose logging"
	flagHealthDesc  = "Check TTS service health and exit"
	flagTextDesc    = "Text to convert to speech"
)

// Flag names.
const (
	flagText    = "text"
	flagOutput  = "output"
	flagChunks  = "chunks"
	flagConfig  = "config"
	flagVerbose = "verbose"
	flagHealth  = "health"
)

// Static errors.
var (
	ErrFailedToLoadConfig = errors.New("failed to load configuration")
	ErrFailedToInitLogger = errors.New("failed to initialize logger")
	ErrFailedToCreateDirs = errors.New("failed to create directories")
	ErrHealthCheckFailed  = errors.New("health check failed")
	ErrEitherTextOrChunks = errors.New(
		"either --text or --chunks must be provided",
	)
	ErrCannotSpecifyBoth     = errors.New("cannot specify both --text and --chunks")
	ErrFailedToProcessText   = errors.New("failed to process text")
	ErrFailedToProcessChunks = errors.New("failed to process chunks")
)

// Error and log messages.
const (
	errHealthCheckFailed = "Health check failed: %v"
	errServiceNotHealthy = "TTS service is not healthy: %v\n"
	errServiceHealthy    = "TTS service is healthy"
)

// Log messages.
const (
	logClientInitialized     = "TTS Client initialized (project root: %s)"
	logProcessingSingleText  = "Processing single text to: %s"
	logSuccessfullyGenerated = "Successfully generated speech: %s"
	logGenerated             = "Generated: %s\n"
	logProcessingChunks      = "Processing chunks from: %s"
	logOutputDirectory       = "Output directory: %s"
	logSuccessfullyProcessed = "Successfully processed all chunks"
	logGeneratedAudioFiles   = "Generated audio files in: %s\n"
)

// File names and paths.
const (
	logFileNameDefault = "tts-client.log"
	logFileNameVerbose = "tts-client-verbose.log"
	defaultOutputFile  = "output.wav"
)

// appFlags holds the parsed command-line flag values.
type appFlags struct {
	text    string
	output  string
	chunks  string
	config  string
	verbose bool
	health  bool
}

func main() {
	err := run()
	if err != nil {
		// A logger might not be initialized yet, so use the standard log package.
		log.Fatalf("Error: %v", err)
	}
}

// run is the main application entry point, returning an error on failure.
func run() error {
	flags := parseFlags()

	cfg, lgr, projectRoot, err := setup(flags.config, flags.verbose)
	if err != nil {
		return err
	}

	defer func() {
		closeErr := lgr.Close()
		if closeErr != nil {
			log.Printf("failed to close logger: %v", closeErr)
		}
	}()

	lgr.Info(logClientInitialized, projectRoot)

	if flags.health {
		return handleHealthCheck(cfg, lgr)
	}

	return handleExecution(cfg, lgr, flags)
}

// parseFlags defines and parses command-line flags, returning them in a struct.
func parseFlags() appFlags {
	var flags appFlags
	flag.StringVar(&flags.text, flagText, "", flagTextDesc)
	flag.StringVar(&flags.output, flagOutput, "", flagOutputDesc)
	flag.StringVar(&flags.chunks, flagChunks, "", flagChunksDesc)
	flag.StringVar(&flags.config, flagConfig, "", flagConfigDesc)
	flag.BoolVar(&flags.verbose, flagVerbose, false, flagVerboseDesc)
	flag.BoolVar(&flags.health, flagHealth, false, flagHealthDesc)
	flag.Parse()

	return flags
}

// setup loads config, initializes the logger, and ensures directories exist.
func setup(
	configPath string,
	verbose bool,
) (*config.Config, *logger.Logger, string, error) {
	cfg, projectRoot, err := loadConfig(configPath)
	if err != nil {
		return nil, nil, "", err
	}

	lgr, err := initLogger(cfg, verbose)
	if err != nil {
		return nil, nil, "", err
	}

	err = ensureDirectories(cfg, lgr)
	if err != nil {
		return nil, nil, "", err
	}

	return cfg, lgr, projectRoot, nil
}

// loadConfig loads configuration from the specified path or current directory.
func loadConfig(configPath string) (*config.Config, string, error) {
	startDir := "."
	if configPath != "" {
		startDir = filepath.Dir(configPath)
	}

	cfg, projectRoot, err := config.Load(startDir)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrFailedToLoadConfig, err)
	}

	return cfg, projectRoot, nil
}

// initLogger creates a logger with the appropriate filename based on verbosity.
func initLogger(cfg *config.Config, verbose bool) (*logger.Logger, error) {
	logFileName := logFileNameDefault
	if verbose {
		logFileName = logFileNameVerbose
	}

	lgr, err := logger.New(cfg.Logging.LogDir, logFileName)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToInitLogger, err)
	}

	return lgr, nil
}

// ensureDirectories creates necessary directories and handles errors.
func ensureDirectories(cfg *config.Config, lgr *logger.Logger) error {
	err := cfg.EnsureDirectories()
	if err != nil {
		lgr.Error("%v: %v", ErrFailedToCreateDirs, err)

		return fmt.Errorf("%w: %w", ErrFailedToCreateDirs, err)
	}

	return nil
}

// handleHealthCheck performs a service health check and prints the result.
func handleHealthCheck(cfg *config.Config, lgr *logger.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), HealthCheckTimeout)
	defer cancel()

	client := tts.NewHTTPClient(cfg.TTS.GetServiceURL(), ClientTimeout)

	err := client.HealthCheck(ctx)
	if err != nil {
		lgr.Error(errHealthCheckFailed, err)
		lgr.Error(errServiceNotHealthy, err)

		return fmt.Errorf("health check failed: %w", err)
	}

	lgr.Info(errServiceHealthy)

	return nil
}

// handleExecution validates flags and dispatches to the correct processing function.
func handleExecution(
	cfg *config.Config,
	lgr *logger.Logger,
	flags appFlags,
) error {
	err := validateFlags(flags, lgr)
	if err != nil {
		return err
	}

	engine := tts.NewHTTPEngine(cfg, lgr)

	defer func() {
		closeErr := engine.Close()
		if closeErr != nil {
			lgr.Error("failed to close tts engine: %v", closeErr)
		}
	}()

	return executeProcessing(engine, cfg, lgr, flags)
}

// validateFlags checks for required and conflicting flags.
func validateFlags(flags appFlags, lgr *logger.Logger) error {
	err := checkRequiredFlags(flags)
	if err != nil {
		logError(lgr, ErrEitherTextOrChunks.Error())

		return err
	}

	err = checkConflictingFlags(flags)
	if err != nil {
		logError(lgr, ErrCannotSpecifyBoth.Error())

		return err
	}

	return nil
}

// checkRequiredFlags ensures at least one required flag is provided.
func checkRequiredFlags(flags appFlags) error {
	if flags.text == "" && flags.chunks == "" {
		flag.Usage()

		return ErrEitherTextOrChunks
	}

	return nil
}

// checkConflictingFlags ensures conflicting flags are not both provided.
func checkConflictingFlags(flags appFlags) error {
	if flags.text != "" && flags.chunks != "" {
		return ErrCannotSpecifyBoth
	}

	return nil
}

// logError logs an error message if logger is available.
func logError(lgr *logger.Logger, message string) {
	if lgr != nil {
		lgr.Error(message)
	}
}

// validateArgumentsOnly validates flags without requiring logger or other dependencies.
func validateArgumentsOnly(flags appFlags) error {
	return validateFlags(flags, nil)
}

// executeProcessing dispatches to the appropriate processing function.
func executeProcessing(
	engine *tts.HTTPEngine,
	cfg *config.Config,
	lgr *logger.Logger,
	flags appFlags,
) error {
	if flags.text != "" {
		return processSingleText(engine, cfg, lgr, flags.text, flags.output)
	}

	if flags.chunks != "" {
		return processChunks(engine, cfg, lgr, flags.chunks, flags.output)
	}

	return nil
}

// processSingleText handles the logic for converting a single text string.
func processSingleText(
	engine *tts.HTTPEngine,
	cfg *config.Config,
	lgr *logger.Logger,
	text, outputFlag string,
) error {
	outputPath := outputFlag
	if outputPath == "" {
		outputPath = filepath.Join(cfg.Paths.OutputDir, defaultOutputFile)
	}

	lgr.Info(logProcessingSingleText, outputPath)

	err := engine.ProcessSingleChunk(text, outputPath)
	if err != nil {
		lgr.Error("%v: %v", ErrFailedToProcessText, err)

		return fmt.Errorf("%w: %w", ErrFailedToProcessText, err)
	}

	lgr.Info(logSuccessfullyGenerated, outputPath)
	lgr.Info(logGenerated, outputPath)

	return nil
}

// processChunks handles the logic for converting a file of text chunks.
func processChunks(
	engine *tts.HTTPEngine,
	cfg *config.Config,
	lgr *logger.Logger,
	chunksPath, outputFlag string,
) error {
	outputDir := outputFlag
	if outputDir == "" {
		outputDir = cfg.Paths.OutputDir
	}

	lgr.Info(logProcessingChunks, chunksPath)
	lgr.Info(logOutputDirectory, outputDir)

	err := engine.ProcessChunks(chunksPath, outputDir)
	if err != nil {
		lgr.Error("%v: %v", ErrFailedToProcessChunks, err)

		return fmt.Errorf("%w: %w", ErrFailedToProcessChunks, err)
	}

	lgr.Info(logSuccessfullyProcessed)
	lgr.Info(logGeneratedAudioFiles, outputDir)

	return nil
}
