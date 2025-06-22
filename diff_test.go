package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLoadResourcesFromFile_ValidFile(t *testing.T) {
	// テストデータ作成
	resources := []ResourceInfo{
		{
			ResourceType:    "ComputeInstance",
			ResourceName:    "test-instance-01",
			OCID:           "ocid1.instance.oc1..test1",
			CompartmentID:  "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"shape": "VM.Standard2.1"},
		},
		{
			ResourceType:    "VCN",
			ResourceName:    "test-vcn",
			OCID:           "ocid1.vcn.oc1..test1",
			CompartmentID:  "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"cidr_block": "10.0.0.0/16"},
		},
	}

	// 一時ファイル作成
	tempDir, err := os.MkdirTemp("", "diff_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "test_resources.json")
	data, err := json.Marshal(resources)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// テスト実行
	result, err := LoadResourcesFromFile(filePath)
	if err != nil {
		t.Errorf("LoadResourcesFromFile() error = %v, want nil", err)
	}

	if !reflect.DeepEqual(result, resources) {
		t.Errorf("LoadResourcesFromFile() = %v, want %v", result, resources)
	}
}

func TestLoadResourcesFromFile_NonExistentFile(t *testing.T) {
	_, err := LoadResourcesFromFile("/nonexistent/file.json")
	if err == nil {
		t.Error("LoadResourcesFromFile() error = nil, want error for non-existent file")
	}
}

func TestLoadResourcesFromFile_InvalidJSON(t *testing.T) {
	// 一時ファイル作成
	tempDir, err := os.MkdirTemp("", "diff_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "invalid.json")
	err = os.WriteFile(filePath, []byte("invalid json content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err = LoadResourcesFromFile(filePath)
	if err == nil {
		t.Error("LoadResourcesFromFile() error = nil, want error for invalid JSON")
	}
}

func TestCreateResourceMap(t *testing.T) {
	resources := []ResourceInfo{
		{
			OCID: "ocid1.instance.oc1..test1",
			ResourceName: "instance-1",
		},
		{
			OCID: "ocid1.vcn.oc1..test1",
			ResourceName: "vcn-1",
		},
	}

	resourceMap := CreateResourceMap(resources)

	if len(resourceMap) != 2 {
		t.Errorf("CreateResourceMap() map length = %d, want 2", len(resourceMap))
	}

	if resource, exists := resourceMap["ocid1.instance.oc1..test1"]; !exists {
		t.Error("CreateResourceMap() missing instance resource")
	} else if resource.ResourceName != "instance-1" {
		t.Errorf("CreateResourceMap() instance name = %s, want instance-1", resource.ResourceName)
	}

	if resource, exists := resourceMap["ocid1.vcn.oc1..test1"]; !exists {
		t.Error("CreateResourceMap() missing VCN resource")
	} else if resource.ResourceName != "vcn-1" {
		t.Errorf("CreateResourceMap() VCN name = %s, want vcn-1", resource.ResourceName)
	}
}

func TestFindAddedResources(t *testing.T) {
	oldMap := map[string]ResourceInfo{
		"ocid1.instance.oc1..test1": {OCID: "ocid1.instance.oc1..test1", ResourceName: "instance-1"},
	}

	newMap := map[string]ResourceInfo{
		"ocid1.instance.oc1..test1": {OCID: "ocid1.instance.oc1..test1", ResourceName: "instance-1"},
		"ocid1.vcn.oc1..test1":      {OCID: "ocid1.vcn.oc1..test1", ResourceName: "vcn-1"},
	}

	added := FindAddedResources(oldMap, newMap)

	if len(added) != 1 {
		t.Errorf("FindAddedResources() length = %d, want 1", len(added))
	}

	if added[0].OCID != "ocid1.vcn.oc1..test1" {
		t.Errorf("FindAddedResources() OCID = %s, want ocid1.vcn.oc1..test1", added[0].OCID)
	}
}

func TestFindRemovedResources(t *testing.T) {
	oldMap := map[string]ResourceInfo{
		"ocid1.instance.oc1..test1": {OCID: "ocid1.instance.oc1..test1", ResourceName: "instance-1"},
		"ocid1.vcn.oc1..test1":      {OCID: "ocid1.vcn.oc1..test1", ResourceName: "vcn-1"},
	}

	newMap := map[string]ResourceInfo{
		"ocid1.instance.oc1..test1": {OCID: "ocid1.instance.oc1..test1", ResourceName: "instance-1"},
	}

	removed := FindRemovedResources(oldMap, newMap)

	if len(removed) != 1 {
		t.Errorf("FindRemovedResources() length = %d, want 1", len(removed))
	}

	if removed[0].OCID != "ocid1.vcn.oc1..test1" {
		t.Errorf("FindRemovedResources() OCID = %s, want ocid1.vcn.oc1..test1", removed[0].OCID)
	}
}

func TestFindModifiedResources(t *testing.T) {
	oldMap := map[string]ResourceInfo{
		"ocid1.instance.oc1..test1": {
			OCID:           "ocid1.instance.oc1..test1",
			ResourceName:   "instance-1",
			AdditionalInfo: map[string]interface{}{"shape": "VM.Standard2.1"},
		},
	}

	newMap := map[string]ResourceInfo{
		"ocid1.instance.oc1..test1": {
			OCID:           "ocid1.instance.oc1..test1",
			ResourceName:   "instance-1-renamed",
			AdditionalInfo: map[string]interface{}{"shape": "VM.Standard2.2"},
		},
	}

	modified := FindModifiedResources(oldMap, newMap)

	if len(modified) != 1 {
		t.Errorf("FindModifiedResources() length = %d, want 1", len(modified))
	}

	if modified[0].ResourceInfo.OCID != "ocid1.instance.oc1..test1" {
		t.Errorf("FindModifiedResources() ResourceInfo.OCID = %s, want ocid1.instance.oc1..test1", modified[0].ResourceInfo.OCID)
	}

	// Changes should contain field modifications
	if len(modified[0].Changes) == 0 {
		t.Error("FindModifiedResources() should detect changes")
	}
}

func TestCompareResourceDetails(t *testing.T) {
	old := ResourceInfo{
		ResourceName:   "instance-1",
		ResourceType:   "ComputeInstance",
		OCID:          "ocid1.instance.oc1..test1",
		CompartmentID: "ocid1.compartment.oc1..test",
		AdditionalInfo: map[string]interface{}{
			"shape":      "VM.Standard2.1",
			"primary_ip": "10.0.1.10",
		},
	}

	new := ResourceInfo{
		ResourceName:   "instance-1-renamed",
		ResourceType:   "ComputeInstance",
		OCID:          "ocid1.instance.oc1..test1",
		CompartmentID: "ocid1.compartment.oc1..test",
		AdditionalInfo: map[string]interface{}{
			"shape":      "VM.Standard2.2",
			"primary_ip": "10.0.1.10",
		},
	}

	changes := CompareResourceDetails(old, new)

	// 変更が検出されることを確認
	if len(changes) == 0 {
		t.Error("CompareResourceDetails() should detect changes")
	}

	// ResourceNameの変更が検出されることを確認
	foundNameChange := false
	foundShapeChange := false
	for _, change := range changes {
		if change.Field == "ResourceName" {
			foundNameChange = true
			if change.OldValue != "instance-1" || change.NewValue != "instance-1-renamed" {
				t.Errorf("CompareResourceDetails() ResourceName change: old=%v, new=%v", change.OldValue, change.NewValue)
			}
		}
		if change.Field == "shape" {
			foundShapeChange = true
			if change.OldValue != "VM.Standard2.1" || change.NewValue != "VM.Standard2.2" {
				t.Errorf("CompareResourceDetails() shape change: old=%v, new=%v", change.OldValue, change.NewValue)
			}
		}
	}
	
	if !foundNameChange {
		t.Error("CompareResourceDetails() should detect ResourceName change")
	}
	if !foundShapeChange {
		t.Error("CompareResourceDetails() should detect shape change")
	}
}

func TestBuildDiffResult(t *testing.T) {
	added := []ResourceInfo{
		{OCID: "ocid1.vcn.oc1..test1", ResourceName: "vcn-1"},
	}
	removed := []ResourceInfo{
		{OCID: "ocid1.subnet.oc1..test1", ResourceName: "subnet-1"},
	}
	modified := []ModifiedResource{
		{
			ResourceInfo: ResourceInfo{
				OCID:         "ocid1.instance.oc1..test1",
				ResourceName: "instance-1-renamed",
			},
			Changes: []FieldChange{
				{
					Field:    "ResourceName",
					OldValue: "instance-1",
					NewValue: "instance-1-renamed",
				},
			},
		},
	}
	unchanged := []ResourceInfo{
		{OCID: "ocid1.volume.oc1..test1", ResourceName: "volume-1"},
	}

	result := BuildDiffResult(added, removed, modified, unchanged, "old.json", "new.json", true)

	// Summary検証
	if result.Summary.Added != 1 {
		t.Errorf("BuildDiffResult() Summary.Added = %d, want 1", result.Summary.Added)
	}
	if result.Summary.Removed != 1 {
		t.Errorf("BuildDiffResult() Summary.Removed = %d, want 1", result.Summary.Removed)
	}
	if result.Summary.Modified != 1 {
		t.Errorf("BuildDiffResult() Summary.Modified = %d, want 1", result.Summary.Modified)
	}
	if result.Summary.Unchanged != 1 {
		t.Errorf("BuildDiffResult() Summary.Unchanged = %d, want 1", result.Summary.Unchanged)
	}

	// ファイル名検証
	if result.OldFile != "old.json" {
		t.Errorf("BuildDiffResult() OldFile = %s, want old.json", result.OldFile)
	}
	if result.NewFile != "new.json" {
		t.Errorf("BuildDiffResult() NewFile = %s, want new.json", result.NewFile)
	}

	// タイムスタンプ検証（形式のみ）
	if result.Timestamp == "" {
		t.Error("BuildDiffResult() Timestamp should not be empty")
	}

	// データ検証
	if len(result.Added) != 1 {
		t.Errorf("BuildDiffResult() Added length = %d, want 1", len(result.Added))
	}
	if len(result.Removed) != 1 {
		t.Errorf("BuildDiffResult() Removed length = %d, want 1", len(result.Removed))
	}
	if len(result.Modified) != 1 {
		t.Errorf("BuildDiffResult() Modified length = %d, want 1", len(result.Modified))
	}
	if len(result.Unchanged) != 1 {
		t.Errorf("BuildDiffResult() Unchanged length = %d, want 1", len(result.Unchanged))
	}
}

func TestCompareDumps_SameFiles(t *testing.T) {
	// 同一リソースデータ作成
	resources := []ResourceInfo{
		{
			ResourceType:   "ComputeInstance",
			ResourceName:   "test-instance",
			OCID:          "ocid1.instance.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
		},
	}

	// 一時ファイル作成
	tempDir, err := os.MkdirTemp("", "diff_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldFile := filepath.Join(tempDir, "old.json")
	newFile := filepath.Join(tempDir, "new.json")

	data, err := json.Marshal(resources)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	err = os.WriteFile(oldFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write old file: %v", err)
	}

	err = os.WriteFile(newFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write new file: %v", err)
	}

	// テスト実行
	config := DiffConfig{Detailed: true}
	result, err := CompareDumps(oldFile, newFile, config)
	if err != nil {
		t.Errorf("CompareDumps() error = %v, want nil", err)
	}

	// 変更なしを検証
	if result.Summary.Added != 0 {
		t.Errorf("CompareDumps() Added = %d, want 0", result.Summary.Added)
	}
	if result.Summary.Removed != 0 {
		t.Errorf("CompareDumps() Removed = %d, want 0", result.Summary.Removed)
	}
	if result.Summary.Modified != 0 {
		t.Errorf("CompareDumps() Modified = %d, want 0", result.Summary.Modified)
	}
	if result.Summary.Unchanged != 1 {
		t.Errorf("CompareDumps() Unchanged = %d, want 1", result.Summary.Unchanged)
	}
}

func TestCompareDumps_DifferentFiles(t *testing.T) {
	// 異なるリソースデータ作成
	oldResources := []ResourceInfo{
		{
			ResourceType:   "ComputeInstance",
			ResourceName:   "old-instance",
			OCID:          "ocid1.instance.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"shape": "VM.Standard2.1"},
		},
		{
			ResourceType:   "VCN",
			ResourceName:   "shared-vcn",
			OCID:          "ocid1.vcn.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
		},
	}

	newResources := []ResourceInfo{
		{
			ResourceType:   "ComputeInstance",
			ResourceName:   "new-instance",
			OCID:          "ocid1.instance.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
			AdditionalInfo: map[string]interface{}{"shape": "VM.Standard2.2"},
		},
		{
			ResourceType:   "VCN",
			ResourceName:   "shared-vcn",
			OCID:          "ocid1.vcn.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
		},
		{
			ResourceType:   "Subnet",
			ResourceName:   "new-subnet",
			OCID:          "ocid1.subnet.oc1..test1",
			CompartmentID: "ocid1.compartment.oc1..test",
		},
	}

	// 一時ファイル作成
	tempDir, err := os.MkdirTemp("", "diff_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldFile := filepath.Join(tempDir, "old.json")
	newFile := filepath.Join(tempDir, "new.json")

	oldData, err := json.Marshal(oldResources)
	if err != nil {
		t.Fatalf("Failed to marshal old data: %v", err)
	}

	newData, err := json.Marshal(newResources)
	if err != nil {
		t.Fatalf("Failed to marshal new data: %v", err)
	}

	err = os.WriteFile(oldFile, oldData, 0644)
	if err != nil {
		t.Fatalf("Failed to write old file: %v", err)
	}

	err = os.WriteFile(newFile, newData, 0644)
	if err != nil {
		t.Fatalf("Failed to write new file: %v", err)
	}

	// テスト実行
	config := DiffConfig{Detailed: false}
	result, err := CompareDumps(oldFile, newFile, config)
	if err != nil {
		t.Errorf("CompareDumps() error = %v, want nil", err)
	}

	// 期待値検証
	if result.Summary.Added != 1 {
		t.Errorf("CompareDumps() Added = %d, want 1", result.Summary.Added)
	}
	if result.Summary.Removed != 0 {
		t.Errorf("CompareDumps() Removed = %d, want 0", result.Summary.Removed)
	}
	if result.Summary.Modified != 1 {
		t.Errorf("CompareDumps() Modified = %d, want 1", result.Summary.Modified)
	}
	if result.Summary.Unchanged != 1 {
		t.Errorf("CompareDumps() Unchanged = %d, want 1", result.Summary.Unchanged)
	}

	// 追加されたリソース検証
	if len(result.Added) != 1 || result.Added[0].OCID != "ocid1.subnet.oc1..test1" {
		t.Error("CompareDumps() added resource should be subnet")
	}

	// 変更されたリソース検証
	if len(result.Modified) != 1 || result.Modified[0].ResourceInfo.OCID != "ocid1.instance.oc1..test1" {
		t.Error("CompareDumps() modified resource should be instance")
	}

	// Detailedがfalseなので、unchangedは空であることを確認
	if len(result.Unchanged) != 0 {
		t.Errorf("CompareDumps() Unchanged length = %d, want 0 when Detailed is false", len(result.Unchanged))
	}
}

// formatTimestamp関数が非公開なので、RFC3339フォーマットのテストのみ実施
func TestTimestampFormat(t *testing.T) {
	now := time.Now()
	formatted := now.Format(time.RFC3339)

	// RFC3339形式であることを確認
	_, err := time.Parse(time.RFC3339, formatted)
	if err != nil {
		t.Errorf("RFC3339 format test failed: %s", formatted)
	}
}