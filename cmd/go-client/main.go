package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"logger"

	"tts/internal/config"
	"tts/internal/tts"
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

// Error and log messages.
const (
	errFailedToLoadConfig    = "Failed to load configuration: %v"
	errFailedToInitLogger    = "Failed to initialize logger: %v"
	errFailedToCreateDirs    = "Failed to create directories: %v"
	errHealthCheckFailed     = "Health check failed: %v"
	errServiceNotHealthy     = "TTS service is not healthy: %v\n"
	errServiceHealthy        = "TTS service is healthy"
	errEitherTextOrChunks    = "Either --text or --chunks must be provided"
	errCannotSpecifyBoth     = "Cannot specify both --text and --chunks"
	errFailedToProcessText   = "Failed to process text: %v"
	errFailedToProcessChunks = "Failed to process chunks: %v"
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

	cfg, logger, projectRoot, err := setup(flags.config, flags.verbose)
	if err != nil {
		return err
	}
	defer logger.Close()

	engine := tts.NewHTTPEngine(cfg, logger)
	defer engine.Close()

	logger.Info(logClientInitialized, projectRoot)

	if flags.health {
		return handleHealthCheck(cfg, logger)
	}

	return handleExecution(engine, cfg, logger, flags)
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
	startDir := "."
	if configPath != "" {
		startDir = filepath.Dir(configPath)
	}

	cfg, projectRoot, err := config.Load(startDir)
	if err != nil {
		return nil, nil, "", fmt.Errorf(errFailedToLoadConfig, err)
	}

	logFileName := logFileNameDefault
	if verbose {
		logFileName = logFileNameVerbose
	}

	logger, err := logger.New(cfg.Logging.LogDir, logFileName)
	if err != nil {
		return nil, nil, "", fmt.Errorf(errFailedToInitLogger, err)
	}

	if err := cfg.EnsureDirectories(); err != nil {
		logger.Error(errFailedToCreateDirs, err)

		return nil, nil, "", fmt.Errorf(errFailedToCreateDirs, err)
	}

	return cfg, logger, projectRoot, nil
}

// handleHealthCheck performs a service health check and prints the result.
func handleHealthCheck(cfg *config.Config, logger *logger.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := tts.NewHTTPClient(cfg.TTS.GetServiceURL(), 10*time.Second)
	err := client.HealthCheck(ctx)
	if err != nil {
		logger.Error(errHealthCheckFailed, err)
		fmt.Printf(errServiceNotHealthy, err)

		return err
	}

	fmt.Println(errServiceHealthy)

	return nil
}

// handleExecution validates flags and dispatches to the correct processing function.
func handleExecution(
	engine *tts.HTTPEngine,
	cfg *config.Config,
	logger *logger.Logger,
	flags appFlags,
) error {
	if flags.text == "" && flags.chunks == "" {
		flag.Usage()
		logger.Error(errEitherTextOrChunks)

		return errors.New(errEitherTextOrChunks)
	}

	if flags.text != "" && flags.chunks != "" {
		logger.Error(errCannotSpecifyBoth)

		return errors.New(errCannotSpecifyBoth)
	}

	if flags.text != "" {
		return processSingleText(engine, cfg, logger, flags.text, flags.output)
	}

	if flags.chunks != "" {
		return processChunks(engine, cfg, logger, flags.chunks, flags.output)
	}

	return nil
}

// processSingleText handles the logic for converting a single text string.
func processSingleText(
	engine *tts.HTTPEngine,
	cfg *config.Config,
	logger *logger.Logger,
	text, outputFlag string,
) error {
	outputPath := outputFlag
	if outputPath == "" {
		outputPath = filepath.Join(cfg.Paths.OutputDir, defaultOutputFile)
	}

	logger.Info(logProcessingSingleText, outputPath)

	err := engine.ProcessSingleChunk(text, outputPath)
	if err != nil {
		logger.Error(errFailedToProcessText, err)

		return fmt.Errorf(errFailedToProcessText, err)
	}

	logger.Info(logSuccessfullyGenerated, outputPath)
	fmt.Printf(logGenerated, outputPath)

	return nil
}

// processChunks handles the logic for converting a file of text chunks.
func processChunks(
	engine *tts.HTTPEngine,
	cfg *config.Config,
	logger *logger.Logger,
	chunksPath, outputFlag string,
) error {
	outputDir := outputFlag
	if outputDir == "" {
		outputDir = cfg.Paths.OutputDir
	}

	logger.Info(logProcessingChunks, chunksPath)
	logger.Info(logOutputDirectory, outputDir)

	err := engine.ProcessChunks(chunksPath, outputDir)
	if err != nil {
		logger.Error(errFailedToProcessChunks, err)

		return fmt.Errorf(errFailedToProcessChunks, err)
	}

	logger.Info(logSuccessfullyProcessed)
	fmt.Printf(logGeneratedAudioFiles, outputDir)

	return nil
}
