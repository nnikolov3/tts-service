// Package audio provides audio data structures, validation, and processing for TTS
// applications.
// This package follows Go coding standards for explicit behavior and maintainable code.
//
// NOTE: This file was renamed from playback.go to better reflect its purpose.
package audio

import (
	"errors"
	"fmt"
	"time"
)

// Constants for default audio quality settings.
const (
	DefaultSampleRate = 44100 // Standard CD quality sample rate.
	DefaultBitDepth   = 16    // Standard CD quality bit depth.
	DefaultChannels   = 2     // Stereo channels.
)

// Constants for supported bit depths.
const (
	BitDepth8  = 8
	BitDepth16 = 16
	BitDepth24 = 24
	BitDepth32 = 32
)

// Constants for quality validation limits.
const (
	MaxSampleRate      = 192000
	MaxChannels        = 8
	MaxVolume          = 10.0
	MaxFilterFrequency = 20000
)

// Constants for error messages and formats.
const (
	ErrFmtSampleRateRange    = "%w: sample rate must be between 1 and %d Hz"
	ErrFmtBitDepthValues     = "%w: bit depth must be 8, 16, 24, or 32"
	ErrFmtChannelsRange      = "%w: channels must be between 1 and %d"
	ErrFmtFadeInNonNegative  = "%w: fade in must be non-negative"
	ErrFmtFadeOutNonNegative = "%w: fade out must be non-negative"
	ErrFmtHighPassRange      = "%w: high pass filter must be between 0 and %d Hz"
	ErrFmtLowPassRange       = "%w: low pass filter must be between 0 and %d Hz"
	ErrFmtVolumeRange        = "%w: volume must be between 0.0 and %.1f"
)

// Common errors for the audio package.
var (
	ErrInvalidQuality = errors.New("invalid quality settings")
)

// Format represents supported audio formats.
type Format string

const (
	// FormatWAV represents WAV audio format.
	FormatWAV Format = "wav"
	// FormatMP3 represents MP3 audio format.
	FormatMP3 Format = "mp3"
	// FormatFLAC represents FLAC audio format.
	FormatFLAC Format = "flac"
	// FormatOGG represents OGG audio format.
	FormatOGG Format = "ogg"
	// FormatM4A represents M4A audio format.
	FormatM4A Format = "m4a"
	// FormatAAC represents AAC audio format.
	FormatAAC Format = "aac"
)

// Quality represents audio quality settings and post-processing effects.
type Quality struct {
	Codec      string  `json:"codec"`
	Bitrate    string  `json:"bitrate,omitempty"`
	SampleRate int     `json:"sampleRate"`
	BitDepth   int     `json:"bitDepth"`
	Channels   int     `json:"channels"`
	Volume     float64 `json:"volume"`
	FadeIn     float64 `json:"fadeIn,omitempty"`
	FadeOut    float64 `json:"fadeOut,omitempty"`
	HighPass   int     `json:"highPass,omitempty"`
	LowPass    int     `json:"lowPass,omitempty"`
	Normalize  bool    `json:"normalize"`
}

// PlaybackResponse represents information about a generated audio file.
type PlaybackResponse struct {
	Error      string        `json:"error,omitempty"`
	Format     Format        `json:"format"`
	Duration   time.Duration `json:"duration"`
	FileSize   int64         `json:"fileSize"`
	SampleRate int           `json:"sampleRate"`
	Channels   int           `json:"channels"`
	Success    bool          `json:"success"`
}

// NewDefaultQuality provides sensible default audio quality settings.
func NewDefaultQuality() Quality {
	return Quality{
		Codec:      "pcm_s16le",
		Bitrate:    "",
		SampleRate: DefaultSampleRate,
		BitDepth:   DefaultBitDepth,
		Channels:   DefaultChannels,
		Volume:     1.0,
		FadeIn:     0.0,
		FadeOut:    0.0,
		HighPass:   0,
		LowPass:    0,
		Normalize:  false,
	}
}

// Validate checks if quality settings are within reasonable bounds.
func (q *Quality) Validate() error {
	audioParamsErr := q.validateAudioParams()
	if audioParamsErr != nil {
		return audioParamsErr
	}

	effectParamsErr := q.validateEffectParams()
	if effectParamsErr != nil {
		return effectParamsErr
	}

	return nil
}

// ApplyEffects processes raw audio data according to the Quality settings.
// Currently returns the audio data unchanged as no effects are implemented.
func (q *Quality) ApplyEffects(audioData []byte) ([]byte, error) {
	return audioData, nil
}

// validateAudioParams checks core audio settings, now using unique error variables.
func (q *Quality) validateAudioParams() error {
	sampleRateErr := validateSampleRate(q.SampleRate)
	if sampleRateErr != nil {
		return sampleRateErr
	}

	bitDepthErr := validateBitDepth(q.BitDepth)
	if bitDepthErr != nil {
		return bitDepthErr
	}

	channelsErr := validateChannels(q.Channels)
	if channelsErr != nil {
		return channelsErr
	}

	return nil
}

// validateEffectParams checks audio effect settings, now using unique error variables.
func (q *Quality) validateVolumeAndFade() error {
	volumeErr := validateVolume(q.Volume)
	if volumeErr != nil {
		return volumeErr
	}

	fadeInErr := validateFadeIn(q.FadeIn)
	if fadeInErr != nil {
		return fadeInErr
	}

	fadeOutErr := validateFadeOut(q.FadeOut)
	if fadeOutErr != nil {
		return fadeOutErr
	}

	return nil
}

func (q *Quality) validateFilterParams() error {
	highPassErr := validateHighPass(q.HighPass)
	if highPassErr != nil {
		return highPassErr
	}

	lowPassErr := validateLowPass(q.LowPass)
	if lowPassErr != nil {
		return lowPassErr
	}

	return nil
}

func (q *Quality) validateEffectParams() error {
	volumeFadeErr := q.validateVolumeAndFade()
	if volumeFadeErr != nil {
		return volumeFadeErr
	}

	return q.validateFilterParams()
}

//
// Validation Helpers
//

func validateSampleRate(sampleRate int) error {
	if sampleRate <= 0 || sampleRate > MaxSampleRate {
		return fmt.Errorf(
			ErrFmtSampleRateRange,
			ErrInvalidQuality,
			MaxSampleRate,
		)
	}

	return nil
}

func validateBitDepth(bitDepth int) error {
	switch bitDepth {
	case BitDepth8, BitDepth16, BitDepth24, BitDepth32:
		return nil
	default:
		return fmt.Errorf(ErrFmtBitDepthValues, ErrInvalidQuality)
	}
}

func validateChannels(channels int) error {
	if channels <= 0 || channels > MaxChannels {
		return fmt.Errorf(ErrFmtChannelsRange, ErrInvalidQuality, MaxChannels)
	}

	return nil
}

func validateVolume(volume float64) error {
	if volume < 0.0 || volume > MaxVolume {
		return fmt.Errorf(ErrFmtVolumeRange, ErrInvalidQuality, MaxVolume)
	}

	return nil
}

func validateFadeIn(fadeIn float64) error {
	if fadeIn < 0.0 {
		return fmt.Errorf(ErrFmtFadeInNonNegative, ErrInvalidQuality)
	}

	return nil
}

func validateFadeOut(fadeOut float64) error {
	if fadeOut < 0.0 {
		return fmt.Errorf(ErrFmtFadeOutNonNegative, ErrInvalidQuality)
	}

	return nil
}

func validateHighPass(highPass int) error {
	if highPass < 0 || highPass > MaxFilterFrequency {
		return fmt.Errorf(
			ErrFmtHighPassRange,
			ErrInvalidQuality,
			MaxFilterFrequency,
		)
	}

	return nil
}

func validateLowPass(lowPass int) error {
	if lowPass < 0 || lowPass > MaxFilterFrequency {
		return fmt.Errorf(
			ErrFmtLowPassRange,
			ErrInvalidQuality,
			MaxFilterFrequency,
		)
	}

	return nil
}
