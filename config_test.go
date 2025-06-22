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
	if config.General.Progress != false {
		t.Errorf("getDefaultConfig() General.Progress = %v, want false", config.General.Progress)
	}

	// Output設定チェック
	if config.Output.File != "" {
		t.Errorf("getDefaultConfig() Output.File = %v, want empty string", config.Output.File)
	}

	// Filters設定チェック
	if config.Filters.IncludeCompartments != nil {
		t.Errorf("getDefaultConfig() Filters.IncludeCompartments = %v, want nil", config.Filters.IncludeCompartments)
	}
	if config.Filters.ExcludeCompartments != nil {
		t.Errorf("getDefaultConfig() Filters.ExcludeCompartments = %v, want nil", config.Filters.ExcludeCompartments)
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
		name           string
		config         *AppConfig
		timeout        *int
		logLevel       *string
		outputFormat   *string
		progress       *bool
		outputFile     *string
		expectedFormat string
		expectedTimeout int
		expectedLogLevel string
		expectedProgress bool
		expectedFile    string
	}{
		{
			name:           "override format",
			config:         getDefaultConfig(),
			timeout:        nil,
			logLevel:       nil,
			outputFormat:   stringPtr("csv"),
			progress:       nil,
			outputFile:     nil,
			expectedFormat: "csv",
			expectedTimeout: 300,
			expectedLogLevel: "normal",
			expectedProgress: true,
			expectedFile:    "",
		},
		{
			name:           "override timeout",
			config:         getDefaultConfig(),
			timeout:        intPtr(600),
			logLevel:       nil,
			outputFormat:   nil,
			progress:       nil,
			outputFile:     nil,
			expectedFormat: "json",
			expectedTimeout: 600,
			expectedLogLevel: "normal",
			expectedProgress: true,
			expectedFile:    "",
		},
		{
			name:           "override all",
			config:         getDefaultConfig(),
			timeout:        intPtr(120),
			logLevel:       stringPtr("debug"),
			outputFormat:   stringPtr("tsv"),
			progress:       boolPtr(false),
			outputFile:     stringPtr("output.json"),
			expectedFormat: "tsv",
			expectedTimeout: 120,
			expectedLogLevel: "debug",
			expectedProgress: false,
			expectedFile:    "output.json",
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
func intPtr(i int) *int { return &i }
func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool { return &b }

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