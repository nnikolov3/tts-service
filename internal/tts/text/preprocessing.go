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

const (
	// NumberBaseTen represents the base for decimal number system.
	NumberBaseTen = 10
	// NumberBaseTwenty represents the boundary for teen numbers.
	NumberBaseTwenty = 20
	// NumberBaseHundred represents the base for hundreds.
	NumberBaseHundred = 100
	// NumberBaseThousand represents the base for thousands.
	NumberBaseThousand = 1000
	// MaxNumberForWords represents the maximum number that can be converted to words.
	MaxNumberForWords = 999999
)

// Regex patterns for text preprocessing.
const (
	urlRegexPattern        = `https?://\S+`
	emailRegexPattern      = `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`
	numberRegexPattern     = `\d+`
	referenceRegexPattern  = `(?:\[\d+\]|\(\d+\)|[¹²³⁴⁵⁶⁷⁸⁹⁰]+|\b\d+\s*\.\s*$)`
	citationRegexPattern   = `\([^)]*\d{4}[^)]*\)|\b\w+\s+et\s+al\.`
	whitespaceRegexPattern = `\s+`
)

// Patterns for preserving URLs and emails.
const (
	urlPlaceholderPattern   = `__URL_PLACEHOLDER_%d__`
	emailPlaceholderPattern = `__EMAIL_PLACEHOLDER_%d__`
)

// Punctuation and formatting constants.
const (
	emDash         = "—"
	enDash         = "–"
	figureDash     = "‒"
	ellipsis       = "..."
	ellipsisChar   = "…"
	carriageReturn = "\r\n"
	lineFeed       = "\n"
	tabChar        = "\t"
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
		urlPattern:           regexp.MustCompile(urlRegexPattern),
		emailPattern:         regexp.MustCompile(emailRegexPattern),
		numberPattern:        regexp.MustCompile(numberRegexPattern),
		referencePattern:     regexp.MustCompile(referenceRegexPattern),
		citationPattern:      regexp.MustCompile(citationRegexPattern),
		whitespacePattern:    regexp.MustCompile(whitespaceRegexPattern),
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
// This implementation uses a simple dictionary lookup as a robust, explicit starting
// point.
func (p *Preprocessor) ConvertToPhonemes(text string) string {
	phonemeDict := p.getPhonemeDict()

	var result strings.Builder

	words := strings.Fields(strings.ToLower(text))

	for i, word := range words {
		phoneme := p.convertWordToPhoneme(word, phonemeDict)
		result.WriteString(phoneme)

		if i < len(words)-1 {
			result.WriteString(" ")
		}
	}

	return result.String()
}

// ConvertToPhonemes transforms cleaned text into a phonetic representation.
// This is a critical step for the acoustic model in a TTS pipeline.
// This implementation uses a simple dictionary lookup as a robust, explicit starting
// point.
func (p *Preprocessor) getPhonemeDict() map[string]string {
	return map[string]string{
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
}

func (p *Preprocessor) convertWordToPhoneme(
	word string,
	phonemeDict map[string]string,
) string {
	cleanWord := strings.Trim(word, `.,!?;:"'`)
	if phonemes, found := phonemeDict[cleanWord]; found {
		return phonemes
	}

	return cleanWord
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
func (p *Preprocessor) preserveTokens(
	text string,
) (processedText string, placeholders map[string]string) {
	placeholders = make(map[string]string)

	counter := 0

	replaceFunc := func(pattern *regexp.Regexp, placeholderFormat string) {
		processedText = pattern.ReplaceAllStringFunc(
			processedText,
			func(match string) string {
				placeholder := fmt.Sprintf(placeholderFormat, counter)

				placeholders[placeholder] = match
				counter++

				return placeholder
			},
		)
	}

	processedText = text

	replaceFunc(p.urlPattern, urlPlaceholderPattern)
	replaceFunc(p.emailPattern, emailPlaceholderPattern)

	return processedText, placeholders
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
	replacer := strings.NewReplacer(carriageReturn, " ", lineFeed, " ", tabChar, " ")

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
		emDash, "-",
		enDash, "-",
		figureDash, "-",
		ellipsisChar, ellipsis,
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
type numberConverter struct {
	ones  []string
	teens []string
	tens  []string
}

func newNumberConverter() *numberConverter {
	return &numberConverter{
		ones: []string{
			"", "one", "two", "three", "four", "five",
			"six", "seven", "eight", "nine",
		},
		teens: []string{
			"ten", "eleven", "twelve", "thirteen", "fourteen",
			"fifteen", "sixteen", "seventeen", "eighteen", "nineteen",
		},
		tens: []string{
			"", "", "twenty", "thirty", "forty", "fifty",
			"sixty", "seventy", "eighty", "ninety",
		},
	}
}

func (nc *numberConverter) convertUnderTen(num int) string {
	return nc.ones[num]
}

func (nc *numberConverter) convertTeens(num int) string {
	return nc.teens[num-NumberBaseTen]
}

func (nc *numberConverter) convertTens(num int) string {
	result := nc.tens[num/NumberBaseTen]
	if num%NumberBaseTen > 0 {
		result += " " + nc.ones[num%NumberBaseTen]
	}

	return result
}

func (nc *numberConverter) convertHundreds(num int) string {
	result := nc.ones[num/NumberBaseHundred] + " hundred"

	remainder := num % NumberBaseHundred
	if remainder > 0 {
		result += " " + nc.convertUnderHundred(remainder)
	}

	return result
}

func (nc *numberConverter) convertUnderHundred(num int) string {
	if num < NumberBaseTen {
		return nc.convertUnderTen(num)
	}

	if num < NumberBaseTwenty {
		return nc.convertTeens(num)
	}

	return nc.convertTens(num)
}

func (nc *numberConverter) processThousands(number int, parts *[]string) int {
	thousands := number / NumberBaseThousand
	if thousands > 0 {
		*parts = append(*parts, nc.convertHundreds(thousands)+" thousand")
	}

	return number % NumberBaseThousand
}

func (nc *numberConverter) processHundreds(number int, parts *[]string) int {
	hundreds := number / NumberBaseHundred
	if hundreds > 0 {
		*parts = append(*parts, nc.convertUnderTen(hundreds)+" hundred")
	}

	return number % NumberBaseHundred
}

func integerToWords(number int) string {
	if number < 0 || number > MaxNumberForWords {
		return strconv.Itoa(number)
	}

	if number == 0 {
		return "zero"
	}

	numberConverter := newNumberConverter()

	var parts []string

	remaining := numberConverter.processThousands(number, &parts)

	remaining = numberConverter.processHundreds(remaining, &parts)

	if remaining > 0 {
		parts = append(parts, numberConverter.convertUnderHundred(remaining))
	}

	return strings.Join(parts, " ")
}
