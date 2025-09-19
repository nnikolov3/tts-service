// main package for the tts-service
package main

import (
	"fmt"
	"os"

	"github.com/book-expert/logger"
	"github.com/book-expert/tts-service/internal/config"
)

func setupLogger(logPath string) (*logger.Logger, error) {
	log, err := logger.New(logPath, "tts-service-bootstrap.log")
	if err != nil {
		return nil, fmt.Errorf("failed to create bootstrap logger: %w", err)
	}

	return log, nil
}

func run() error {
	// 1. Create a temporary logger for the bootstrap process
	bootstrapLog, err := setupLogger(os.TempDir())
	if err != nil {
		// If bootstrap logger fails, we can only print to stderr
		fmt.Fprintf(os.Stderr, "FATAL: Failed to create bootstrap logger: %v\n", err)

		return err
	}

	bootstrapLog.Info("Bootstrap logger created.")

	// 2. Load configuration using the central configurator
	cfg, err := config.Load(bootstrapLog)
	if err != nil {
		bootstrapLog.Error("Failed to load configuration: %v", err)

		return fmt.Errorf("failed to load configuration: %w", err)
	}

	bootstrapLog.Info("Configuration loaded successfully.")

	// 3. Initialize the final logger based on the loaded configuration
	finalLog, err := setupLogger(cfg.Paths.BaseLogsDir)
	if err != nil {
		bootstrapLog.Error("Failed to create final logger: %v", err)

		return fmt.Errorf("failed to create final logger: %w", err)
	}

	defer func() {
		closeErr := finalLog.Close()
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "error closing final logger: %v\n", closeErr)
		}
	}()

	// 4. Log confirmation message
	logMessage := "TTS-Service successfully initialized. Listening for jobs on subject: %s"
	finalLog.System(logMessage, cfg.NATS.TextProcessedSubject)

	// Future steps will involve setting up NATS and the main worker loop here.
	// For now, the service will start, log, and then exit.

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Service exited with error: %v\n", err)
		os.Exit(1)
	}
}
