// Package tts provides TTS (Text-to-Speech) functionality.
//
// This package implements TTS functionality that was previously
// handled by Python utilities, following Go coding standards and design
// principles for explicit behavior and maintainable code.
package tts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Report section headers and messages.
const (
	reportHeaderAnalysis  = "=== OuteTTS Cleanup Analysis ==="
	reportHeaderSummary   = "=== Summary ==="
	reportFilesRemoved    = "📁 Files that can be removed (replaced by Go):"
	reportDirsRemoved     = "📂 Directories that can be removed:"
	reportFilesKept       = "🔒 Essential files to keep (ML functionality):"
	reportDirsKept        = "🔒 Essential directories to keep:"
	reportRecommendations = "💡 Recommendations:"
	reportErrors          = "❌ Errors encountered:"
)

// Summary format strings.
const (
	summaryFilesToRemove   = "Files to remove: %d\n"
	summaryDirsToRemove    = "Dirs to remove: %d\n"
	summaryFilesToKeep     = "Files to keep: %d\n"
	summaryDirsToKeep      = "Dirs to keep: %d\n"
	summaryRecommendations = "Recommendations: %d\n"
)

// List item format.
const (
	listItemFormat = "  - %s\n"
)

// Error messages.
const (
	errMissingGoImpl     = "missing Go implementations: %s"
	replacementFormat    = "%s -> %s"
	replacementSeparator = ", "
)

// CleanupReport provides information about cleanup opportunities.
type CleanupReport struct {
	RemovedFiles    []string
	RemovedDirs     []string
	KeptFiles       []string
	KeptDirs        []string
	Recommendations []string
	Errors          []string
}

// AnalyzeOuteTTSCleanup analyzes what can be cleaned up from OuteTTS Python
// implementation.
func AnalyzeOuteTTSCleanup(outettsDir string) (*CleanupReport, error) {
	report := &CleanupReport{}

	// Define what can be removed (replaced by Go implementations)
	removableFiles := []string{
		"utils/chunking.py",      // Replaced by internal/chunking/chunking.go
		"utils/preprocessing.py", // Can be implemented in Go
		"utils/helpers.py",       // Simple utilities, can be Go
		"anyascii/__init__.py",   // Can be pure Go implementation
		"whisper/transcribe.py",  // Can use Go HTTP client for API calls
	}

	// Define what must be kept (core ML functionality)
	essentialFiles := []string{
		"interface.py",              // Main OuteTTS interface
		"version/interface.py",      // Core model interfaces
		"version/playback.py",       // Audio playback
		"dac/interface.py",          // DAC audio codec (ML-based)
		"models/config.py",          // Model configuration
		"models/hf_model.py",        // Hugging Face model loading
		"models/gguf_model.py",      // GGUF model loading
		"models/exl2_model.py",      // ExL2 model loading
		"models/vllm_model.py",      // VLLM model loading
		"models/llamacpp_server.py", // Llama.cpp server integration
	}

	// Define removable directories
	removableDirs := []string{
		"anyascii/_data", // ASCII conversion data (can be Go)
	}

	// Define essential directories
	essentialDirs := []string{
		"version/v1",    // Version 1 interface
		"version/v2",    // Version 2 interface
		"version/v3",    // Version 3 interface
		"dac",           // DAC audio codec
		"models",        // Model implementations
		"wav_tokenizer", // Audio tokenization
	}

	// Analyze files
	for _, file := range removableFiles {
		filePath := filepath.Join(outettsDir, file)
		if _, err := os.Stat(filePath); err == nil {
			report.RemovedFiles = append(report.RemovedFiles, file)
		}
	}

	for _, file := range essentialFiles {
		filePath := filepath.Join(outettsDir, file)
		if _, err := os.Stat(filePath); err == nil {
			report.KeptFiles = append(report.KeptFiles, file)
		}
	}

	// Analyze directories
	for _, dir := range removableDirs {
		dirPath := filepath.Join(outettsDir, dir)
		if _, err := os.Stat(dirPath); err == nil {
			report.RemovedDirs = append(report.RemovedDirs, dir)
		}
	}

	for _, dir := range essentialDirs {
		dirPath := filepath.Join(outettsDir, dir)
		if _, err := os.Stat(dirPath); err == nil {
			report.KeptDirs = append(report.KeptDirs, dir)
		}
	}

	// Generate recommendations
	report.Recommendations = []string{
		"Text chunking has been replaced by Go implementation in internal/chunking/",
		"Text preprocessing can be implemented in Go using regex and unicode packages",
		"AnyASCII conversion can be pure Go using unicode normalization",
		"Whisper integration can use Go HTTP client for API calls",
		"Configuration management is now handled by Go structs",
		"Audio file operations can use Go audio libraries",
		"Keep core ML components (DAC, model loading, inference) in Python",
		"Use Go for orchestration, Python for ML inference",
	}

	return report, nil
}

// PrintCleanupReport prints a formatted cleanup report.
func PrintCleanupReport(report *CleanupReport) {
	fmt.Println(reportHeaderAnalysis)
	fmt.Println()

	if len(report.RemovedFiles) > 0 {
		fmt.Println(reportFilesRemoved)

		for _, file := range report.RemovedFiles {
			fmt.Printf(listItemFormat, file)
		}

		fmt.Println()
	}

	if len(report.RemovedDirs) > 0 {
		fmt.Println(reportDirsRemoved)

		for _, dir := range report.RemovedDirs {
			fmt.Printf(listItemFormat, dir)
		}

		fmt.Println()
	}

	if len(report.KeptFiles) > 0 {
		fmt.Println(reportFilesKept)

		for _, file := range report.KeptFiles {
			fmt.Printf(listItemFormat, file)
		}

		fmt.Println()
	}

	if len(report.KeptDirs) > 0 {
		fmt.Println(reportDirsKept)

		for _, dir := range report.KeptDirs {
			fmt.Printf(listItemFormat, dir)
		}

		fmt.Println()
	}

	if len(report.Recommendations) > 0 {
		fmt.Println(reportRecommendations)

		for _, rec := range report.Recommendations {
			fmt.Printf(listItemFormat, rec)
		}

		fmt.Println()
	}

	if len(report.Errors) > 0 {
		fmt.Println(reportErrors)

		for _, err := range report.Errors {
			fmt.Printf(listItemFormat, err)
		}

		fmt.Println()
	}

	fmt.Println(reportHeaderSummary)
	fmt.Printf(summaryFilesToRemove, len(report.RemovedFiles))
	fmt.Printf(summaryDirsToRemove, len(report.RemovedDirs))
	fmt.Printf(summaryFilesToKeep, len(report.KeptFiles))
	fmt.Printf(summaryDirsToKeep, len(report.KeptDirs))
	fmt.Printf(summaryRecommendations, len(report.Recommendations))
}

// GetGoReplacements returns a map of Python files to their Go replacements.
func GetGoReplacements() map[string]string {
	return map[string]string{
		"utils/chunking.py":      "internal/chunking/chunking.go",
		"utils/preprocessing.py": "internal/tts/text/preprocessing.go",
		"utils/helpers.py":       "internal/tts/utils/helpers.go",
		"anyascii/__init__.py":   "internal/tts/text/anyascii.go",
		"whisper/transcribe.py":  "internal/tts/whisper/client.go",
		"models/config.py":       "internal/tts/config/config.go",
	}
}

// ValidateGoImplementation checks if Go replacements exist.
func ValidateGoImplementation() error {
	replacements := GetGoReplacements()

	var missing []string

	for pythonFile, goFile := range replacements {
		if _, err := os.Stat(goFile); os.IsNotExist(err) {
			missing = append(
				missing,
				fmt.Sprintf(replacementFormat, pythonFile, goFile),
			)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf(
			errMissingGoImpl,
			strings.Join(missing, replacementSeparator),
		)
	}

	return nil
}
