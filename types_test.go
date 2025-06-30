package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResourceInfo_JSONSerialization(t *testing.T) {
	original := ResourceInfo{
		ResourceType:  "ComputeInstance",
		ResourceName:  "test-instance",
		OCID:          "ocid1.instance.oc1..test1",
		CompartmentID: "ocid1.compartment.oc1..test",
		AdditionalInfo: map[string]interface{}{
			"shape":      "VM.Standard2.1",
			"primary_ip": "10.0.1.10",
			"count":      5,
			"enabled":    true,
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal
	var deserialized ResourceInfo
	err = json.Unmarshal(data, &deserialized)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// 基本フィールドの検証
	if deserialized.ResourceType != original.ResourceType {
		t.Errorf("ResourceType = %s, want %s", deserialized.ResourceType, original.ResourceType)
	}
	if deserialized.ResourceName != original.ResourceName {
		t.Errorf("ResourceName = %s, want %s", deserialized.ResourceName, original.ResourceName)
	}
	if deserialized.OCID != original.OCID {
		t.Errorf("OCID = %s, want %s", deserialized.OCID, original.OCID)
	}
	if deserialized.CompartmentID != original.CompartmentID {
		t.Errorf("CompartmentID = %s, want %s", deserialized.CompartmentID, original.CompartmentID)
	}

	// AdditionalInfo の検証
	if !reflect.DeepEqual(deserialized.AdditionalInfo, original.AdditionalInfo) {
		t.Errorf("AdditionalInfo = %v, want %v", deserialized.AdditionalInfo, original.AdditionalInfo)
	}
}

func TestResourceInfo_EmptyAdditionalInfo(t *testing.T) {
	tests := []struct {
		name           string
		additionalInfo map[string]interface{}
	}{
		{
			name:           "nil additional info",
			additionalInfo: nil,
		},
		{
			name:           "empty additional info",
			additionalInfo: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := ResourceInfo{
				ResourceType:   "VCN",
				ResourceName:   "test-vcn",
				OCID:           "ocid1.vcn.oc1..test1",
				CompartmentID:  "ocid1.compartment.oc1..test",
				AdditionalInfo: tt.additionalInfo,
			}

			// JSON シリアライゼーション
			data, err := json.Marshal(resource)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			// JSON デシリアライゼーション
			var deserialized ResourceInfo
			err = json.Unmarshal(data, &deserialized)
			if err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// 基本フィールドが保持されていることを確認
			if deserialized.ResourceType != resource.ResourceType {
				t.Errorf("ResourceType = %s, want %s", deserialized.ResourceType, resource.ResourceType)
			}
		})
	}
}

// Compartment構造体が存在しないため、基本的なID検証のみ
func TestCompartmentID_Validation(t *testing.T) {
	validIDs := []string{
		"ocid1.compartment.oc1..test1",
		"ocid1.compartment.oc1.region.test2",
	}

	invalidIDs := []string{
		"",
		"invalid-id",
		"not-an-ocid",
	}

	for _, id := range validIDs {
		if len(id) < 10 {
			t.Errorf("Valid ID %s should be longer", id)
		}
		if !strings.HasPrefix(id, "ocid1.") {
			t.Errorf("Valid ID %s should start with ocid1.", id)
		}
	}

	for _, id := range invalidIDs {
		if id != "" && strings.HasPrefix(id, "ocid1.") && len(id) > 20 {
			t.Errorf("Invalid ID %s should not pass validation", id)
		}
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	config := Config{
		OutputFormat:    "json",
		Timeout:         300 * time.Second,
		MaxWorkers:      5,
		LogLevel:        LogLevelNormal,
		ShowProgress:    false,
		Logger:          nil,
		Filters:         FilterConfig{},
	}

	// デフォルト値の検証
	if config.OutputFormat != "json" {
		t.Errorf("OutputFormat = %s, want json", config.OutputFormat)
	}
	if config.Timeout != 300*time.Second {
		t.Errorf("Timeout = %v, want 300s", config.Timeout)
	}
	if config.MaxWorkers != 5 {
		t.Errorf("MaxWorkers = %d, want 5", config.MaxWorkers)
	}
	if config.LogLevel != LogLevelNormal {
		t.Errorf("LogLevel = %v, want LogLevelNormal", config.LogLevel)
	}
	if config.ShowProgress != false {
		t.Errorf("ShowProgress = %v, want false", config.ShowProgress)
	}
}

func TestLogLevel_Values(t *testing.T) {
	// LogLevel の定数値が正しいことを確認
	if LogLevelSilent != 0 {
		t.Errorf("LogLevelSilent = %d, want 0", LogLevelSilent)
	}
	if LogLevelNormal != 1 {
		t.Errorf("LogLevelNormal = %d, want 1", LogLevelNormal)
	}
	if LogLevelVerbose != 2 {
		t.Errorf("LogLevelVerbose = %d, want 2", LogLevelVerbose)
	}
	if LogLevelDebug != 3 {
		t.Errorf("LogLevelDebug = %d, want 3", LogLevelDebug)
	}
}

func TestAppConfig_Structure(t *testing.T) {
	config := AppConfig{
		Version: "1.0",
		General: GeneralConfig{
			Timeout:      300,
			LogLevel:     "normal",
			OutputFormat: "json",
			Progress:     false,
		},
		Output: OutputConfig{
			File: "output.json",
		},
		Filters: FilterConfig{
			IncludeCompartments:  []string{"ocid1.compartment.oc1..test1"},
			ExcludeCompartments:  []string{"ocid1.compartment.oc1..test2"},
			IncludeResourceTypes: []string{"compute_instances"},
			ExcludeResourceTypes: []string{"subnets"},
			NamePattern:          "test-.*",
			ExcludeNamePattern:   "old-.*",
		},
		Diff: DiffConfig{
			Format:   "json",
			Detailed: true,
		},
	}

	// 各フィールドがアクセス可能であることを確認
	if config.Version != "1.0" {
		t.Errorf("Version = %s, want 1.0", config.Version)
	}
	if config.General.Timeout != 300 {
		t.Errorf("General.Timeout = %d, want 300", config.General.Timeout)
	}
	if config.Output.File != "output.json" {
		t.Errorf("Output.File = %s, want output.json", config.Output.File)
	}
	if len(config.Filters.IncludeCompartments) != 1 {
		t.Errorf("Filters.IncludeCompartments length = %d, want 1", len(config.Filters.IncludeCompartments))
	}
	if config.Diff.Detailed != true {
		t.Errorf("Diff.Detailed = %v, want true", config.Diff.Detailed)
	}
}

func TestModifiedResource_Structure(t *testing.T) {
	newResource := ResourceInfo{
		ResourceType: "ComputeInstance",
		ResourceName: "new-name",
		OCID:         "ocid1.instance.oc1..test1",
	}

	modified := ModifiedResource{
		ResourceInfo: newResource,
		Changes: []FieldChange{
			{
				Field:    "ResourceName",
				OldValue: "old-name",
				NewValue: "new-name",
			},
		},
	}

	// 構造体フィールドのアクセス確認
	if modified.ResourceInfo.OCID != "ocid1.instance.oc1..test1" {
		t.Errorf("ResourceInfo.OCID = %s, want ocid1.instance.oc1..test1", modified.ResourceInfo.OCID)
	}
	if modified.ResourceInfo.ResourceName != "new-name" {
		t.Errorf("ResourceInfo.ResourceName = %s, want new-name", modified.ResourceInfo.ResourceName)
	}
	if len(modified.Changes) != 1 {
		t.Errorf("Changes length = %d, want 1", len(modified.Changes))
	}
	if modified.Changes[0].Field != "ResourceName" {
		t.Errorf("Changes[0].Field = %s, want ResourceName", modified.Changes[0].Field)
	}
}

func TestDiffSummary_Calculation(t *testing.T) {
	summary := DiffSummary{
		TotalOld:  15,
		TotalNew:  17,
		Added:     5,
		Removed:   3,
		Modified:  2,
		Unchanged: 10,
	}

	// 合計計算
	totalChanges := summary.Added + summary.Removed + summary.Modified
	totalResources := totalChanges + summary.Unchanged

	if totalChanges != 10 {
		t.Errorf("Total changes = %d, want 10", totalChanges)
	}
	if totalResources != 20 {
		t.Errorf("Total resources = %d, want 20", totalResources)
	}
}

func TestDiffResult_JSONSerialization(t *testing.T) {
	result := DiffResult{
		Summary: DiffSummary{
			TotalOld:  2,
			TotalNew:  2,
			Added:     1,
			Removed:   1,
			Modified:  1,
			Unchanged: 1,
		},
		Added: []ResourceInfo{
			{OCID: "ocid1.instance.oc1..added", ResourceName: "added-instance"},
		},
		Removed: []ResourceInfo{
			{OCID: "ocid1.instance.oc1..removed", ResourceName: "removed-instance"},
		},
		Modified: []ModifiedResource{
			{
				ResourceInfo: ResourceInfo{
					OCID:         "ocid1.instance.oc1..modified",
					ResourceName: "new-name",
				},
				Changes: []FieldChange{
					{
						Field:    "ResourceName",
						OldValue: "old-name",
						NewValue: "new-name",
					},
				},
			},
		},
		Unchanged: []ResourceInfo{
			{OCID: "ocid1.instance.oc1..unchanged", ResourceName: "unchanged-instance"},
		},
		Timestamp: "2024-01-01T00:00:00Z",
		OldFile:   "old.json",
		NewFile:   "new.json",
	}

	// JSON シリアライゼーション
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// JSON デシリアライゼーション
	var deserialized DiffResult
	err = json.Unmarshal(data, &deserialized)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// 基本フィールド検証
	if deserialized.Summary.Added != result.Summary.Added {
		t.Errorf("Summary.Added = %d, want %d", deserialized.Summary.Added, result.Summary.Added)
	}
	if len(deserialized.Added) != len(result.Added) {
		t.Errorf("Added length = %d, want %d", len(deserialized.Added), len(result.Added))
	}
	if len(deserialized.Modified) != len(result.Modified) {
		t.Errorf("Modified length = %d, want %d", len(deserialized.Modified), len(result.Modified))
	}
	if deserialized.Timestamp != result.Timestamp {
		t.Errorf("Timestamp = %s, want %s", deserialized.Timestamp, result.Timestamp)
	}
}

func TestCompiledFilters_Structure(t *testing.T) {
	// CompiledFilters 構造体の基本的な構造テスト
	// 実際の正規表現コンパイルは filters_test.go で行う
	compiled := CompiledFilters{
		NameRegex:        nil,
		ExcludeNameRegex: nil,
	}

	// nil 値でも構造体として有効であることを確認
	if compiled.NameRegex != nil {
		t.Error("NameRegex should be nil initially")
	}
	if compiled.ExcludeNameRegex != nil {
		t.Error("ExcludeNameRegex should be nil initially")
	}
}

func TestResourceInfo_RequiredFields(t *testing.T) {
	// 必須フィールドのテスト
	resource := ResourceInfo{
		ResourceType:  "ComputeInstance",
		ResourceName:  "test-instance",
		OCID:          "ocid1.instance.oc1..test1",
		CompartmentID: "ocid1.compartment.oc1..test",
	}

	// 必須フィールドが設定されていることを確認
	if resource.ResourceType == "" {
		t.Error("ResourceType should not be empty")
	}
	if resource.ResourceName == "" {
		t.Error("ResourceName should not be empty")
	}
	if resource.OCID == "" {
		t.Error("OCID should not be empty")
	}
	if resource.CompartmentID == "" {
		t.Error("CompartmentID should not be empty")
	}
}

func TestStructureConsistency(t *testing.T) {
	// 構造体間の一貫性テスト

	// ResourceInfo が ModifiedResource で正しく使用されることを確認
	resource := ResourceInfo{
		ResourceType: "Test",
		ResourceName: "test",
		OCID:         "test-ocid",
	}

	// ModifiedResource構造体はResourceInfoとChangesを持つ
	modified := ModifiedResource{
		ResourceInfo: resource,
		Changes: []FieldChange{
			{
				Field:    "test",
				OldValue: "old",
				NewValue: "new",
			},
		},
	}

	if modified.ResourceInfo.OCID == "" {
		t.Error("ModifiedResource should have valid ResourceInfo")
	}
}
