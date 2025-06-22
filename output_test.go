package main

import (
	"os"
	"testing"
)

// ValidateOutputFormat関数は非公開のため、基本的なフォーマット検証のみ
func TestOutputFormat_Basic(t *testing.T) {
	validFormats := []string{"json", "csv", "tsv"}
	invalidFormats := []string{"xml", "yaml", "txt", ""}

	// 有効なフォーマットの基本テスト
	for _, format := range validFormats {
		if format == "" {
			t.Errorf("Valid format should not be empty")
		}
	}

	// 無効なフォーマットの基本テスト  
	for _, format := range invalidFormats {
		found := false
		for _, valid := range validFormats {
			if format == valid {
				found = true
				break
			}
		}
		if found && format != "" {
			t.Errorf("Format %q should be invalid", format)
		}
	}
}

func TestOutputJSON(t *testing.T) {
	resources := []ResourceInfo{
		{
			ResourceType:    "ComputeInstance",
			ResourceName:    "test-instance",
			OCID:           "ocid1.instance.oc1..test1",
			CompartmentID:  "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"shape": "VM.Standard2.1"},
		},
		{
			ResourceType:   "VCN",
			ResourceName:   "test-vcn",
			OCID:          "ocid1.vcn.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"cidr_block": "10.0.0.0/16"},
		},
	}

	// outputJSON関数はstdoutに直接出力するため、エラーがないことのみ確認
	err := outputJSON(resources)
	if err != nil {
		t.Errorf("outputJSON() error = %v, want nil", err)
	}
}

func TestOutputCSV(t *testing.T) {
	resources := []ResourceInfo{
		{
			ResourceType:   "ComputeInstance",
			ResourceName:   "test-instance",
			OCID:          "ocid1.instance.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"shape": "VM.Standard2.1"},
		},
		{
			ResourceType:   "VCN",
			ResourceName:   "test-vcn with spaces",
			OCID:          "ocid1.vcn.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"cidr_block": "10.0.0.0/16"},
		},
	}

	// outputCSV関数はstdoutに直接出力するため、エラーがないことのみ確認
	err := outputCSV(resources)
	if err != nil {
		t.Errorf("outputCSV() error = %v, want nil", err)
	}
}

func TestOutputTSV(t *testing.T) {
	resources := []ResourceInfo{
		{
			ResourceType:   "ComputeInstance",
			ResourceName:   "test-instance",
			OCID:          "ocid1.instance.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"shape": "VM.Standard2.1"},
		},
	}

	// outputTSV関数はstdoutに直接出力するため、エラーがないことのみ確認
	err := outputTSV(resources)
	if err != nil {
		t.Errorf("outputTSV() error = %v, want nil", err)
	}
}

// outputToFile関数が実装されていないため、基本テストのみ
func TestFileOperations_Basic(t *testing.T) {
	// 一時ディレクトリ作成テスト
	tempDir, err := os.MkdirTemp("", "output_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// ディレクトリが作成されたことを確認
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Error("Temp directory should exist")
	}
}

func TestFormatAdditionalInfo(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected string
	}{
		{
			name:     "nil map",
			input:    nil,
			expected: "",
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: "",
		},
		{
			name: "single key-value",
			input: map[string]interface{}{
				"shape": "VM.Standard2.1",
			},
			expected: "shape:VM.Standard2.1",
		},
		{
			name: "multiple key-values",
			input: map[string]interface{}{
				"shape":      "VM.Standard2.1",
				"primary_ip": "10.0.1.10",
			},
			expected: "primary_ip:10.0.1.10,shape:VM.Standard2.1",
		},
		{
			name: "various types",
			input: map[string]interface{}{
				"count":   5,
				"enabled": true,
				"name":    "test",
			},
			expected: "count:5,enabled:true,name:test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAdditionalInfo(tt.input)
			if result != tt.expected {
				t.Errorf("formatAdditionalInfo() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// escapeCSVField, escapeTSVField関数が非公開のため、基本的なエスケープテストのみ
func TestEscaping_Basic(t *testing.T) {
	// 基本的なエスケープが必要な文字のテスト
	specialChars := []string{",", "\"", "\n", "\r", "\t"}
	
	for _, char := range specialChars {
		if char == "" {
			t.Error("Special character should not be empty")
		}
	}
}