package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"
)

// DiffConfig represents the diff analysis configuration
type DiffConfig struct {
	Format     string `yaml:"format"`      // "json" or "text"
	Detailed   bool   `yaml:"detailed"`    // include unchanged resources
	OutputFile string `yaml:"output_file"` // output file path
}

// DiffResult represents the comparison result between two resource dumps
type DiffResult struct {
	Summary   DiffSummary        `json:"summary"`
	Added     []ResourceInfo     `json:"added"`
	Removed   []ResourceInfo     `json:"removed"`
	Modified  []ModifiedResource `json:"modified"`
	Unchanged []ResourceInfo     `json:"unchanged,omitempty"`
	Timestamp string             `json:"timestamp"`
	OldFile   string             `json:"old_file"`
	NewFile   string             `json:"new_file"`
}

// DiffSummary provides statistical overview of the differences
type DiffSummary struct {
	TotalOld       int                    `json:"total_old"`
	TotalNew       int                    `json:"total_new"`
	Added          int                    `json:"added"`
	Removed        int                    `json:"removed"`
	Modified       int                    `json:"modified"`
	Unchanged      int                    `json:"unchanged"`
	ByResourceType map[string]DiffStats   `json:"by_resource_type"`
}

// DiffStats holds statistics for a specific resource type
type DiffStats struct {
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Modified  int `json:"modified"`
	Unchanged int `json:"unchanged"`
}

// ModifiedResource represents a resource that has been changed
type ModifiedResource struct {
	ResourceInfo ResourceInfo  `json:"resource_info"`
	Changes      []FieldChange `json:"changes"`
}

// FieldChange represents a specific field modification
type FieldChange struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// CompareDumps performs diff analysis between two JSON dump files
func CompareDumps(oldFile, newFile string, config DiffConfig) (*DiffResult, error) {
	logger.Info("Starting diff analysis: %s vs %s", oldFile, newFile)
	
	// Validate input files
	if err := validateDiffFiles(oldFile, newFile); err != nil {
		return nil, err
	}
	
	// Load resources from both files
	oldResources, err := LoadResourcesFromFile(oldFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load old file %s: %w", oldFile, err)
	}
	
	newResources, err := LoadResourcesFromFile(newFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load new file %s: %w", newFile, err)
	}
	
	logger.Verbose("Loaded %d resources from old file, %d from new file", len(oldResources), len(newResources))
	
	// Create resource maps for efficient comparison
	oldMap := CreateResourceMap(oldResources)
	newMap := CreateResourceMap(newResources)
	
	// Perform diff analysis
	added := FindAddedResources(oldMap, newMap)
	removed := FindRemovedResources(oldMap, newMap)
	modified := FindModifiedResources(oldMap, newMap)
	unchanged := FindUnchangedResources(oldMap, newMap)
	
	// Build result
	result := BuildDiffResult(added, removed, modified, unchanged, oldFile, newFile, config.Detailed)
	
	logger.Info("Diff analysis complete: +%d, -%d, ~%d resources", len(added), len(removed), len(modified))
	return result, nil
}

// LoadResourcesFromFile loads ResourceInfo array from a JSON file
func LoadResourcesFromFile(filename string) ([]ResourceInfo, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	var resources []ResourceInfo
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&resources); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}
	
	return resources, nil
}

// CreateResourceMap creates a map with OCID as key for efficient lookups
func CreateResourceMap(resources []ResourceInfo) map[string]ResourceInfo {
	resourceMap := make(map[string]ResourceInfo, len(resources))
	for _, resource := range resources {
		if resource.OCID != "" {
			resourceMap[resource.OCID] = resource
		} else {
			// Fallback key for resources without OCID
			fallbackKey := fmt.Sprintf("%s:%s:%s", resource.CompartmentID, resource.ResourceType, resource.ResourceName)
			resourceMap[fallbackKey] = resource
		}
	}
	return resourceMap
}

// FindAddedResources identifies resources present in new but not in old
func FindAddedResources(oldMap, newMap map[string]ResourceInfo) []ResourceInfo {
	var added []ResourceInfo
	for ocid, resource := range newMap {
		if _, exists := oldMap[ocid]; !exists {
			added = append(added, resource)
		}
	}
	
	// Sort for consistent output
	sort.Slice(added, func(i, j int) bool {
		if added[i].ResourceType != added[j].ResourceType {
			return added[i].ResourceType < added[j].ResourceType
		}
		return added[i].ResourceName < added[j].ResourceName
	})
	
	return added
}

// FindRemovedResources identifies resources present in old but not in new
func FindRemovedResources(oldMap, newMap map[string]ResourceInfo) []ResourceInfo {
	var removed []ResourceInfo
	for ocid, resource := range oldMap {
		if _, exists := newMap[ocid]; !exists {
			removed = append(removed, resource)
		}
	}
	
	// Sort for consistent output
	sort.Slice(removed, func(i, j int) bool {
		if removed[i].ResourceType != removed[j].ResourceType {
			return removed[i].ResourceType < removed[j].ResourceType
		}
		return removed[i].ResourceName < removed[j].ResourceName
	})
	
	return removed
}

// FindModifiedResources identifies resources that exist in both but with differences
func FindModifiedResources(oldMap, newMap map[string]ResourceInfo) []ModifiedResource {
	var modified []ModifiedResource
	
	for ocid, oldResource := range oldMap {
		if newResource, exists := newMap[ocid]; exists {
			changes := CompareResourceDetails(oldResource, newResource)
			if len(changes) > 0 {
				modified = append(modified, ModifiedResource{
					ResourceInfo: newResource,
					Changes:      changes,
				})
			}
		}
	}
	
	// Sort for consistent output
	sort.Slice(modified, func(i, j int) bool {
		if modified[i].ResourceInfo.ResourceType != modified[j].ResourceInfo.ResourceType {
			return modified[i].ResourceInfo.ResourceType < modified[j].ResourceInfo.ResourceType
		}
		return modified[i].ResourceInfo.ResourceName < modified[j].ResourceInfo.ResourceName
	})
	
	return modified
}

// FindUnchangedResources identifies resources that are identical in both dumps
func FindUnchangedResources(oldMap, newMap map[string]ResourceInfo) []ResourceInfo {
	var unchanged []ResourceInfo
	
	for ocid, oldResource := range oldMap {
		if newResource, exists := newMap[ocid]; exists {
			changes := CompareResourceDetails(oldResource, newResource)
			if len(changes) == 0 {
				unchanged = append(unchanged, newResource)
			}
		}
	}
	
	// Sort for consistent output
	sort.Slice(unchanged, func(i, j int) bool {
		if unchanged[i].ResourceType != unchanged[j].ResourceType {
			return unchanged[i].ResourceType < unchanged[j].ResourceType
		}
		return unchanged[i].ResourceName < unchanged[j].ResourceName
	})
	
	return unchanged
}

// CompareResourceDetails compares two ResourceInfo objects and returns list of changes
func CompareResourceDetails(old, new ResourceInfo) []FieldChange {
	var changes []FieldChange
	
	// Compare basic fields
	if old.ResourceName != new.ResourceName {
		changes = append(changes, FieldChange{
			Field:    "ResourceName",
			OldValue: old.ResourceName,
			NewValue: new.ResourceName,
		})
	}
	
	if old.CompartmentID != new.CompartmentID {
		changes = append(changes, FieldChange{
			Field:    "CompartmentID",
			OldValue: old.CompartmentID,
			NewValue: new.CompartmentID,
		})
	}
	
	// Compare AdditionalInfo maps
	changes = append(changes, compareAdditionalInfo(old.AdditionalInfo, new.AdditionalInfo)...)
	
	return changes
}

// compareAdditionalInfo compares two AdditionalInfo maps and returns field changes
func compareAdditionalInfo(oldInfo, newInfo map[string]interface{}) []FieldChange {
	var changes []FieldChange
	
	// Get all unique keys from both maps
	allKeys := getAllKeys(oldInfo, newInfo)
	
	for _, key := range allKeys {
		oldVal, oldExists := oldInfo[key]
		newVal, newExists := newInfo[key]
		
		if !oldExists && newExists {
			// Field added
			changes = append(changes, FieldChange{
				Field:    fmt.Sprintf("AdditionalInfo.%s", key),
				OldValue: nil,
				NewValue: newVal,
			})
		} else if oldExists && !newExists {
			// Field removed
			changes = append(changes, FieldChange{
				Field:    fmt.Sprintf("AdditionalInfo.%s", key),
				OldValue: oldVal,
				NewValue: nil,
			})
		} else if oldExists && newExists && !reflect.DeepEqual(oldVal, newVal) {
			// Field modified
			changes = append(changes, FieldChange{
				Field:    fmt.Sprintf("AdditionalInfo.%s", key),
				OldValue: oldVal,
				NewValue: newVal,
			})
		}
	}
	
	// Sort changes by field name for consistent output
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Field < changes[j].Field
	})
	
	return changes
}

// getAllKeys returns all unique keys from two maps
func getAllKeys(map1, map2 map[string]interface{}) []string {
	keySet := make(map[string]bool)
	
	for key := range map1 {
		keySet[key] = true
	}
	for key := range map2 {
		keySet[key] = true
	}
	
	var keys []string
	for key := range keySet {
		keys = append(keys, key)
	}
	
	sort.Strings(keys)
	return keys
}

// BuildDiffResult constructs the final DiffResult object
func BuildDiffResult(added, removed []ResourceInfo, modified []ModifiedResource, unchanged []ResourceInfo, oldFile, newFile string, includeUnchanged bool) *DiffResult {
	result := &DiffResult{
		Added:     added,
		Removed:   removed,
		Modified:  modified,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		OldFile:   oldFile,
		NewFile:   newFile,
	}
	
	if includeUnchanged {
		result.Unchanged = unchanged
	}
	
	// Build summary statistics
	result.Summary = DiffSummary{
		TotalOld:       len(removed) + len(modified) + len(unchanged),
		TotalNew:       len(added) + len(modified) + len(unchanged),
		Added:          len(added),
		Removed:        len(removed),
		Modified:       len(modified),
		Unchanged:      len(unchanged),
		ByResourceType: buildResourceTypeStats(added, removed, modified, unchanged),
	}
	
	return result
}

// buildResourceTypeStats creates per-resource-type statistics
func buildResourceTypeStats(added, removed []ResourceInfo, modified []ModifiedResource, unchanged []ResourceInfo) map[string]DiffStats {
	stats := make(map[string]DiffStats)
	
	// Count added resources
	for _, resource := range added {
		stat := stats[resource.ResourceType]
		stat.Added++
		stats[resource.ResourceType] = stat
	}
	
	// Count removed resources
	for _, resource := range removed {
		stat := stats[resource.ResourceType]
		stat.Removed++
		stats[resource.ResourceType] = stat
	}
	
	// Count modified resources
	for _, resource := range modified {
		stat := stats[resource.ResourceInfo.ResourceType]
		stat.Modified++
		stats[resource.ResourceInfo.ResourceType] = stat
	}
	
	// Count unchanged resources
	for _, resource := range unchanged {
		stat := stats[resource.ResourceType]
		stat.Unchanged++
		stats[resource.ResourceType] = stat
	}
	
	return stats
}

// OutputDiffResult outputs the diff result in the specified format
func OutputDiffResult(result *DiffResult, config DiffConfig) error {
	var writer io.Writer
	
	if config.OutputFile != "" {
		file, err := os.Create(config.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file %s: %w", config.OutputFile, err)
		}
		defer file.Close()
		writer = file
		logger.Info("Writing diff result to file: %s", config.OutputFile)
	} else {
		writer = os.Stdout
	}
	
	switch strings.ToLower(config.Format) {
	case "json":
		return OutputDiffJSON(result, writer)
	case "text":
		return OutputDiffText(result, writer)
	default:
		return fmt.Errorf("unsupported diff format: %s", config.Format)
	}
}

// OutputDiffJSON outputs the diff result in JSON format
func OutputDiffJSON(result *DiffResult, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// OutputDiffText outputs the diff result in human-readable text format
func OutputDiffText(result *DiffResult, writer io.Writer) error {
	fmt.Fprintf(writer, "OCI Resource Dump Comparison Report\n")
	fmt.Fprintf(writer, "===================================\n\n")
	
	fmt.Fprintf(writer, "Files Compared:\n")
	fmt.Fprintf(writer, "  Old: %s (%d resources)\n", result.OldFile, result.Summary.TotalOld)
	fmt.Fprintf(writer, "  New: %s (%d resources)\n", result.NewFile, result.Summary.TotalNew)
	fmt.Fprintf(writer, "\nGenerated: %s\n\n", result.Timestamp)
	
	// Summary section
	fmt.Fprintf(writer, "SUMMARY\n")
	fmt.Fprintf(writer, "-------\n")
	totalChanges := result.Summary.Added + result.Summary.Removed + result.Summary.Modified
	fmt.Fprintf(writer, "Total Changes: %d resources affected\n", totalChanges)
	fmt.Fprintf(writer, "  Added:     %d resources\n", result.Summary.Added)
	fmt.Fprintf(writer, "  Removed:   %d resources\n", result.Summary.Removed)
	fmt.Fprintf(writer, "  Modified:  %d resources\n", result.Summary.Modified)
	fmt.Fprintf(writer, "  Unchanged: %d resources\n\n", result.Summary.Unchanged)
	
	// Resource type breakdown
	if len(result.Summary.ByResourceType) > 0 {
		fmt.Fprintf(writer, "CHANGES BY RESOURCE TYPE\n")
		fmt.Fprintf(writer, "------------------------\n")
		
		var resourceTypes []string
		for resourceType := range result.Summary.ByResourceType {
			resourceTypes = append(resourceTypes, resourceType)
		}
		sort.Strings(resourceTypes)
		
		for _, resourceType := range resourceTypes {
			stats := result.Summary.ByResourceType[resourceType]
			total := stats.Added + stats.Removed + stats.Modified + stats.Unchanged
			fmt.Fprintf(writer, "%s: +%d, -%d, ~%d (%d total)\n", 
				resourceType, stats.Added, stats.Removed, stats.Modified, total)
		}
		fmt.Fprintf(writer, "\n")
	}
	
	// Added resources
	if len(result.Added) > 0 {
		fmt.Fprintf(writer, "ADDED RESOURCES (%d)\n", len(result.Added))
		fmt.Fprintf(writer, "-------------------\n")
		for _, resource := range result.Added {
			fmt.Fprintf(writer, "+ %s: %s (%s)\n", resource.ResourceType, resource.ResourceName, resource.OCID)
			fmt.Fprintf(writer, "  Compartment: %s\n", resource.CompartmentID)
			if len(resource.AdditionalInfo) > 0 {
				fmt.Fprintf(writer, "  %s\n", formatAdditionalInfo(resource.AdditionalInfo))
			}
			fmt.Fprintf(writer, "\n")
		}
	}
	
	// Removed resources
	if len(result.Removed) > 0 {
		fmt.Fprintf(writer, "REMOVED RESOURCES (%d)\n", len(result.Removed))
		fmt.Fprintf(writer, "---------------------\n")
		for _, resource := range result.Removed {
			fmt.Fprintf(writer, "- %s: %s (%s)\n", resource.ResourceType, resource.ResourceName, resource.OCID)
			fmt.Fprintf(writer, "  Compartment: %s\n", resource.CompartmentID)
			if len(resource.AdditionalInfo) > 0 {
				fmt.Fprintf(writer, "  %s\n", formatAdditionalInfo(resource.AdditionalInfo))
			}
			fmt.Fprintf(writer, "\n")
		}
	}
	
	// Modified resources
	if len(result.Modified) > 0 {
		fmt.Fprintf(writer, "MODIFIED RESOURCES (%d)\n", len(result.Modified))
		fmt.Fprintf(writer, "-----------------------\n")
		for _, modified := range result.Modified {
			resource := modified.ResourceInfo
			fmt.Fprintf(writer, "~ %s: %s (%s)\n", resource.ResourceType, resource.ResourceName, resource.OCID)
			fmt.Fprintf(writer, "  Compartment: %s\n", resource.CompartmentID)
			fmt.Fprintf(writer, "  Changes:\n")
			for _, change := range modified.Changes {
				fmt.Fprintf(writer, "    - %s: %v → %v\n", 
					strings.TrimPrefix(change.Field, "AdditionalInfo."), 
					formatValue(change.OldValue), 
					formatValue(change.NewValue))
			}
			fmt.Fprintf(writer, "\n")
		}
	}
	
	// Unchanged resources (if detailed mode)
	if result.Unchanged != nil && len(result.Unchanged) > 0 {
		fmt.Fprintf(writer, "UNCHANGED RESOURCES (%d)\n", len(result.Unchanged))
		fmt.Fprintf(writer, "-----------------------\n")
		for _, resource := range result.Unchanged {
			fmt.Fprintf(writer, "= %s: %s (%s)\n", resource.ResourceType, resource.ResourceName, resource.OCID)
		}
		fmt.Fprintf(writer, "\n")
	}
	
	return nil
}

// formatAdditionalInfo formats additional info for text output
func formatAdditionalInfo(info map[string]interface{}) string {
	var parts []string
	
	// Prioritize important fields
	priorityFields := []string{"shape", "primary_ip", "cidr_block", "size_gb", "performance_tier"}
	
	for _, field := range priorityFields {
		if value, exists := info[field]; exists {
			parts = append(parts, fmt.Sprintf("%s: %v", field, formatValue(value)))
		}
	}
	
	// Add other fields (up to 3 more)
	count := 0
	for key, value := range info {
		if count >= 3 {
			break
		}
		found := false
		for _, priorityField := range priorityFields {
			if key == priorityField {
				found = true
				break
			}
		}
		if !found {
			parts = append(parts, fmt.Sprintf("%s: %v", key, formatValue(value)))
			count++
		}
	}
	
	return strings.Join(parts, ", ")
}

// formatValue formats a value for display
func formatValue(value interface{}) string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", value)
}

// validateDiffFiles validates that both input files exist and are readable
func validateDiffFiles(oldFile, newFile string) error {
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		return fmt.Errorf("old file not found: %s", oldFile)
	}
	
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		return fmt.Errorf("new file not found: %s", newFile)
	}
	
	if oldFile == newFile {
		return fmt.Errorf("old and new files cannot be the same: %s", oldFile)
	}
	
	return nil
}