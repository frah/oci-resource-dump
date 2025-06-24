package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/identity"
)

// TestSystemIntegration_CompartmentNameEndToEnd tests the complete compartment name feature
func TestSystemIntegration_CompartmentNameEndToEnd(t *testing.T) {
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)
	
	// Test complete workflow: Cache → Resource Creation → Output
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}
	
	// Setup test data
	testCompartments := map[string]string{
		"ocid1.compartment.oc1..prod123":    "prod-compartment",
		"ocid1.compartment.oc1..dev456":     "dev-compartment", 
		"ocid1.compartment.oc1..staging789": "staging-compartment",
	}
	
	// Populate cache
	for ocid, name := range testCompartments {
		cache.cache[ocid] = name
	}
	
	ctx := context.Background()
	
	// Test resource creation with compartment names
	resources := []ResourceInfo{}
	for ocid, expectedName := range testCompartments {
		resource := createResourceInfo(
			ctx,
			"ComputeInstances",
			fmt.Sprintf("test-instance-%s", expectedName),
			fmt.Sprintf("ocid1.instance.oc1..test-%s", expectedName),
			ocid,
			map[string]interface{}{"shape": "VM.Standard2.1"},
			cache,
		)
		resources = append(resources, resource)
	}
	
	// Verify all resources have correct compartment names
	for _, resource := range resources {
		expectedName := testCompartments[resource.CompartmentID]
		if resource.CompartmentName != expectedName {
			t.Errorf("Resource %s: expected compartment name %q, got %q", 
				resource.ResourceName, expectedName, resource.CompartmentName)
		}
	}
	
	// Test output generation
	testOutputs := []struct {
		format string
		hasCompartmentName bool
	}{
		{"json", true},
		{"csv", true},
		{"tsv", true},
	}
	
	for _, test := range testOutputs {
		t.Run(fmt.Sprintf("output_%s", test.format), func(t *testing.T) {
			// Create temporary file for testing
			tempFile := filepath.Join(t.TempDir(), fmt.Sprintf("test_output.%s", test.format))
			
			err := outputResourcesToFile(resources, test.format, tempFile)
			if err != nil {
				t.Fatalf("Failed to output %s format: %v", test.format, err)
			}
			
			// Verify file exists and contains compartment names
			content, err := os.ReadFile(tempFile)
			if err != nil {
				t.Fatalf("Failed to read output file: %v", err)
			}
			
			contentStr := string(content)
			
			// Check that compartment names appear in output
			for _, expectedName := range testCompartments {
				if !strings.Contains(contentStr, expectedName) {
					t.Errorf("Output %s missing compartment name: %s", test.format, expectedName)
				}
			}
		})
	}
}

// TestSystemIntegration_ErrorHandling tests error scenarios
func TestSystemIntegration_ErrorHandling(t *testing.T) {
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)
	
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}
	
	ctx := context.Background()
	
	// Test with invalid compartment OCID
	invalidOCID := "invalid-ocid-format"
	resource := createResourceInfo(
		ctx,
		"ComputeInstances",
		"test-instance",
		"ocid1.instance.oc1..test123",
		invalidOCID,
		map[string]interface{}{"shape": "VM.Standard2.1"},
		cache,
	)
	
	// Should get fallback compartment name
	if resource.CompartmentName == "" {
		t.Error("Expected fallback compartment name for invalid OCID, got empty string")
	}
	
	if !strings.Contains(resource.CompartmentName, "...") {
		t.Errorf("Expected short OCID format in compartment name, got: %s", resource.CompartmentName)
	}
}

// TestSystemIntegration_PerformanceUnderLoad tests performance under load
func TestSystemIntegration_PerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)
	
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}
	
	// Pre-populate cache with many compartments
	numCompartments := 1000
	for i := 0; i < numCompartments; i++ {
		ocid := fmt.Sprintf("ocid1.compartment.oc1..test%08d", i)
		name := fmt.Sprintf("compartment-%d", i)
		cache.cache[ocid] = name
	}
	
	ctx := context.Background()
	
	// Test high-load resource creation
	numGoroutines := 50
	resourcesPerGoroutine := 100
	
	var wg sync.WaitGroup
	start := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < resourcesPerGoroutine; j++ {
				compartmentIndex := (goroutineID*resourcesPerGoroutine + j) % numCompartments
				ocid := fmt.Sprintf("ocid1.compartment.oc1..test%08d", compartmentIndex)
				
				resource := createResourceInfo(
					ctx,
					"ComputeInstances",
					fmt.Sprintf("instance-%d-%d", goroutineID, j),
					fmt.Sprintf("ocid1.instance.oc1..test-%d-%d", goroutineID, j),
					ocid,
					map[string]interface{}{"shape": "VM.Standard2.1"},
					cache,
				)
				
				// Verify compartment name is resolved
				expectedName := fmt.Sprintf("compartment-%d", compartmentIndex)
				if resource.CompartmentName != expectedName {
					t.Errorf("Goroutine %d: expected %s, got %s", 
						goroutineID, expectedName, resource.CompartmentName)
				}
			}
		}(i)
	}
	
	wg.Wait()
	elapsed := time.Since(start)
	
	totalResources := numGoroutines * resourcesPerGoroutine
	resourcesPerSecond := float64(totalResources) / elapsed.Seconds()
	
	t.Logf("Performance test: %d resources in %v (%.2f resources/sec)", 
		totalResources, elapsed, resourcesPerSecond)
	
	// Performance threshold: should process at least 1000 resources per second
	if resourcesPerSecond < 1000 {
		t.Errorf("Performance below threshold: %.2f resources/sec (expected >= 1000)", resourcesPerSecond)
	}
}

// TestSystemIntegration_FilteringWithCompartmentNames tests filtering integration
func TestSystemIntegration_FilteringWithCompartmentNames(t *testing.T) {
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)
	
	// Test compartment filtering with compartment names
	compartments := []identity.Compartment{
		{
			Id:             &[]string{"ocid1.compartment.oc1..prod123"}[0],
			Name:           &[]string{"prod-compartment"}[0],
			LifecycleState: identity.CompartmentLifecycleStateActive,
		},
		{
			Id:             &[]string{"ocid1.compartment.oc1..dev456"}[0],
			Name:           &[]string{"dev-compartment"}[0],
			LifecycleState: identity.CompartmentLifecycleStateActive,
		},
	}
	
	// Test include filter
	filterConfig := FilterConfig{
		IncludeCompartments: []string{"ocid1.compartment.oc1..prod123"},
	}
	
	filtered := ApplyCompartmentFilter(compartments, filterConfig)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 compartment after filtering, got %d", len(filtered))
	}
	
	if filtered[0].Name == nil || *filtered[0].Name != "prod-compartment" {
		t.Error("Filtered compartment should be prod-compartment")
	}
	
	// Test resource type filtering
	if !ApplyResourceTypeFilter("ComputeInstances", FilterConfig{
		IncludeResourceTypes: []string{"compute_instances"},
	}) {
		t.Error("compute_instances should be included")
	}
	
	if ApplyResourceTypeFilter("ComputeInstances", FilterConfig{
		ExcludeResourceTypes: []string{"compute_instances"},
	}) {
		t.Error("compute_instances should be excluded")
	}
	
	// Test name pattern filtering
	compiledFilters, err := CompileFilters(FilterConfig{
		NamePattern: "^prod-.*",
	})
	if err != nil {
		t.Fatalf("Failed to compile filters: %v", err)
	}
	
	if !ApplyNameFilter("prod-web-server", compiledFilters) {
		t.Error("prod-web-server should match pattern")
	}
	
	if ApplyNameFilter("dev-web-server", compiledFilters) {
		t.Error("dev-web-server should not match pattern")
	}
}

// TestSystemIntegration_OutputConsistency tests output consistency across formats
func TestSystemIntegration_OutputConsistency(t *testing.T) {
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)
	
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}
	
	// Setup test data
	cache.cache["ocid1.compartment.oc1..test123"] = "test-compartment"
	
	ctx := context.Background()
	
	// Create test resource
	resource := createResourceInfo(
		ctx,
		"ComputeInstances",
		"test-instance",
		"ocid1.instance.oc1..test123",
		"ocid1.compartment.oc1..test123",
		map[string]interface{}{
			"shape":      "VM.Standard2.1",
			"primary_ip": "10.0.1.10",
		},
		cache,
	)
	
	resources := []ResourceInfo{resource}
	
	// Test all output formats in temporary directory
	tempDir := t.TempDir()
	
	formats := []string{"json", "csv", "tsv"}
	for _, format := range formats {
		fileName := filepath.Join(tempDir, fmt.Sprintf("test.%s", format))
		
		err := outputResourcesToFile(resources, format, fileName)
		if err != nil {
			t.Fatalf("Failed to output %s format: %v", format, err)
		}
		
		// Verify file exists
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			t.Errorf("Output file not created for format %s", format)
		}
		
		// Read and verify content
		content, err := os.ReadFile(fileName)
		if err != nil {
			t.Fatalf("Failed to read %s file: %v", format, err)
		}
		
		contentStr := string(content)
		
		// All formats should contain these key elements
		expectedElements := []string{
			"ComputeInstances",
			"test-compartment",
			"test-instance",
			"ocid1.instance.oc1..test123",
			"ocid1.compartment.oc1..test123",
		}
		
		for _, element := range expectedElements {
			if !strings.Contains(contentStr, element) {
				t.Errorf("Format %s missing element: %s", format, element)
			}
		}
	}
}

// TestSystemIntegration_TimeoutHandling tests timeout scenarios
func TestSystemIntegration_TimeoutHandling(t *testing.T) {
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)
	
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}
	
	// Test with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	
	// This should complete quickly using cache or fallback
	resource := createResourceInfo(
		ctx,
		"ComputeInstances", 
		"test-instance",
		"ocid1.instance.oc1..test123",
		"ocid1.compartment.oc1..nonexistent",
		map[string]interface{}{"shape": "VM.Standard2.1"},
		cache,
	)
	
	// Should get some form of compartment name (cached or fallback)
	if resource.CompartmentName == "" {
		t.Error("Expected non-empty compartment name even with timeout")
	}
	
	// Verify other fields are populated correctly
	if resource.ResourceType != "ComputeInstances" {
		t.Errorf("Expected ResourceType ComputeInstances, got %s", resource.ResourceType)
	}
	
	if resource.ResourceName != "test-instance" {
		t.Errorf("Expected ResourceName test-instance, got %s", resource.ResourceName)
	}
}

// TestSystemIntegration_MemoryUsage tests memory efficiency
func TestSystemIntegration_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}
	
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)
	
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}
	
	// Test memory usage with large cache
	numCompartments := 10000
	
	// Populate cache
	for i := 0; i < numCompartments; i++ {
		ocid := fmt.Sprintf("ocid1.compartment.oc1..test%08d", i)
		name := fmt.Sprintf("compartment-%d", i)
		cache.cache[ocid] = name
	}
	
	// Verify cache size
	cacheSize, _ := cache.GetCacheStats()
	if cacheSize != numCompartments {
		t.Errorf("Expected cache size %d, got %d", numCompartments, cacheSize)
	}
	
	// Test cache access patterns
	ctx := context.Background()
	
	// Random access pattern
	for i := 0; i < 1000; i++ {
		compartmentIndex := i % numCompartments
		ocid := fmt.Sprintf("ocid1.compartment.oc1..test%08d", compartmentIndex)
		
		name := cache.GetCompartmentName(ctx, ocid)
		expectedName := fmt.Sprintf("compartment-%d", compartmentIndex)
		
		if name != expectedName {
			t.Errorf("Cache miss for OCID %s: expected %s, got %s", ocid, expectedName, name)
		}
	}
	
	t.Logf("Memory test completed with %d compartments in cache", numCompartments)
}