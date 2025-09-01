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
	"slices"
	"strings"

	"github.com/nnikolov3/configurator"
)

const (
	dirPermissions = 0o750
)

// Static errors.
var (
	ErrModelPathEmpty   = errors.New("model_path cannot be empty")
	ErrWorkersPositive  = errors.New("workers must be positive")
	ErrTimeoutPositive  = errors.New("timeout_seconds must be positive")
	ErrServiceHostEmpty = errors.New(
		"service_host cannot be empty when using HTTP service",
	)
	ErrServicePortRange    = errors.New("service_port must be between 1 and 65535")
	ErrGPUMemoryPositive   = errors.New("gpu_memory_limit_gb must be positive")
	ErrTemperatureRange    = errors.New("temperature must be between 0 and 2")
	ErrInvalidQuality      = errors.New("quality must be one of the valid options")
	ErrFieldCannotBeEmpty  = errors.New("field cannot be empty")
	ErrLogDirEmpty         = errors.New("log_dir cannot be empty")
	ErrInvalidLevel        = errors.New("level must be one of the valid options")
	ErrMaxFileSizePositive = errors.New("max_file_size_mb must be positive")
	ErrMaxFilesPositive    = errors.New("max_files must be positive")
)

// Helper functions for dynamic error messages.
func newInvalidQualityError(validQualities []string) error {
	return fmt.Errorf(
		"%w: %s",
		ErrInvalidQuality,
		strings.Join(validQualities, commaSeparator),
	)
}

func newFieldCannotBeEmptyError(fieldName string) error {
	return fmt.Errorf("%w: %s", ErrFieldCannotBeEmpty, fieldName)
}

func newInvalidLevelError(validLevels []string) error {
	return fmt.Errorf(
		"%w: %s",
		ErrInvalidLevel,
		strings.Join(validLevels, commaSeparator),
	)
}

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
	cfg.resolvePaths(projectRoot)

	// Validate configuration
	validationErr := cfg.Validate()
	if validationErr != nil {
		return nil, "", fmt.Errorf(errInvalidConfiguration, validationErr)
	}

	return &cfg, projectRoot, nil
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

// Validate validates the TTS configuration by checking basic parameters,
// HTTP service settings, and advanced parameters.
func (c *TTSConfig) Validate() error {
	basicErr := c.validateBasicParams()
	if basicErr != nil {
		return basicErr
	}

	httpErr := c.validateHTTPService()
	if httpErr != nil {
		return httpErr
	}

	return c.validateAdvancedParams()
}

// Validate validates the paths configuration.
func (c *PathsConfig) Validate() error {
	fieldsToValidate := []struct {
		Name  string
		Value string
	}{
		{"input_dir", c.InputDir},
		{"output_dir", c.OutputDir},
		{"logs_dir", c.LogsDir},
		{"models_dir", c.ModelsDir},
	}

	for _, field := range fieldsToValidate {
		if field.Value == "" {
			return newFieldCannotBeEmptyError(field.Name)
		}
	}

	return nil
}

// Validate validates the logging configuration.
func (c *LoggingConfig) Validate() error {
	if c.LogDir == "" {
		return ErrLogDirEmpty
	}

	validLevels := []string{"debug", "info", "warn", "error"}
	if !slices.Contains(validLevels, c.Level) {
		return newInvalidLevelError(validLevels)
	}

	if c.MaxFileSizeMB <= 0 {
		return ErrMaxFileSizePositive
	}

	if c.MaxFiles <= 0 {
		return ErrMaxFilesPositive
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

// Validate validates the TTS configuration.
func (c *TTSConfig) validateBasicParams() error {
	if c.ModelPath == "" {
		return ErrModelPathEmpty
	}

	if c.Workers <= 0 {
		return ErrWorkersPositive
	}

	if c.TimeoutSeconds <= 0 {
		return ErrTimeoutPositive
	}

	return nil
}

func (c *TTSConfig) validateHTTPService() error {
	if !c.UseHTTPService {
		return nil
	}

	if c.ServiceHost == "" {
		return ErrServiceHostEmpty
	}

	if c.ServicePort <= 0 || c.ServicePort > 65535 {
		return ErrServicePortRange
	}

	return nil
}

func (c *TTSConfig) validateAdvancedParams() error {
	if c.GPUMemoryLimitGB <= 0 {
		return ErrGPUMemoryPositive
	}

	if c.Temperature < 0 || c.Temperature > 2 {
		return ErrTemperatureRange
	}

	validQualities := []string{"fast", "balanced", "high"}
	if !slices.Contains(validQualities, c.Quality) {
		return newInvalidQualityError(validQualities)
	}

	return nil
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
		err := os.MkdirAll(dir, dirPermissions)
		if err != nil {
			return fmt.Errorf(errFailedToCreateDir, dir, err)
		}
	}

	return nil
}

// resolvePaths converts relative paths to absolute paths based on project root.
func (c *Config) resolveTTSPath(projectRoot string) {
	if !filepath.IsAbs(c.TTS.ModelPath) {
		c.TTS.ModelPath = filepath.Join(projectRoot, c.TTS.ModelPath)
	}
}

func (c *Config) resolveDirectoryPaths(projectRoot string) {
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
}

func (c *Config) resolveLoggingPath(projectRoot string) {
	if !filepath.IsAbs(c.Logging.LogDir) {
		c.Logging.LogDir = filepath.Join(projectRoot, c.Logging.LogDir)
	}
}

func (c *Config) resolvePaths(projectRoot string) {
	c.resolveTTSPath(projectRoot)
	c.resolveDirectoryPaths(projectRoot)
	c.resolveLoggingPath(projectRoot)
}
