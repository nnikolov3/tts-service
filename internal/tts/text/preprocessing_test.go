package text_test

import (
	"testing"

	"tts/internal/tts/text"
)

// preprocessorTestCase defines a standard test case for the preprocessor.
type preprocessorTestCase struct {
	name     string
	input    string
	expected string
}

// runPreprocessorTests is a helper function to run table-driven tests for a given
// processing function.
func runPreprocessorTests(
	t *testing.T,
	tests []preprocessorTestCase,
	processFunc func(p *text.Preprocessor, text string) string,
) {
	t.Helper()

	preprocessor := text.NewPreprocessor()

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := processFunc(preprocessor, testCase.input)
			if result != testCase.expected {
				t.Errorf("Expected %q, got %q", testCase.expected, result)
			}
		})
	}
}

func TestNewPreprocessor(t *testing.T) {
	t.Parallel()

	preprocessor := text.NewPreprocessor()
	if preprocessor == nil {
		t.Fatal("NewPreprocessor returned nil")
	}
}

func TestPreprocessor_PreprocessText_EmptyInput(t *testing.T) {
	t.Parallel()

	preprocessor := text.NewPreprocessor()

	result := preprocessor.PreprocessText("")
	if result != "" {
		t.Errorf("Expected empty string for empty input, got %q", result)
	}
}

func TestPreprocessor_PreprocessText_BasicText(t *testing.T) {
	t.Parallel()

	preprocessor := text.NewPreprocessor()
	input := "Hello world"
	result := preprocessor.PreprocessText(input)

	if result == "" {
		t.Error("Expected non-empty result for basic text")
	}

	if result != "Hello world." {
		t.Errorf("Expected 'Hello world.', got %q", result)
	}
}

func TestPreprocessor_PreprocessText_AbbreviationExpansion(t *testing.T) {
	t.Parallel()

	tests := []preprocessorTestCase{
		{
			name:     "Mr expansion",
			input:    "Mr. Smith",
			expected: "Mister Smith.",
		},
		{
			name:     "Dr expansion",
			input:    "Dr. Johnson",
			expected: "Doctor Johnson.",
		},
		{
			name:     "Multiple abbreviations",
			input:    "Mr. and Mrs. Smith",
			expected: "Mister and Misses Smith.",
		},
		{
			name:     "Inc. expansion",
			input:    "Future Tech Inc.",
			expected: "Future Tech Incorporated.",
		},
	}
	runPreprocessorTests(t, tests, func(p *text.Preprocessor, txt string) string {
		return p.PreprocessText(txt)
	})
}

// TestNumberNormalization verifies that numbers are correctly converted to words.
func TestPreprocessor_PreprocessText_NumberNormalization(t *testing.T) {
	t.Parallel()

	tests := []preprocessorTestCase{
		{
			name:     "Single digit number",
			input:    "There are 3 cars.",
			expected: "There are three cars.",
		},
		{
			name:     "Teen number",
			input:    "I have 17 friends.",
			expected: "I have seventeen friends.",
		},
		{
			name:     "Two-digit number",
			input:    "The answer is 42.",
			expected: "The answer is forty two.",
		},
		{
			name:     "Hundred number",
			input:    "He has 100 dollars.",
			expected: "He has one hundred dollars.",
		},
		{
			name:     "Complex hundred number",
			input:    "The building is 356 feet tall.",
			expected: "The building is three hundred fifty six feet tall.",
		},
		{
			name:     "Thousand number",
			input:    "About 5000 people attended.",
			expected: "About five thousand people attended.",
		},
		{
			name:     "Maximum number",
			input:    "The max value is 999999.",
			expected: "The max value is nine hundred ninety nine thousand nine hundred ninety nine.",
		},
		{
			name:     "Number over the limit",
			input:    "A million is 1000000.",
			expected: "A million is 1000000.",
		},
	}
	runPreprocessorTests(t, tests, func(p *text.Preprocessor, txt string) string {
		return p.PreprocessText(txt)
	})
}

// TestTokenPreservation ensures that URLs and emails are not modified during
// preprocessing.
func TestPreprocessor_PreprocessText_TokenPreservation(t *testing.T) {
	t.Parallel()

	tests := []preprocessorTestCase{
		{
			name:     "URL only",
			input:    "Please visit https://example.com for more info.",
			expected: "Please visit https://example.com for more info.",
		},
		{
			name:     "Email only",
			input:    "Contact us at support@example.org.",
			expected: "Contact us at support@example.org.",
		},
		{
			name:     "URL and Email mixed with other processing",
			input:    "Mr. Doe's site is http://johndoe.com, email him at john.doe@email.com for 1 copy.",
			expected: "Mister Doe's site is http://johndoe.com, email him at john.doe@email.com for one copy.",
		},
		{
			name:     "Multiple URLs and emails",
			input:    "See https://a.com and email b@c.com. Also check http://d.com.",
			expected: "See https://a.com and email b@c.com. Also check http://d.com.",
		},
	}
	runPreprocessorTests(t, tests, func(p *text.Preprocessor, txt string) string {
		return p.PreprocessText(txt)
	})
}

// TestContentRemoval verifies removal of citations and references.
func TestPreprocessor_PreprocessText_ContentRemoval(t *testing.T) {
	t.Parallel()

	tests := []preprocessorTestCase{
		{
			name:     "Bracketed reference",
			input:    "This is a statement [1].",
			expected: "This is a statement.",
		},
		{
			name:     "Parenthetical reference",
			input:    "This is another statement (2).",
			expected: "This is another statement.",
		},
		{
			name:     "Superscript reference",
			input:    "A third statement¹.",
			expected: "A third statement.",
		},
		{
			name:     "Et al citation",
			input:    "As shown by Smith et al. this is true.",
			expected: "As shown by this is true.",
		},
		{
			name:     "Year in parentheses citation",
			input:    "The study (Johnson, 2021) shows results.",
			expected: "The study shows results.",
		},
		{
			name:     "Multiple removals",
			input:    "First point [1], then another (see Smith et al.).",
			expected: "First point, then another.",
		},
	}
	runPreprocessorTests(t, tests, func(p *text.Preprocessor, txt string) string {
		return p.PreprocessText(txt)
	})
}

// TestWhitespaceAndFormatting checks normalization of whitespace, quotes, and dashes.
func TestPreprocessor_PreprocessText_WhitespaceAndFormatting(t *testing.T) {
	t.Parallel()

	tests := []preprocessorTestCase{
		{
			name:     "Multiple spaces",
			input:    "Hello   world",
			expected: "Hello world.",
		},
		{
			name:     "Tabs and newlines",
			input:    "Line 1\nand\tline 2.",
			expected: "Line 1 and line 2.",
		},
		{
			name:     "Smart quotes",
			input:    "He said, “Hello.”",
			expected: `He said, "Hello."`,
		},
		{
			name:     "Various dashes",
			input:    "This is a range (1–5) — it's important.",
			expected: "This is a range (1-5) - it's important.",
		},
		{
			name:     "Excessive punctuation",
			input:    "Hello!!! How are you??",
			expected: "Hello! How are you?",
		},
		{
			name:     "No final punctuation",
			input:    "This sentence has no end",
			expected: "This sentence has no end.",
		},
		{
			name:     "Already has final punctuation",
			input:    "Are you sure?",
			expected: "Are you sure?",
		},
	}
	runPreprocessorTests(t, tests, func(p *text.Preprocessor, txt string) string {
		return p.PreprocessText(txt)
	})
}

// TestComprehensive applies multiple preprocessing rules in a single test.
func TestPreprocessor_PreprocessText_Comprehensive(t *testing.T) {
	t.Parallel()

	preprocessor := text.NewPreprocessor()
	input := "  Dr. Smith's latest paper [1] (see Smith et al., 2023) is available at " +
		"http://example.com. It discusses 10 key findings. Contact him at dr.smith@example.org!!  "
	expected := "Doctor Smith's latest paper is available at http://example.com. " +
		"It discusses ten key findings. Contact him at dr.smith@example.org!"

	result := preprocessor.PreprocessText(input)
	if result != expected {
		t.Errorf(
			"Comprehensive test failed.\nExpected: %q\nGot:      %q",
			expected,
			result,
		)
	}
}

func TestPreprocessor_ConvertToPhonemes(t *testing.T) {
	t.Parallel()

	tests := []preprocessorTestCase{
		{name: "single known word", input: "hello", expected: "h ɛ l oʊ"},
		{
			name:     "multiple known words",
			input:    "hello world",
			expected: "h ɛ l oʊ w ɜ r l d",
		},
		{
			name:     "mixed known and unknown",
			input:    "hello unknown",
			expected: "h ɛ l oʊ unknown",
		},
		{name: "empty input", input: "", expected: ""},
		{
			name:     "numbers converted to words",
			input:    "one two three",
			expected: "w ʌ n t u θ r i",
		},
		{
			name:     "case insensitive",
			input:    "HELLO World",
			expected: "h ɛ l oʊ w ɜ r l d",
		},
		{name: "word with punctuation", input: "world!", expected: "w ɜ r l d"},
	}

	runPreprocessorTests(t, tests, func(p *text.Preprocessor, txt string) string {
		return p.ConvertToPhonemes(txt)
	})
}
