// Package config provides the configuration structure for the tts-service.
package config

import (
	"fmt"
	"github.com/book-expert/configurator"
	"github.com/book-expert/logger"
)

// NATSConfig holds the configuration for NATS.
type NATSConfig struct {
	URL                      string `toml:"url"`
	TTStreamName             string `toml:"tts_stream_name"`
	TTSConsumerName          string `toml:"tts_consumer_name"`
	TextProcessedSubject     string `toml:"text_processed_subject"`
	AudioChunkCreatedSubject string `toml:"audio_chunk_created_subject"`
	AudioObjectStoreBucket   string `toml:"audio_object_store_bucket"`
}

// TTSServiceConfig holds the specific configuration for the TTS service.
type TTSServiceConfig struct {
	ModelPath      string  `toml:"model_path"`
	Temperature    float64 `toml:"temperature"`
	TimeoutSeconds int     `toml:"timeout_seconds"`
}

// PathsConfig holds the configuration for file paths.
type PathsConfig struct {
	BaseLogsDir string `toml:"base_logs_dir"`
}

// Config is the root configuration structure.
type Config struct {
	NATS  NATSConfig       `toml:"nats"`
	TTS   TTSServiceConfig `toml:"tts_service"`
	Paths PathsConfig      `toml:"paths"`
}

// Load loads the configuration for the tts-service.
func Load(log *logger.Logger) (*Config, error) {
	var cfg Config

	err := configurator.Load(&cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration from configurator: %w", err)
	}

	return &cfg, nil
}
