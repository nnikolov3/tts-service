package main

import (
	"flag"
	"os"
	"strings"
	"testing"
)

// Test constants, updated to follow CamelCase convention.
const (
	TestExpectedTextFlag      = "Expected text flag %q, got %q"
	TestErrEitherTextOrChunks = "Either --text or --chunks must be provided"
	TestErrCannotSpecifyBoth  = "Cannot specify both --text and --chunks"
)

// TestMainFlags verifies that command-line flags are parsed correctly.
// This test uses a table-driven structure for clarity and extensibility.
func TestMainFlags(t *testing.T) {
	t.Parallel()
	// Save original command line args to restore them after the test.
	oldArgs := os.Args

	t.Cleanup(func() { os.Args = oldArgs })

	tests := []struct {
		name     string
		wantText string
		args     []string
	}{
		{
			name:     "text flag parsing",
			args:     []string{"cmd", "--text", "Hello, world!"},
			wantText: "Hello, world!",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			// Reset flag parsing state for each test run to ensure isolation.
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			// Set test-specific arguments.
			os.Args = testCase.args

			// Define and parse the flags, simulating the main function's
			// setup.
			textFlag := flag.String(flagText, "", flagTextDesc)
			flag.Parse()

			// Assert that the parsed flag matches the expected value.
			if *textFlag != testCase.wantText {
				t.Errorf(
					TestExpectedTextFlag,
					testCase.wantText,
					*textFlag,
				)
			}
		})
	}
}

// TestArgumentValidation verifies the business logic for required and conflicting
// arguments.
// This replaces the previous non-functional placeholder test. It validates inputs at the
// application's boundary, adhering to the principle of explicit error checking.
func TestArgumentValidation(t *testing.T) {
	t.Parallel()

	oldArgs := os.Args

	t.Cleanup(func() { os.Args = oldArgs })

	tests := getValidationTestCases()

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			flags := setupTestFlags(t, testCase)
			err := validateArgumentsOnly(flags)
			validateTestResult(t, testCase, err)
		})
	}
}

// getValidationTestCases returns test cases for argument validation.
func getValidationTestCases() []struct {
	name          string
	expectedError string
	args          []string
	wantErr       bool
} {
	return []struct {
		name          string
		expectedError string
		args          []string
		wantErr       bool
	}{
		{
			name:          "success with text flag",
			args:          []string{"cmd", "--text", "some text"},
			wantErr:       false,
			expectedError: "",
		},
		{
			name:          "success with chunks flag",
			args:          []string{"cmd", "--chunks", "file.json"},
			wantErr:       false,
			expectedError: "",
		},
		{
			name: "error with both flags",
			args: []string{
				"cmd",
				"--text",
				"some text",
				"--chunks",
				"file.json",
			},
			wantErr:       true,
			expectedError: TestErrCannotSpecifyBoth,
		},
		{
			name:          "error with no flags",
			args:          []string{"cmd"},
			wantErr:       true,
			expectedError: TestErrEitherTextOrChunks,
		},
	}
}

// setupTestFlags prepares flags for a test case.
func setupTestFlags(_ *testing.T, testCase struct {
	name          string
	expectedError string
	args          []string
	wantErr       bool
},
) appFlags {
	flag.CommandLine = flag.NewFlagSet(testCase.name, flag.ContinueOnError)
	os.Args = testCase.args

	var flags appFlags
	flag.StringVar(&flags.text, flagText, "", flagTextDesc)
	flag.StringVar(&flags.chunks, flagChunks, "", flagChunksDesc)
	flag.Parse()

	return flags
}

// validateTestResult checks if the test result matches expectations.
func validateTestResult(t *testing.T, testCase struct {
	name          string
	expectedError string
	args          []string
	wantErr       bool
}, err error,
) {
	t.Helper()

	if testCase.wantErr {
		validateExpectedError(t, testCase.expectedError, err)

		return
	}

	validateNoError(t, err)
}

// validateExpectedError checks that an expected error occurred.
func validateExpectedError(t *testing.T, expectedError string, err error) {
	t.Helper()

	if err == nil {
		t.Errorf("Expected an error but got none")

		return
	}

	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf(
			"Expected error to contain %q, but got %q",
			expectedError,
			err.Error(),
		)
	}
}

// validateNoError checks that no error occurred when none was expected.
func validateNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("Did not expect an error, but got: %v", err)
	}
}
