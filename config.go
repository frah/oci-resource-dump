package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppConfig represents the YAML configuration structure
// Phase 2C: Configuration with filtering and diff support
type AppConfig struct {
	Version string        `yaml:"version"`
	General GeneralConfig `yaml:"general"`
	Output  OutputConfig  `yaml:"output"`
	Filters FilterConfig  `yaml:"filters"`
	Diff    DiffConfig    `yaml:"diff"`
}

// GeneralConfig holds general execution settings
type GeneralConfig struct {
	Timeout      int    `yaml:"timeout"`       // Timeout in seconds
	LogLevel     string `yaml:"log_level"`     // Log level: silent, normal, verbose, debug
	OutputFormat string `yaml:"output_format"` // Output format: json, csv, tsv
	Progress     bool   `yaml:"progress"`      // Progress bar display
}

// OutputConfig holds output-related settings
type OutputConfig struct {
	File string `yaml:"file"` // Output file path (empty = stdout)
}

// Default configuration values
func getDefaultConfig() *AppConfig {
	return &AppConfig{
		Version: "1.0",
		General: GeneralConfig{
			Timeout:      300,    // 5 minutes default
			LogLevel:     "normal",
			OutputFormat: "json",
			Progress:     true,
		},
		Output: OutputConfig{
			File: "", // stdout by default
		},
		Filters: FilterConfig{
			IncludeCompartments:  []string{},
			ExcludeCompartments:  []string{},
			IncludeResourceTypes: []string{},
			ExcludeResourceTypes: []string{},
			NamePattern:          "",
			ExcludeNamePattern:   "",
		},
		Diff: DiffConfig{
			Format:     "json",
			Detailed:   false,
			OutputFile: "",
		},
	}
}

// Configuration file search paths in priority order
func getConfigPaths() []string {
	paths := []string{}
	
	// 1. Environment variable
	if configFile := os.Getenv("OCI_DUMP_CONFIG_FILE"); configFile != "" {
		paths = append(paths, configFile)
	}
	
	// 2. Current directory
	paths = append(paths, "./oci-resource-dump.yaml")
	
	// 3. Home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(homeDir, ".oci-resource-dump.yaml"))
	}
	
	// 4. System directory
	paths = append(paths, "/etc/oci-resource-dump.yaml")
	
	return paths
}

// LoadConfig loads configuration from YAML file with fallback to defaults
func LoadConfig() (*AppConfig, error) {
	// Start with default configuration
	config := getDefaultConfig()
	
	// Try to find and load configuration file
	for _, path := range getConfigPaths() {
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse configuration file %s: %w", path, err)
			}
			
			break // Use first found configuration file
		}
	}
	
	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return config, nil
}

// validateConfig validates the loaded configuration
func validateConfig(config *AppConfig) error {
	// Validate log level
	validLogLevels := []string{"silent", "normal", "verbose", "debug"}
	if !contains(validLogLevels, config.General.LogLevel) {
		return fmt.Errorf("invalid log_level '%s', must be one of: %v", config.General.LogLevel, validLogLevels)
	}
	
	// Validate output format
	validFormats := []string{"json", "csv", "tsv"}
	if !contains(validFormats, config.General.OutputFormat) {
		return fmt.Errorf("invalid output_format '%s', must be one of: %v", config.General.OutputFormat, validFormats)
	}
	
	// Validate timeout
	if config.General.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %d", config.General.Timeout)
	}
	
	return nil
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// SaveConfig saves the current configuration to a YAML file
func SaveConfig(config *AppConfig, filename string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}
	
	return nil
}

// GenerateDefaultConfigFile creates a default configuration file
func GenerateDefaultConfigFile(filename string) error {
	config := getDefaultConfig()
	return SaveConfig(config, filename)
}

// MergeWithCLIArgs merges configuration file settings with CLI arguments
// CLI arguments have higher priority than configuration file
func MergeWithCLIArgs(config *AppConfig, cliTimeout *int, cliLogLevel *string, cliFormat *string, cliProgress *bool, cliOutputFile *string) {
	// CLI timeout overrides config
	if cliTimeout != nil {
		config.General.Timeout = *cliTimeout
	}
	
	// CLI log level overrides config
	if cliLogLevel != nil && *cliLogLevel != "" {
		config.General.LogLevel = *cliLogLevel
	}
	
	// CLI format overrides config
	if cliFormat != nil && *cliFormat != "" {
		config.General.OutputFormat = *cliFormat
	}
	
	// CLI progress overrides config
	if cliProgress != nil {
		config.General.Progress = *cliProgress
	}
	
	// CLI output file overrides config
	if cliOutputFile != nil && *cliOutputFile != "" {
		config.Output.File = *cliOutputFile
	}
}