package ttsutils_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"tts/internal/tts/ttsutils"
)

// setupModelFile creates a temporary directory structure and a dummy model file for
// testing.
func setupModelFile(t *testing.T, dir, modelName string) {
	t.Helper()

	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create test directory %q: %v", dir, err)
	}

	_, err = os.Create(filepath.Join(dir, modelName))
	if err != nil {
		t.Fatalf("Failed to create test model file in %q: %v", dir, err)
	}
}

func TestGetCacheDir_WithOverride(t *testing.T) {
	t.Parallel()

	expectedPath := "/custom/cache/dir"
	t.Setenv("CACHE_DIR", expectedPath)

	result := ttsutils.GetCacheDir()
	if result != expectedPath {
		t.Errorf(
			"Expected cache dir %q, but got %q",
			expectedPath,
			result,
		)
	}
}

func TestGetCacheDir_OSDefault(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// This test can't run without a home directory, so we skip it.
		t.Skip("Skipping test: could not determine user home directory")
	}

	expected := filepath.Join(homeDir, ".cache", "tts-service")
	result := ttsutils.GetCacheDir()

	if result != expected {
		t.Errorf(
			"Expected default cache dir %q for OS %s, but got %q",
			expected,
			runtime.GOOS,
			result,
		)
	}
}

// TestEnsureDir verifies that a directory is created if it doesn't exist.
func TestEnsureDir(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "new", "dir")

	err := ttsutils.EnsureDir(testPath)
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	_, err = os.Stat(testPath)
	if os.IsNotExist(err) {
		t.Errorf("Directory %q was not created", testPath)
	}

	err = ttsutils.EnsureDir(testPath)
	if err != nil {
		t.Errorf("EnsureDir failed on existing directory: %v", err)
	}
}

func TestGetModelPath_InCurrentDir(t *testing.T) {
	t.Parallel()

	modelName := "test_model.bin"
	subdir := t.TempDir()
	// t.Chdir automatically changes back to the original directory after the test.
	t.Chdir(subdir)

	setupModelFile(t, subdir, modelName)

	path, err := ttsutils.GetModelPath(modelName)
	if err != nil {
		t.Fatalf("Expected to find model, but got error: %v", err)
	}

	expected, err := filepath.Abs(modelName)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if path != expected {
		t.Errorf("Expected path %q, got %q", expected, path)
	}
}

func TestGetModelPath_InCacheDir(t *testing.T) {
	t.Parallel()

	modelName := "test_model.bin"
	cacheDir := t.TempDir()
	t.Setenv("CACHE_DIR", cacheDir)
	setupModelFile(t, filepath.Join(cacheDir, "models"), modelName)

	_, err := ttsutils.GetModelPath(modelName)
	if err != nil {
		t.Errorf(
			"Expected to find model in cache, but got error: %v",
			err,
		)
	}
}

func TestGetModelPath_NotFound(t *testing.T) {
	t.Parallel()

	_, err := ttsutils.GetModelPath("non_existent_model.bin")
	if !errors.Is(err, ttsutils.ErrModelNotFound) {
		t.Errorf("Expected ErrModelNotFound, but got %v", err)
	}
}

// TestFormatDuration verifies duration formatting logic.
func TestFormatDuration(t *testing.T) {
	t.Parallel()

	const (
		halfMinuteInSeconds    = 30.5
		exactMinuteInSeconds   = 60
		minuteAndHalfInSeconds = 90.5
		exactHourInSeconds     = 3600
		hourAndMinuteInSeconds = 3670
	)

	testCases := []struct {
		name     string
		expected string
		seconds  float64
	}{
		{
			name:     "less than a minute",
			seconds:  halfMinuteInSeconds,
			expected: "30.5s",
		},
		{
			name:     "exactly a minute",
			seconds:  exactMinuteInSeconds,
			expected: "1m 0.0s",
		},
		{
			name:     "less than an hour",
			seconds:  minuteAndHalfInSeconds,
			expected: "1m 30.5s",
		},
		{name: "exactly an hour", seconds: exactHourInSeconds, expected: "1h 0m"},
		{
			name:     "more than an hour",
			seconds:  hourAndMinuteInSeconds,
			expected: "1h 1m",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := ttsutils.FormatDuration(testCase.seconds)
			if result != testCase.expected {
				t.Errorf("Expected %q, got %q", testCase.expected, result)
			}
		})
	}
}

// TestFormatFileSize verifies file size formatting logic.
func TestFormatFileSize(t *testing.T) {
	t.Parallel()

	const (
		bytesTestValue               int64 = 500
		kibibytesTestValue           int64 = 2048
		oneAndHalfMebibytesTestValue int64 = 1572864
		twoGibibytesTestValue        int64 = 2147483648
	)

	testCases := []struct {
		name     string
		expected string
		bytes    int64
	}{
		{name: "bytes", bytes: bytesTestValue, expected: "500 B"},
		{name: "kilobytes", bytes: kibibytesTestValue, expected: "2.0 KB"},
		{
			name:     "megabytes",
			bytes:    oneAndHalfMebibytesTestValue,
			expected: "1.5 MB",
		},
		{name: "gigabytes", bytes: twoGibibytesTestValue, expected: "2.0 GB"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := ttsutils.FormatFileSize(testCase.bytes)
			if result != testCase.expected {
				t.Errorf("Expected %q, got %q", testCase.expected, result)
			}
		})
	}
}

// TestIsValidAudioFile verifies audio file extension checks.
func TestIsValidAudioFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		filename string
		isValid  bool
	}{
		{"test.wav", true},
		{"test.mp3", true},
		{"test.flac", true},
		{"test.ogg", true},
		{"test.m4a", true},
		{"test.aac", true},
		{"test.txt", false},
		{"image.jpg", false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.filename, func(t *testing.T) {
			t.Parallel()

			if result := ttsutils.IsValidAudioFile(testCase.filename); result != testCase.isValid {
				t.Errorf(
					"IsValidAudioFile(%q) = %v; want %v",
					testCase.filename,
					result,
					testCase.isValid,
				)
			}
		})
	}
}

// TestIsValidTextFile verifies text file extension checks.
func TestIsValidTextFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		filename string
		isValid  bool
	}{
		{"test.txt", true},
		{"test.md", true},
		{"test.json", true},
		{"test.xml", true},
		{"test.html", true},
		{"test.htm", true},
		{"test.wav", false},
		{"archive.zip", false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.filename, func(t *testing.T) {
			t.Parallel()

			if result := ttsutils.IsValidTextFile(testCase.filename); result != testCase.isValid {
				t.Errorf(
					"IsValidTextFile(%q) = %v; want %v",
					testCase.filename,
					result,
					testCase.isValid,
				)
			}
		})
	}
}

// TestGetFileExtension verifies it returns the extension without the dot.
func TestGetFileExtension(t *testing.T) {
	t.Parallel()

	result := ttsutils.GetFileExtension("archive.tar.gz")
	if result != "gz" {
		t.Errorf("Expected 'gz', got %q", result)
	}
}

// TestSanitizeFilename verifies that invalid characters are removed.
func TestSanitizeFilename(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"no changes", "valid_filename.txt", "valid_filename.txt"},
		{
			"replaces invalid chars",
			"in<va>l:id\"/\\|?*name.txt",
			"in_va_l_id_______name.txt",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := ttsutils.SanitizeFilename(testCase.input)
			if result != testCase.expected {
				t.Errorf(
					"Expected sanitized filename %q, got %q",
					testCase.expected,
					result,
				)
			}
		})
	}
}
