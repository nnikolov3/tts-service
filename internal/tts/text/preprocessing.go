// Package text provides text preprocessing utilities for TTS applications.
//
// This package implements text preprocessing functions that were previously
// handled by Python utilities, following Go coding standards and design
// principles for explicit behavior and maintainable code.
package text

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Regex patterns for text preprocessing, following ALL_CAPS convention.
const (
	URL_REGEX_PATTERN        = `https?://[^\s]+`
	EMAIL_REGEX_PATTERN      = `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
	NUMBER_REGEX_PATTERN     = `\d+`
	REFERENCE_REGEX_PATTERN  = `(?:\[\d+\]|\(\d+\)|[¹²³⁴⁵⁶⁷⁸⁹⁰]+|\b\d+\s*\.\s*$)`
	CITATION_REGEX_PATTERN   = `\([^)]*\d{4}[^)]*\)|\b\w+\s+et\s+al\.`
	WHITESPACE_REGEX_PATTERN = `\s+`
)

// Placeholder patterns for preserving URLs and emails.
const (
	URL_PLACEHOLDER_PATTERN   = `__URL_PLACEHOLDER_%d__`
	EMAIL_PLACEHOLDER_PATTERN = `__EMAIL_PLACEHOLDER_%d__`
)

// Punctuation and formatting constants.
const (
	EM_DASH       = "—"
	EN_DASH       = "–"
	FIGURE_DASH   = "‒"
	ELLIPSIS      = "..."
	ELLIPSIS_CHAR = "…"
	CRLF          = "\r\n"
	LF            = "\n"
	TAB           = "\t"
)

// Preprocessor provides text preprocessing functionality for TTS.
type Preprocessor struct {
	// Precompiled regex patterns for performance.
	urlPattern        *regexp.Regexp
	emailPattern      *regexp.Regexp
	numberPattern     *regexp.Regexp
	referencePattern  *regexp.Regexp
	citationPattern   *regexp.Regexp
	whitespacePattern *regexp.Regexp
	// Efficient replacer for common abbreviations.
	abbreviationReplacer *strings.Replacer
}

// NewPreprocessor creates a new text preprocessor with compiled patterns and replacers.
// This adheres to the principle of setting up reusable components upfront.
func NewPreprocessor() *Preprocessor {
	abbreviations := []string{
		"Mr.", "Mister",
		"Mrs.", "Misses",
		"Ms.", "Miss",
		"Dr.", "Doctor",
		"St.", "Saint",
		"Co.", "Company",
		"Ltd.", "Limited",
		"Corp.", "Corporation",
		"Inc.", "Incorporated",
	}

	return &Preprocessor{
		urlPattern:           regexp.MustCompile(URL_REGEX_PATTERN),
		emailPattern:         regexp.MustCompile(EMAIL_REGEX_PATTERN),
		numberPattern:        regexp.MustCompile(NUMBER_REGEX_PATTERN),
		referencePattern:     regexp.MustCompile(REFERENCE_REGEX_PATTERN),
		citationPattern:      regexp.MustCompile(CITATION_REGEX_PATTERN),
		whitespacePattern:    regexp.MustCompile(WHITESPACE_REGEX_PATTERN),
		abbreviationReplacer: strings.NewReplacer(abbreviations...),
	}
}

// PreprocessText performs comprehensive text normalization and cleaning.
// The pipeline makes the common case fast by applying cheaper transformations first.
func (p *Preprocessor) PreprocessText(text string) string {
	if text == "" {
		return text
	}

	// Step 1: Normalize standard forms (abbreviations, numbers).
	normalizedText := p.expandAbbreviations(text)
	normalizedText = p.normalizeNumbers(normalizedText)

	// Step 2: Preserve tokens that should not be cleaned (URLs, emails).
	// This step is now atomic and correctly handles multiple occurrences.
	preservedText, placeholders := p.preserveTokens(normalizedText)

	// Step 3: Remove unwanted content like citations and references.
	cleanedText := p.removeReferences(preservedText)
	cleanedText = p.removeCitations(cleanedText)

	// Step 4: Normalize whitespace and formatting.
	cleanedText = p.normalizeWhitespace(cleanedText)

	// Step 5: Restore preserved tokens.
	restoredText := p.restoreTokens(cleanedText, placeholders)

	// Step 6: Perform final punctuation and sentence structure cleanup.
	finalText := p.finalCleanup(restoredText)

	return finalText
}

// ConvertToPhonemes transforms cleaned text into a phonetic representation.
// This is a critical step for the acoustic model in a TTS pipeline.
// This implementation uses a simple dictionary lookup as a robust, explicit starting point.
func (p *Preprocessor) ConvertToPhonemes(text string) string {
	// A simple dictionary for Grapheme-to-Phoneme (G2P) conversion.
	phonemeDict := map[string]string{
		"hello":    "h ɛ l oʊ",
		"world":    "w ɜ r l d",
		"go":       "g oʊ",
		"is":       "ɪ z",
		"a":        "ə",
		"powerful": "p aʊ ər f ə l",
		"language": "l æ ŋ g w ə dʒ",
		"doctor":   "d ɑ k t ər",
		"saint":    "s eɪ n t",
		"one":      "w ʌ n",
		"two":      "t u",
		"three":    "θ r i",
		"hundred":  "h ʌ n d r ə d",
		"thousand": "θ aʊ z ə n d",
		"text":     "t ɛ k s t",
		"to":       "t u",
		"speech":   "s p i tʃ",
		"system":   "s ɪ s t ə m",
	}

	var result strings.Builder
	words := strings.Fields(strings.ToLower(text))

	for i, word := range words {
		// Clean the word of any surrounding punctuation for dictionary lookup.
		cleanWord := strings.Trim(word, `.,!?;:"'`)

		if phonemes, found := phonemeDict[cleanWord]; found {
			result.WriteString(phonemes)
		} else {
			// Fallback for out-of-dictionary words is the word itself.
			// A more advanced system might attempt to guess the phonemes.
			result.WriteString(cleanWord)
		}

		if i < len(words)-1 {
			result.WriteString(" ")
		}
	}

	return result.String()
}

// expandAbbreviations converts common abbreviations to their full form.
func (p *Preprocessor) expandAbbreviations(text string) string {
	return p.abbreviationReplacer.Replace(text)
}

// normalizeNumbers finds all integers in the text and converts them to words.
func (p *Preprocessor) normalizeNumbers(text string) string {
	return p.numberPattern.ReplaceAllStringFunc(text, func(s string) string {
		num, err := strconv.Atoi(s)
		if err != nil {
			return s // Return original string if not a valid integer
		}
		return integerToWords(num)
	})
}

// preserveTokens temporarily replaces URLs and emails with placeholders.
// This function is crucial for preventing the cleaning process from corrupting them.
// It now correctly handles multiple identical tokens.
func (p *Preprocessor) preserveTokens(text string) (string, map[string]string) {
	placeholders := make(map[string]string)
	i := 0

	replaceFunc := func(pattern *regexp.Regexp, placeholderFormat string) {
		text = pattern.ReplaceAllStringFunc(text, func(match string) string {
			placeholder := fmt.Sprintf(placeholderFormat, i)
			placeholders[placeholder] = match
			i++
			return placeholder
		})
	}

	replaceFunc(p.urlPattern, URL_PLACEHOLDER_PATTERN)
	replaceFunc(p.emailPattern, EMAIL_PLACEHOLDER_PATTERN)

	return text, placeholders
}

// restoreTokens restores URLs and emails from placeholders.
func (p *Preprocessor) restoreTokens(text string, placeholders map[string]string) string {
	for placeholder, original := range placeholders {
		text = strings.ReplaceAll(text, placeholder, original)
	}
	return text
}

// removeReferences removes citation markers and references.
func (p *Preprocessor) removeReferences(text string) string {
	return p.referencePattern.ReplaceAllString(text, "")
}

// removeCitations removes academic citations.
func (p *Preprocessor) removeCitations(text string) string {
	return p.citationPattern.ReplaceAllString(text, "")
}

// normalizeWhitespace normalizes various whitespace characters.
func (p *Preprocessor) normalizeWhitespace(text string) string {
	// Replace various whitespace with a standard space.
	text = p.whitespacePattern.ReplaceAllString(text, " ")

	// Clean up common formatting issues.
	replacer := strings.NewReplacer(CRLF, " ", LF, " ", TAB, " ")
	text = replacer.Replace(text)

	return strings.TrimSpace(text)
}

// finalCleanup performs final text cleanup.
func (p *Preprocessor) finalCleanup(text string) string {
	// Remove excessive punctuation.
	text = p.removeExcessivePunctuation(text)

	// Normalize quotes and dashes.
	text = p.normalizeQuotesAndDashes(text)

	// Ensure proper sentence endings.
	text = p.ensureProperSentenceEndings(text)

	return text
}

// removeExcessivePunctuation removes repeated punctuation marks.
func (p *Preprocessor) removeExcessivePunctuation(text string) string {
	var (
		result       []rune
		lastWasPunct bool
	)

	for _, char := range text {
		isPunct := unicode.IsPunct(char)
		if isPunct && !lastWasPunct {
			result = append(result, char)
		} else if !isPunct {
			result = append(result, char)
		}
		lastWasPunct = isPunct
	}

	return string(result)
}

// normalizeQuotesAndDashes normalizes various quote and dash characters.
func (p *Preprocessor) normalizeQuotesAndDashes(text string) string {
	// Using a single, efficient replacer is preferred.
	replacer := strings.NewReplacer(
		EM_DASH, "-",
		EN_DASH, "-",
		FIGURE_DASH, "-",
		ELLIPSIS_CHAR, ELLIPSIS,
		"“", `"`, "”", `"`, // Smart quotes to standard quotes
		"‘", "'", "’", "'", // Smart single quotes to standard
	)
	return replacer.Replace(text)
}

// ensureProperSentenceEndings ensures sentences end with proper punctuation.
func (p *Preprocessor) ensureProperSentenceEndings(text string) string {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return ""
	}

	// Check if text ends with sentence-ending punctuation.
	lastChar, _ := utf8.DecodeLastRuneInString(trimmedText)
	if !unicode.IsPunct(lastChar) {
		return trimmedText + "."
	}

	switch lastChar {
	case '.', '!', '?':
		return trimmedText
	default:
		return trimmedText + "."
	}
}

// integerToWords converts an integer into its English word representation.
// This is a simplified implementation for demonstration.
func integerToWords(n int) string {
	if n < 0 || n > 999999 {
		return strconv.Itoa(n) // Fallback for out-of-range numbers.
	}
	if n == 0 {
		return "zero"
	}

	// Helper arrays for number conversion.
	ones := []string{"", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine"}
	teens := []string{
		"ten",
		"eleven",
		"twelve",
		"thirteen",
		"fourteen",
		"fifteen",
		"sixteen",
		"seventeen",
		"eighteen",
		"nineteen",
	}
	tens := []string{
		"",
		"",
		"twenty",
		"thirty",
		"forty",
		"fifty",
		"sixty",
		"seventy",
		"eighty",
		"ninety",
	}

	var parts []string
	processPart := func(num int, suffix string) int {
		if num > 0 {
			parts = append(parts, toWords(num)+" "+suffix)
		}
		return 0 // Dummy return to fit in a structure if needed later
	}

	processPart(n/1000, "thousand")
	n %= 1000
	processPart(n/100, "hundred")
	n %= 100

	if n > 0 {
		var tempPart string
		if n < 10 {
			tempPart = ones[n]
		} else if n < 20 {
			tempPart = teens[n-10]
		} else {
			tempPart = tens[n/10]
			if n%10 > 0 {
				tempPart += " " + ones[n%10]
			}
		}
		parts = append(parts, tempPart)
	}

	return strings.Join(parts, " ")
}
