// Package ttsutils provides file and path utility functions for applications.
//
// This package focuses on platform-agnostic ways to handle application paths,
// format data for display, and sanitize filenames, adhering to Go's best practices
// for clarity, error handling, and maintainability.
package ttsutils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Environment variable names used for path resolution.
const (
	envCacheDir = "CACHE_DIR"
)

// Common application directory and path constants.
const (
	appName                = "tts-service"
	cacheDirName           = "cache"
	modelsDirName          = "models"
	tmpDir                 = "/tmp"
	dotCache               = ".cache"
	defaultDirPermissions  = 0o750
	dot                    = "."
	invalidCharReplacement = "_"
)

// Data size constants.
const (
	byteUnit = 1
	kilobyte = byteUnit * 1024
	megabyte = kilobyte * 1024
	gigabyte = megabyte * 1024
)

// Time and size formatting constants.
const (
	secondsInMinute = 60
	secondsInHour   = 3600
	formatSeconds   = "%.1fs"
	formatMinutes   = "%dm %.1fs"
	formatHours     = "%dh %dm"
	formatGB        = "%.1f GB"
	formatMB        = "%.1f MB"
	formatKB        = "%.1f KB"
	formatBytes     = "%d B"
)

// File extension constants.
const (
	extAAC  = ".aac"
	extFLAC = ".flac"
	extHTM  = ".htm"
	extHTML = ".html"
	extJSON = ".json"
	extM4A  = ".m4a"
	extMD   = ".md"
	extMP3  = ".mp3"
	extOGG  = ".ogg"
	extTXT  = ".txt"
	extWAV  = ".wav"
	extXML  = ".xml"
)

// Error message and format string constants.
const (
	errModelNotFoundMsg               = "model not found"
	errFmtFailedToCreateDir           = "failed to create directory %s: %w"
	errFmtCouldNotResolveAbsolutePath = "could not resolve absolute path for %q: %w"
	errFmtErrorCheckingModelPath      = "error checking model path %q: %w"
	errFmtModelNotFound               = "%w: %s"
)

// ErrModelNotFound is returned when a model file cannot be located.
var ErrModelNotFound = errors.New(errModelNotFoundMsg)

// GetCacheDir returns the application's cache directory, respecting an environment
// variable override and falling back to a standard user-based cache directory.
func GetCacheDir() string {
	// Honor the user-defined CACHE_DIR if it's set.
	if cacheDir := os.Getenv(envCacheDir); cacheDir != "" {
		return cacheDir
	}

	// Default to a .cache directory in the user's home.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to a temporary directory if home cannot be determined.
		return filepath.Join(tmpDir, appName, cacheDirName)
	}

	return filepath.Join(homeDir, dotCache, appName)
}

// EnsureDir ensures a directory exists at the given path, creating it if it doesn't.
func EnsureDir(path string) error {
	_, statErr := os.Stat(path)
	if os.IsNotExist(statErr) {
		// MkdirAll is used to create parent directories as needed.
		mkdirErr := os.MkdirAll(path, defaultDirPermissions)
		if mkdirErr != nil {
			return fmt.Errorf(
				errFmtFailedToCreateDir,
				path,
				mkdirErr,
			)
		}
	}

	return nil
}

// resolveSinglePath checks if a file exists at a given path.
// If it exists, it returns the absolute path and found=true.
// If it doesn't exist, it returns found=false and no error.
// If a file system error other than "not found" occurs, it returns an error.
func resolveSinglePath(path string) (resolvedPath string, found bool, err error) {
	_, statErr := os.Stat(path)
	if statErr == nil {
		// Path is valid. Get its absolute representation.
		absPath, errAbs := filepath.Abs(path)
		if errAbs != nil {
			// This is a fatal error (e.g., CWD is invalid). Return the error
			// to stop the search.
			return "", false, fmt.Errorf(
				errFmtCouldNotResolveAbsolutePath,
				path,
				errAbs,
			)
		}
		// Success: return the absolute path and signal that it was found.
		return absPath, true, nil
	} else if !os.IsNotExist(statErr) {
		// A different error occurred (e.g., permissions). This is also fatal.
		return "", false, fmt.Errorf(errFmtErrorCheckingModelPath, path, statErr)
	}

	// The path does not exist; signal to continue searching without error.
	return "", false, nil
}

// GetModelPath resolves the absolute path to a model file by checking standard locations.
// It searches a prioritized list of paths by calling a helper for each candidate.
func GetModelPath(modelName string) (string, error) {
	// Build a list of candidate paths in the desired search order.
	candidatePaths := []string{
		modelName, // Handles absolute paths and relative paths from the current directory.
		filepath.Join(
			modelsDirName,
			modelName,
		), // Check in a local "models" directory.
		filepath.Join(
			GetCacheDir(),
			modelsDirName,
			modelName,
		), // Check in the cache.
	}

	// Iterate through the candidates, using the helper to check each one.
	for _, path := range candidatePaths {
		resolvedPath, found, err := resolveSinglePath(path)
		if err != nil {
			// A fatal error occurred in the helper, so we stop and return it.
			return "", err
		} else if found {
			// The helper found the model, so we can return the path
			// immediately.
			return resolvedPath, nil
		}
	}

	// If the loop completes, the model was not found in any location.
	return "", fmt.Errorf(errFmtModelNotFound, ErrModelNotFound, modelName)
}

// FormatDuration formats a duration in a human-readable string (e.g., "1h 15m", "5m
// 30.5s", "45.2s").
func FormatDuration(seconds float64) string {
	if seconds < secondsInMinute {
		return fmt.Sprintf(formatSeconds, seconds)
	}

	if seconds < secondsInHour {
		minutes := int(seconds / secondsInMinute)
		remainingSeconds := seconds - float64(minutes*secondsInMinute)

		return fmt.Sprintf(formatMinutes, minutes, remainingSeconds)
	}

	hours := int(seconds / secondsInHour)
	remainingSeconds := seconds - float64(hours*secondsInHour)
	remainingMinutes := int(remainingSeconds / secondsInMinute)

	return fmt.Sprintf(formatHours, hours, remainingMinutes)
}

// FormatFileSize formats a file size in a human-readable string (e.g., "1.2 GB", "500.5
// MB").
func FormatFileSize(bytes int64) string {
	switch {
	case bytes >= gigabyte:
		return fmt.Sprintf(formatGB, float64(bytes)/gigabyte)
	case bytes >= megabyte:
		return fmt.Sprintf(formatMB, float64(bytes)/megabyte)
	case bytes >= kilobyte:
		return fmt.Sprintf(formatKB, float64(bytes)/kilobyte)
	default:
		return fmt.Sprintf(formatBytes, bytes)
	}
}

// IsValidAudioFile checks if a filename has a common audio file extension.
func IsValidAudioFile(filename string) bool {
	ext := filepath.Ext(filename)
	switch ext {
	case extWAV, extMP3, extFLAC, extOGG, extM4A, extAAC:
		return true
	default:
		return false
	}
}

// IsValidTextFile checks if a filename has a common text or markup file extension.
func IsValidTextFile(filename string) bool {
	ext := filepath.Ext(filename)
	switch ext {
	case extTXT, extMD, extJSON, extXML, extHTML, extHTM:
		return true
	default:
		return false
	}
}

// GetFileExtension returns the file extension without the leading dot.
func GetFileExtension(filename string) string {
	return strings.TrimPrefix(filepath.Ext(filename), dot)
}

// SanitizeFilename removes or replaces characters that are invalid in most filesystems.
func SanitizeFilename(filename string) string {
	// Create a replacer for improved performance and readability over a manual loop.
	replacer := strings.NewReplacer(
		"<", invalidCharReplacement,
		">", invalidCharReplacement,
		":", invalidCharReplacement,
		"\"", invalidCharReplacement,
		"/", invalidCharReplacement,
		"\\", invalidCharReplacement,
		"|", invalidCharReplacement,
		"?", invalidCharReplacement,
		"*", invalidCharReplacement,
	)

	return replacer.Replace(filename)
}
