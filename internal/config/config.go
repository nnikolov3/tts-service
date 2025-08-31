// Package config provides configuration management for TTS applications.
//
// This package implements configuration functionality that was previously
// handled by Python utilities, following Go coding standards and design
// principles for explicit behavior and maintainable code.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nnikolov3/configurator"
)

// Error messages.
const (
	errFailedToLoadProjectConfig = "failed to load project config: %w"
	errFailedToResolvePaths      = "failed to resolve paths: %w"
	errInvalidConfiguration      = "invalid configuration: %w"
	errTTSConfig                 = "TTS config: %w"
	errPathsConfig               = "paths config: %w"
	errLoggingConfig             = "logging config: %w"
	errModelPathEmpty            = "model_path cannot be empty"
	errServiceHostEmpty          = "service_host cannot be empty when using HTTP service"
	errServicePortRange          = "service_port must be between 1 and 65535"
	errWorkersPositive           = "workers must be positive"
	errTimeoutPositive           = "timeout_seconds must be positive"
	errGPUMemoryPositive         = "gpu_memory_limit_gb must be positive"
	errTemperatureRange          = "temperature must be between 0 and 2"
	errQualityMustBeOneOf        = "quality must be one of: %s"
	errCannotBeEmpty             = "%s cannot be empty"
	errLogDirEmpty               = "log_dir cannot be empty"
	errLevelMustBeOneOf          = "level must be one of: %s"
	errMaxFileSizePositive       = "max_file_size_mb must be positive"
	errMaxFilesPositive          = "max_files must be positive"
	errFailedToCreateDir         = "failed to create directory %s: %w"
)

// Device constants.
const (
	deviceAuto = "auto"
	deviceCUDA = "cuda"
	deviceCPU  = "cpu"
)

// URL format.
const (
	serviceURLFormat = "http://%s:%d"
)

// Separator.
const (
	commaSeparator = ", "
)

// Config represents the complete TTS project configuration.
type Config struct {
	Paths   PathsConfig   `toml:"paths"`
	Logging LoggingConfig `toml:"logging"`
	TTS     TTSConfig     `toml:"tts"`
}

// TTSConfig represents TTS-specific configuration.
type TTSConfig struct {
	ModelPath         string  `toml:"model_path"`
	ServiceHost       string  `toml:"service_host"`
	Quality           string  `toml:"quality"`
	Device            string  `toml:"device"`
	Workers           int     `toml:"workers"`
	TopP              float64 `toml:"top_p"`
	GPUMemoryLimitGB  float64 `toml:"gpu_memory_limit_gb"`
	MirostatEta       float64 `toml:"mirostat_eta"`
	MirostatTau       float64 `toml:"mirostat_tau"`
	ServicePort       int     `toml:"service_port"`
	Temperature       float64 `toml:"temperature"`
	RepetitionPenalty float64 `toml:"repetition_penalty"`
	RepetitionRange   int     `toml:"repetition_range"`
	TopK              int     `toml:"top_k"`
	TimeoutSeconds    int     `toml:"timeout_seconds"`
	MinP              float64 `toml:"min_p"`
	Mirostat          bool    `toml:"mirostat"`
	UseHTTPService    bool    `toml:"use_http_service"`
	UseGPU            bool    `toml:"use_gpu"`
}

// PathsConfig represents directory path configuration.
type PathsConfig struct {
	InputDir  string `toml:"input_dir"`
	OutputDir string `toml:"output_dir"`
	LogsDir   string `toml:"logs_dir"`
	ModelsDir string `toml:"models_dir"`
}

// LoggingConfig represents logging configuration.
type LoggingConfig struct {
	Level         string `toml:"level"`
	LogDir        string `toml:"log_dir"`
	MaxFileSizeMB int    `toml:"max_file_size_mb"`
	MaxFiles      int    `toml:"max_files"`
}

// Load loads the project configuration from project.toml starting from the given
// directory.
func Load(startDir string) (*Config, string, error) {
	var cfg Config

	projectRoot, err := configurator.LoadFromProject(startDir, &cfg)
	if err != nil {
		return nil, "", fmt.Errorf(errFailedToLoadProjectConfig, err)
	}

	// Resolve relative paths to absolute paths based on project root
	if err := cfg.resolvePaths(projectRoot); err != nil {
		return nil, "", fmt.Errorf(errFailedToResolvePaths, err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, "", fmt.Errorf(errInvalidConfiguration, err)
	}

	return &cfg, projectRoot, nil
}

// resolvePaths converts relative paths to absolute paths based on project root.
func (c *Config) resolvePaths(projectRoot string) error {
	// Resolve TTS model path if relative
	if !filepath.IsAbs(c.TTS.ModelPath) {
		c.TTS.ModelPath = filepath.Join(projectRoot, c.TTS.ModelPath)
	}

	// Resolve directory paths if relative
	if !filepath.IsAbs(c.Paths.InputDir) {
		c.Paths.InputDir = filepath.Join(projectRoot, c.Paths.InputDir)
	}

	if !filepath.IsAbs(c.Paths.OutputDir) {
		c.Paths.OutputDir = filepath.Join(projectRoot, c.Paths.OutputDir)
	}

	if !filepath.IsAbs(c.Paths.LogsDir) {
		c.Paths.LogsDir = filepath.Join(projectRoot, c.Paths.LogsDir)
	}

	if !filepath.IsAbs(c.Paths.ModelsDir) {
		c.Paths.ModelsDir = filepath.Join(projectRoot, c.Paths.ModelsDir)
	}

	// Resolve logging directory if relative
	if !filepath.IsAbs(c.Logging.LogDir) {
		c.Logging.LogDir = filepath.Join(projectRoot, c.Logging.LogDir)
	}

	return nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	err := c.TTS.Validate()
	if err != nil {
		return fmt.Errorf(errTTSConfig, err)
	}

	err = c.Paths.Validate()
	if err != nil {
		return fmt.Errorf(errPathsConfig, err)
	}

	err = c.Logging.Validate()
	if err != nil {
		return fmt.Errorf(errLoggingConfig, err)
	}

	return nil
}

// Validate validates the TTS configuration.
func (c *TTSConfig) Validate() error {
	if c.ModelPath == "" {
		return errors.New(errModelPathEmpty)
	}

	if c.UseHTTPService {
		if c.ServiceHost == "" {
			return errors.New(errServiceHostEmpty)
		}

		if c.ServicePort <= 0 || c.ServicePort > 65535 {
			return errors.New(errServicePortRange)
		}
	}

	if c.Workers <= 0 {
		return errors.New(errWorkersPositive)
	}

	if c.TimeoutSeconds <= 0 {
		return errors.New(errTimeoutPositive)
	}

	if c.GPUMemoryLimitGB <= 0 {
		return errors.New(errGPUMemoryPositive)
	}

	if c.Temperature < 0 || c.Temperature > 2 {
		return errors.New(errTemperatureRange)
	}

	validQualities := []string{"fast", "balanced", "high"}
	if !contains(validQualities, c.Quality) {
		return fmt.Errorf(
			errQualityMustBeOneOf,
			strings.Join(validQualities, commaSeparator),
		)
	}

	return nil
}

// Validate validates the paths configuration.
func (c *PathsConfig) Validate() error {
	paths := map[string]string{
		"input_dir":  c.InputDir,
		"output_dir": c.OutputDir,
		"logs_dir":   c.LogsDir,
		"models_dir": c.ModelsDir,
	}

	for name, path := range paths {
		if path == "" {
			return fmt.Errorf(errCannotBeEmpty, name)
		}
	}

	return nil
}

// Validate validates the logging configuration.
func (c *LoggingConfig) Validate() error {
	if c.LogDir == "" {
		return errors.New(errLogDirEmpty)
	}

	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, c.Level) {
		return fmt.Errorf(
			errLevelMustBeOneOf,
			strings.Join(validLevels, commaSeparator),
		)
	}

	if c.MaxFileSizeMB <= 0 {
		return errors.New(errMaxFileSizePositive)
	}

	if c.MaxFiles <= 0 {
		return errors.New(errMaxFilesPositive)
	}

	return nil
}

// GetServiceURL returns the full URL for the TTS service.
func (c *TTSConfig) GetServiceURL() string {
	return fmt.Sprintf(serviceURLFormat, c.ServiceHost, c.ServicePort)
}

// GetDevice returns the device to use for TTS processing.
func (c *TTSConfig) GetDevice() string {
	if c.Device != deviceAuto {
		return c.Device
	}

	if c.UseGPU {
		return deviceCUDA
	}

	return deviceCPU
}

// EnsureDirectories creates all configured directories if they don't exist.
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.Paths.InputDir,
		c.Paths.OutputDir,
		c.Paths.LogsDir,
		c.Paths.ModelsDir,
		c.Logging.LogDir,
	}

	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			return fmt.Errorf(errFailedToCreateDir, dir, err)
		}
	}

	return nil
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}

	return false
}
