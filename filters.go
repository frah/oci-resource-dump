package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/identity"
)

// FilterConfig represents the filtering configuration
type FilterConfig struct {
	IncludeCompartments  []string `yaml:"include_compartments"`
	ExcludeCompartments  []string `yaml:"exclude_compartments"`
	IncludeResourceTypes []string `yaml:"include_resource_types"`
	ExcludeResourceTypes []string `yaml:"exclude_resource_types"`
	NamePattern          string   `yaml:"name_pattern"`
	ExcludeNamePattern   string   `yaml:"exclude_name_pattern"`
}

// Compiled regex patterns for efficient matching
type CompiledFilters struct {
	NameRegex        *regexp.Regexp
	ExcludeNameRegex *regexp.Regexp
}

// supportedResourceTypes maps CLI-friendly names to internal resource type names
var resourceTypeAliases = map[string]string{
	"compute_instances":      "ComputeInstances",
	"vcns":                   "VCNs",
	"subnets":                "Subnets",
	"block_volumes":          "BlockVolumes",
	"object_storage_buckets": "ObjectStorageBuckets",
	"object_storage":         "ObjectStorageBuckets", // Short alias for compatibility
	"oke_clusters":           "OKEClusters",
	"load_balancers":         "LoadBalancers",
	"database_systems":       "DatabaseSystems",
	"databases":              "DatabaseSystems", // Short alias for compatibility
	"drgs":                   "DRGs",
	"autonomous_databases":   "AutonomousDatabases",
	"functions":              "Functions",
	"api_gateways":           "APIGateways",
	"file_storage_systems":   "FileStorageSystems",
	"file_storage":           "FileStorageSystems", // Short alias for compatibility
	"network_load_balancers": "NetworkLoadBalancers",
	"streams":                "Streams",
	"streaming":              "Streams", // Short alias for compatibility
}

// reverseResourceTypeAliases maps internal names to CLI-friendly names
var reverseResourceTypeAliases = map[string]string{
	"ComputeInstances":     "compute_instances",
	"VCNs":                 "vcns",
	"Subnets":              "subnets",
	"BlockVolumes":         "block_volumes",
	"ObjectStorageBuckets": "object_storage_buckets",
	"OKEClusters":          "oke_clusters",
	"LoadBalancers":        "load_balancers",
	"DatabaseSystems":      "database_systems",
	"DRGs":                 "drgs",
	"AutonomousDatabases":  "autonomous_databases",
	"Functions":            "functions",
	"APIGateways":          "api_gateways",
	"FileStorageSystems":   "file_storage_systems",
	"NetworkLoadBalancers": "network_load_balancers",
	"Streams":              "streams",
}

// supportedResourceTypes contains all supported resource type names (internal format)
var supportedResourceTypes = []string{
	"ComputeInstances",
	"VCNs",
	"Subnets",
	"BlockVolumes",
	"ObjectStorageBuckets",
	"OKEClusters",
	"LoadBalancers",
	"DatabaseSystems",
	"DRGs",
	"AutonomousDatabases",
	"Functions",
	"APIGateways",
	"FileStorageSystems",
	"NetworkLoadBalancers",
	"Streams",
}

// ValidateFilterConfig validates the filter configuration
func ValidateFilterConfig(filter FilterConfig) error {
	// Validate compartment OCIDs format
	for _, ocid := range filter.IncludeCompartments {
		if !isValidCompartmentOCID(ocid) {
			return fmt.Errorf("invalid compartment OCID format: %s", ocid)
		}
	}
	for _, ocid := range filter.ExcludeCompartments {
		if !isValidCompartmentOCID(ocid) {
			return fmt.Errorf("invalid compartment OCID format: %s", ocid)
		}
	}

	// Validate resource types
	for _, rt := range filter.IncludeResourceTypes {
		if !isValidResourceType(rt) {
			return fmt.Errorf("unknown resource type '%s', supported types: %v", rt, getSupportedResourceTypeNames())
		}
	}
	for _, rt := range filter.ExcludeResourceTypes {
		if !isValidResourceType(rt) {
			return fmt.Errorf("unknown resource type '%s', supported types: %v", rt, getSupportedResourceTypeNames())
		}
	}

	// Validate regex patterns
	if filter.NamePattern != "" {
		if _, err := regexp.Compile(filter.NamePattern); err != nil {
			return fmt.Errorf("invalid regex pattern '%s': %v", filter.NamePattern, err)
		}
	}
	if filter.ExcludeNamePattern != "" {
		if _, err := regexp.Compile(filter.ExcludeNamePattern); err != nil {
			return fmt.Errorf("invalid regex pattern '%s': %v", filter.ExcludeNamePattern, err)
		}
	}

	return nil
}

// CompileFilters compiles regex patterns for efficient matching
func CompileFilters(filter FilterConfig) (*CompiledFilters, error) {
	compiled := &CompiledFilters{}

	if filter.NamePattern != "" {
		regex, err := regexp.Compile(filter.NamePattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile name pattern '%s': %v", filter.NamePattern, err)
		}
		compiled.NameRegex = regex
	}

	if filter.ExcludeNamePattern != "" {
		regex, err := regexp.Compile(filter.ExcludeNamePattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile exclude name pattern '%s': %v", filter.ExcludeNamePattern, err)
		}
		compiled.ExcludeNameRegex = regex
	}

	return compiled, nil
}

// ApplyCompartmentFilter filters compartments based on include/exclude lists
func ApplyCompartmentFilter(compartments []identity.Compartment, filter FilterConfig) []identity.Compartment {
	if len(filter.IncludeCompartments) == 0 && len(filter.ExcludeCompartments) == 0 {
		return compartments // No filtering
	}

	var filtered []identity.Compartment

	for _, compartment := range compartments {
		compartmentID := *compartment.Id

		// Apply include filter (if specified, only include compartments in the list)
		if len(filter.IncludeCompartments) > 0 {
			if !stringInSlice(compartmentID, filter.IncludeCompartments) {
				continue // Skip this compartment
			}
		}

		// Apply exclude filter (skip compartments in the exclude list)
		if len(filter.ExcludeCompartments) > 0 {
			if stringInSlice(compartmentID, filter.ExcludeCompartments) {
				continue // Skip this compartment
			}
		}

		filtered = append(filtered, compartment)
	}

	return filtered
}

// ApplyResourceTypeFilter checks if a resource type should be processed
func ApplyResourceTypeFilter(resourceType string, filter FilterConfig) bool {
	// Apply include filter (if specified, only process resource types in the list)
	if len(filter.IncludeResourceTypes) > 0 {
		included := false
		for _, rt := range filter.IncludeResourceTypes {
			if normalizeResourceType(rt) == resourceType {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// Apply exclude filter (skip resource types in the exclude list)
	if len(filter.ExcludeResourceTypes) > 0 {
		for _, rt := range filter.ExcludeResourceTypes {
			if normalizeResourceType(rt) == resourceType {
				return false
			}
		}
	}

	return true
}

// ApplyNameFilter checks if a resource name matches the filter criteria
func ApplyNameFilter(resourceName string, compiled *CompiledFilters) bool {
	// Apply include name pattern (if specified, only include resources matching the pattern)
	if compiled.NameRegex != nil {
		if !compiled.NameRegex.MatchString(resourceName) {
			return false
		}
	}

	// Apply exclude name pattern (skip resources matching the exclude pattern)
	if compiled.ExcludeNameRegex != nil {
		if compiled.ExcludeNameRegex.MatchString(resourceName) {
			return false
		}
	}

	return true
}

// Helper functions

// isValidCompartmentOCID validates the OCID format for compartments
func isValidCompartmentOCID(ocid string) bool {
	// Basic OCID format validation
	// Format: ocid1.compartment.oc1..<unique_id>
	return strings.HasPrefix(ocid, "ocid1.compartment.oc1..")
}

// isValidResourceType checks if the resource type is supported
func isValidResourceType(resourceType string) bool {
	// Check both CLI-friendly names and internal names
	if _, exists := resourceTypeAliases[strings.ToLower(resourceType)]; exists {
		return true
	}
	return stringInSlice(resourceType, supportedResourceTypes)
}

// normalizeResourceType converts CLI-friendly resource type names to internal names
func normalizeResourceType(resourceType string) string {
	if internal, exists := resourceTypeAliases[strings.ToLower(resourceType)]; exists {
		return internal
	}
	return resourceType // Return as-is if not found in aliases
}

// getSupportedResourceTypeNames returns a list of all supported resource type names (CLI-friendly)
func getSupportedResourceTypeNames() []string {
	var names []string
	for alias := range resourceTypeAliases {
		names = append(names, alias)
	}
	return names
}

// stringInSlice checks if a string exists in a slice
func stringInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// ParseResourceTypeList parses a comma-separated string of resource types
func ParseResourceTypeList(input string) []string {
	if input == "" {
		return nil
	}

	var result []string
	types := strings.Split(input, ",")
	for _, t := range types {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			// Normalize to lowercase for consistency
			result = append(result, strings.ToLower(trimmed))
		}
	}
	return result
}

// ParseCompartmentList parses a comma-separated string of compartment OCIDs
func ParseCompartmentList(input string) []string {
	if input == "" {
		return nil
	}

	var result []string
	ocids := strings.Split(input, ",")
	for _, ocid := range ocids {
		trimmed := strings.TrimSpace(ocid)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
