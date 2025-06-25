package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()

	// バージョンチェック
	if config.Version != "1.0" {
		t.Errorf("getDefaultConfig() Version = %v, want 1.0", config.Version)
	}

	// General設定チェック
	if config.General.Timeout != 300 {
		t.Errorf("getDefaultConfig() General.Timeout = %v, want 300", config.General.Timeout)
	}
	if config.General.LogLevel != "normal" {
		t.Errorf("getDefaultConfig() General.LogLevel = %v, want normal", config.General.LogLevel)
	}
	if config.General.OutputFormat != "json" {
		t.Errorf("getDefaultConfig() General.OutputFormat = %v, want json", config.General.OutputFormat)
	}
	if config.General.Progress != true {
		t.Errorf("getDefaultConfig() General.Progress = %v, want true", config.General.Progress)
	}

	// Output設定チェック
	if config.Output.File != "" {
		t.Errorf("getDefaultConfig() Output.File = %v, want empty string", config.Output.File)
	}

	// Filters設定チェック（空のスライスが期待値）
	if len(config.Filters.IncludeCompartments) != 0 {
		t.Errorf("getDefaultConfig() Filters.IncludeCompartments = %v, want empty slice", config.Filters.IncludeCompartments)
	}
	if len(config.Filters.ExcludeCompartments) != 0 {
		t.Errorf("getDefaultConfig() Filters.ExcludeCompartments = %v, want empty slice", config.Filters.ExcludeCompartments)
	}

	// Diff設定チェック
	if config.Diff.Format != "json" {
		t.Errorf("getDefaultConfig() Diff.Format = %v, want json", config.Diff.Format)
	}
	if config.Diff.Detailed != false {
		t.Errorf("getDefaultConfig() Diff.Detailed = %v, want false", config.Diff.Detailed)
	}
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	config := &AppConfig{
		Version: "1.0",
		General: GeneralConfig{
			Timeout:      300,
			LogLevel:     "normal",
			OutputFormat: "json",
			Progress:     false,
		},
		Output: OutputConfig{
			File: "",
		},
		Filters: FilterConfig{},
		Diff: DiffConfig{
			Format:   "json",
			Detailed: false,
		},
	}

	err := validateConfig(config)
	if err != nil {
		t.Errorf("validateConfig() error = %v, want nil", err)
	}
}

func TestValidateConfig_InvalidTimeout(t *testing.T) {
	config := getDefaultConfig()
	config.General.Timeout = -1

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() error = nil, want error for negative timeout")
	}
}

func TestValidateConfig_InvalidLogLevel(t *testing.T) {
	config := getDefaultConfig()
	config.General.LogLevel = "invalid"

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() error = nil, want error for invalid log level")
	}
}

func TestValidateConfig_InvalidOutputFormat(t *testing.T) {
	config := getDefaultConfig()
	config.General.OutputFormat = "invalid"

	err := validateConfig(config)
	if err == nil {
		t.Error("validateConfig() error = nil, want error for invalid output format")
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	// 一時ディレクトリを作成してカレントディレクトリを変更
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// カレントディレクトリを一時的に変更
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() error = %v, want nil", err)
	}

	// デフォルト値と比較
	defaultConfig := getDefaultConfig()
	if !reflect.DeepEqual(config, defaultConfig) {
		t.Error("LoadConfig() should return default config when no file exists")
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	// 一時ディレクトリを作成
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 有効なYAMLファイルを作成
	configContent := `version: "1.0"
general:
  timeout: 600
  log_level: "debug"
  output_format: "csv"
  progress: true
output:
  file: "test_output.json"
filters:
  include_compartments:
    - "ocid1.compartment.oc1..test"
  name_pattern: "test-*"
diff:
  detailed: true
`

	configPath := filepath.Join(tempDir, "oci-resource-dump.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// カレントディレクトリを一時的に変更
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() error = %v, want nil", err)
	}

	// 期待値と比較
	if config.General.Timeout != 600 {
		t.Errorf("LoadConfig() General.Timeout = %v, want 600", config.General.Timeout)
	}
	if config.General.LogLevel != "debug" {
		t.Errorf("LoadConfig() General.LogLevel = %v, want debug", config.General.LogLevel)
	}
	if config.General.OutputFormat != "csv" {
		t.Errorf("LoadConfig() General.OutputFormat = %v, want csv", config.General.OutputFormat)
	}
	if config.General.Progress != true {
		t.Errorf("LoadConfig() General.Progress = %v, want true", config.General.Progress)
	}
	if config.Output.File != "test_output.json" {
		t.Errorf("LoadConfig() Output.File = %v, want test_output.json", config.Output.File)
	}
	if len(config.Filters.IncludeCompartments) != 1 || config.Filters.IncludeCompartments[0] != "ocid1.compartment.oc1..test" {
		t.Errorf("LoadConfig() Filters.IncludeCompartments = %v, want [ocid1.compartment.oc1..test]", config.Filters.IncludeCompartments)
	}
	if config.Filters.NamePattern != "test-*" {
		t.Errorf("LoadConfig() Filters.NamePattern = %v, want test-*", config.Filters.NamePattern)
	}
	if config.Diff.Detailed != true {
		t.Errorf("LoadConfig() Diff.Detailed = %v, want true", config.Diff.Detailed)
	}
}

func TestLoadConfig_InvalidYaml(t *testing.T) {
	// 一時ディレクトリを作成
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 無効なYAMLファイルを作成
	configContent := `version: "1.0"
general:
  timeout: invalid_number
  log_level: [
`

	configPath := filepath.Join(tempDir, "oci-resource-dump.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// カレントディレクトリを一時的に変更
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	_, err = LoadConfig()
	if err == nil {
		t.Error("LoadConfig() error = nil, want error for invalid YAML")
	}
}

func TestGenerateDefaultConfigFile(t *testing.T) {
	// 一時ディレクトリを作成
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "oci-resource-dump.yaml")

	err = GenerateDefaultConfigFile(configPath)
	if err != nil {
		t.Errorf("GenerateDefaultConfigFile() error = %v, want nil", err)
	}

	// ファイルが存在することを確認
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("GenerateDefaultConfigFile() should create config file")
	}

	// ファイル内容を読み込んで検証
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read generated config file: %v", err)
	}

	// YAML内容の基本チェック
	if len(content) == 0 {
		t.Error("GenerateDefaultConfigFile() created empty file")
	}
}

func TestMergeWithCLIArgs(t *testing.T) {
	tests := []struct {
		name             string
		config           *AppConfig
		timeout          *int
		logLevel         *string
		outputFormat     *string
		progress         *bool
		outputFile       *string
		expectedFormat   string
		expectedTimeout  int
		expectedLogLevel string
		expectedProgress bool
		expectedFile     string
	}{
		{
			name:             "override format",
			config:           getDefaultConfig(),
			timeout:          nil,
			logLevel:         nil,
			outputFormat:     stringPtr("csv"),
			progress:         nil,
			outputFile:       nil,
			expectedFormat:   "csv",
			expectedTimeout:  300,
			expectedLogLevel: "normal",
			expectedProgress: true,
			expectedFile:     "",
		},
		{
			name:             "override timeout",
			config:           getDefaultConfig(),
			timeout:          intPtr(600),
			logLevel:         nil,
			outputFormat:     nil,
			progress:         nil,
			outputFile:       nil,
			expectedFormat:   "json",
			expectedTimeout:  600,
			expectedLogLevel: "normal",
			expectedProgress: true,
			expectedFile:     "",
		},
		{
			name:             "override all explicitly",
			config:           getDefaultConfig(),
			timeout:          intPtr(120),
			logLevel:         stringPtr("debug"),
			outputFormat:     stringPtr("tsv"),
			progress:         boolPtr(false), // explicitly set to false
			outputFile:       stringPtr("output.json"),
			expectedFormat:   "tsv",
			expectedTimeout:  120,
			expectedLogLevel: "debug",
			expectedProgress: false,
			expectedFile:     "output.json",
		},
		{
			name:             "CLI not specified (Issue #2/#3 reproduction)",
			config:           getDefaultConfig(),
			timeout:          intPtr(-1),           // CLI default (not specified)
			logLevel:         stringPtr("NOT_SET"), // CLI default (not specified)
			outputFormat:     stringPtr("NOT_SET"), // CLI default (not specified)
			progress:         nil,                  // No explicit flag (not specified)
			outputFile:       stringPtr("NOT_SET"), // CLI default (not specified)
			expectedFormat:   "json",               // Should keep config file value
			expectedTimeout:  300,                  // Should keep config file value (Issue #3)
			expectedLogLevel: "normal",             // Should keep config file value
			expectedProgress: true,                 // Should keep config file value
			expectedFile:     "",                   // Should keep config file value
		},
		{
			name:             "Mix of specified and not specified",
			config:           getDefaultConfig(),
			timeout:          intPtr(450),          // Explicitly specified
			logLevel:         stringPtr("NOT_SET"), // Not specified
			outputFormat:     stringPtr("csv"),     // Explicitly specified
			progress:         nil,                  // Not specified
			outputFile:       stringPtr("NOT_SET"), // Not specified
			expectedFormat:   "csv",                // Should use CLI override
			expectedTimeout:  450,                  // Should use CLI override
			expectedLogLevel: "normal",             // Should keep config file value
			expectedProgress: true,                 // Should keep config file value
			expectedFile:     "",                   // Should keep config file value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeWithCLIArgs(tt.config, tt.timeout, tt.logLevel, tt.outputFormat, tt.progress, tt.outputFile)

			if tt.config.General.OutputFormat != tt.expectedFormat {
				t.Errorf("MergeWithCLIArgs() OutputFormat = %v, want %v", tt.config.General.OutputFormat, tt.expectedFormat)
			}
			if tt.config.General.Timeout != tt.expectedTimeout {
				t.Errorf("MergeWithCLIArgs() Timeout = %v, want %v", tt.config.General.Timeout, tt.expectedTimeout)
			}
			if tt.config.General.LogLevel != tt.expectedLogLevel {
				t.Errorf("MergeWithCLIArgs() LogLevel = %v, want %v", tt.config.General.LogLevel, tt.expectedLogLevel)
			}
			if tt.config.General.Progress != tt.expectedProgress {
				t.Errorf("MergeWithCLIArgs() Progress = %v, want %v", tt.config.General.Progress, tt.expectedProgress)
			}
			if tt.config.Output.File != tt.expectedFile {
				t.Errorf("MergeWithCLIArgs() File = %v, want %v", tt.config.Output.File, tt.expectedFile)
			}
		})
	}
}

// ヘルパー関数
func intPtr(i int) *int          { return &i }
func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }

func TestGetConfigPaths(t *testing.T) {
	paths := getConfigPaths()

	// 最低3つのパスが返されることを確認（環境変数なしの場合）
	if len(paths) < 3 {
		t.Errorf("getConfigPaths() returned %d paths, want at least 3", len(paths))
	}

	// 現在のディレクトリパスが含まれることを確認
	foundCurrentDir := false
	for _, path := range paths {
		if path == "./oci-resource-dump.yaml" {
			foundCurrentDir = true
			break
		}
	}
	if !foundCurrentDir {
		t.Error("getConfigPaths() should include ./oci-resource-dump.yaml")
	}

	// システムディレクトリパスが含まれることを確認
	foundSystemDir := false
	for _, path := range paths {
		if path == "/etc/oci-resource-dump.yaml" {
			foundSystemDir = true
			break
		}
	}
	if !foundSystemDir {
		t.Error("getConfigPaths() should include /etc/oci-resource-dump.yaml")
	}
}

// TestIssue2and3_ConfigFileNotLoaded reproduces GitHub Issues #2 and #3
// This test simulates the exact scenario described in the issues
func TestIssue2and3_ConfigFileNotLoaded(t *testing.T) {
	// 一時ディレクトリを作成
	tempDir, err := os.MkdirTemp("", "issue_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// config.yaml with timeout=300 (Issue #3 test case)
	configContent := `version: "1.0"
general:
  timeout: 300
  log_level: "verbose"
  output_format: "json"
  progress: true
output:
  file: ""
`

	configPath := filepath.Join(tempDir, "oci-resource-dump.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// カレントディレクトリを一時的に変更
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Simulate CLI arguments with default values (Issue reproduction)
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Before fix: CLI defaults would override config file
	// After fix: CLI defaults should NOT override config file

	// Test BEFORE applying MergeWithCLIArgs - config should be loaded correctly
	if config.General.Timeout != 300 {
		t.Errorf("Config file loading failed: timeout = %v, want 300", config.General.Timeout)
	}
	if config.General.LogLevel != "verbose" {
		t.Errorf("Config file loading failed: log_level = %v, want verbose", config.General.LogLevel)
	}

	// Simulate CLI arguments with special "not specified" values (after fix)
	timeoutCLI := -1            // Special value meaning "not specified"
	logLevelCLI := "NOT_SET"    // Special value meaning "not specified"
	formatCLI := "NOT_SET"      // Special value meaning "not specified"
	var progressCLI *bool = nil // No explicit flag (not specified)
	outputFileCLI := "NOT_SET"  // Special value meaning "not specified"

	MergeWithCLIArgs(config, &timeoutCLI, &logLevelCLI, &formatCLI, progressCLI, &outputFileCLI)

	// After fix: config file values should be preserved
	if config.General.Timeout != 300 {
		t.Errorf("Issue #3 not fixed: timeout = %v, want 300 (config file value should be preserved)", config.General.Timeout)
	}
	if config.General.LogLevel != "verbose" {
		t.Errorf("Issue #2 not fixed: log_level = %v, want verbose (config file value should be preserved)", config.General.LogLevel)
	}
	if config.General.OutputFormat != "json" {
		t.Errorf("Issue #2 not fixed: output_format = %v, want json (config file value should be preserved)", config.General.OutputFormat)
	}
	if config.General.Progress != true {
		t.Errorf("Issue #2 not fixed: progress = %v, want true (config file value should be preserved)", config.General.Progress)
	}
}

// TestIssue2and3_BeforeFix simulates the broken behavior before the fix
// This test should FAIL before the fix and PASS after the fix
func TestIssue2and3_BeforeFix_SimulateBrokenBehavior(t *testing.T) {
	config := getDefaultConfig()

	// Modify config to simulate what would be loaded from a config file
	config.General.Timeout = 600        // From config file
	config.General.LogLevel = "debug"   // From config file
	config.General.OutputFormat = "csv" // From config file
	config.General.Progress = true      // From config file

	// Simulate old broken MergeWithCLIArgs behavior (before fix)
	// This is what would happen with CLI defaults before our fix
	brokenTimeoutCLI := 0      // Old CLI default
	brokenLogLevelCLI := ""    // Old CLI default
	brokenFormatCLI := ""      // Old CLI default
	brokenProgressCLI := false // CLI default
	brokenOutputFileCLI := ""  // Old CLI default

	// Simulate broken behavior - this would incorrectly override config values
	if brokenTimeoutCLI == 0 {
		// In broken version, this would set timeout to 0 (immediate timeout)
		// Don't actually apply the broken behavior, just verify our fix works
		t.Logf("Before fix: timeout would be incorrectly set to %d", brokenTimeoutCLI)
	}
	if brokenLogLevelCLI == "" {
		t.Logf("Before fix: log_level would be incorrectly set to empty string")
	}
	if brokenFormatCLI == "" {
		t.Logf("Before fix: format would be incorrectly set to empty string")
	}
	if !brokenProgressCLI {
		t.Logf("Before fix: progress would be incorrectly set to %t", brokenProgressCLI)
	}
	if brokenOutputFileCLI == "" {
		t.Logf("Before fix: output_file would be incorrectly set to empty string")
	}

	// Apply our fixed MergeWithCLIArgs with special "not specified" values
	fixedTimeoutCLI := -1           // Fixed: special value meaning "not specified"
	fixedLogLevelCLI := "NOT_SET"   // Fixed: special value meaning "not specified"
	fixedFormatCLI := "NOT_SET"     // Fixed: special value meaning "not specified"
	fixedProgressCLI := false       // CLI default
	fixedOutputFileCLI := "NOT_SET" // Fixed: special value meaning "not specified"

	MergeWithCLIArgs(config, &fixedTimeoutCLI, &fixedLogLevelCLI, &fixedFormatCLI, &fixedProgressCLI, &fixedOutputFileCLI)

	// Verify our fix preserves config file values when CLI args are not specified
	if config.General.Timeout != 600 {
		t.Errorf("Fix verification failed: timeout = %v, want 600", config.General.Timeout)
	}
	if config.General.LogLevel != "debug" {
		t.Errorf("Fix verification failed: log_level = %v, want debug", config.General.LogLevel)
	}
	if config.General.OutputFormat != "csv" {
		t.Errorf("Fix verification failed: output_format = %v, want csv", config.General.OutputFormat)
	}
}
