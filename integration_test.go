package main

import (
	"context"
	"sync"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

// TestCompartmentNameFilteringIntegration tests that compartment name filtering works with the new compartment name functionality
func TestCompartmentNameFilteringIntegration(t *testing.T) {
	// Create mock cache
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}
	
	// Setup test compartment names
	cache.cache["ocid1.compartment.oc1..prod123"] = "prod-compartment"
	cache.cache["ocid1.compartment.oc1..dev456"] = "dev-compartment"
	cache.cache["ocid1.compartment.oc1..test789"] = "test-compartment"

	ctx := context.Background()

	// Test that createResourceInfo correctly uses the cache
	testCases := []struct {
		name            string
		compartmentID   string
		expectedCompName string
	}{
		{
			name:            "prod compartment",
			compartmentID:   "ocid1.compartment.oc1..prod123",
			expectedCompName: "prod-compartment",
		},
		{
			name:            "dev compartment",
			compartmentID:   "ocid1.compartment.oc1..dev456",
			expectedCompName: "dev-compartment",
		},
		{
			name:            "test compartment",
			compartmentID:   "ocid1.compartment.oc1..test789",
			expectedCompName: "test-compartment",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := createResourceInfo(
				ctx,
				"ComputeInstance",
				"test-instance",
				"ocid1.instance.oc1..test123",
				tc.compartmentID,
				map[string]interface{}{"shape": "VM.Standard2.1"},
				cache,
			)

			if resource.CompartmentName != tc.expectedCompName {
				t.Errorf("Expected compartment name %q, got %q", tc.expectedCompName, resource.CompartmentName)
			}

			if resource.CompartmentID != tc.compartmentID {
				t.Errorf("Expected compartment ID %q, got %q", tc.compartmentID, resource.CompartmentID)
			}
		})
	}
}

// TestResourceTypeFilteringWithCompartmentNames tests that resource type filtering works correctly with compartment names
func TestResourceTypeFilteringWithCompartmentNames(t *testing.T) {
	filterConfig := FilterConfig{
		IncludeResourceTypes: []string{"compute_instances", "vcns"},
	}

	// Test various resource types
	testCases := []struct {
		resourceType string
		shouldPass   bool
	}{
		{"ComputeInstances", true},
		{"VCNs", true},
		{"Subnets", false},
		{"BlockVolumes", false},
		{"LoadBalancers", false},
	}

	for _, tc := range testCases {
		t.Run(tc.resourceType, func(t *testing.T) {
			result := ApplyResourceTypeFilter(tc.resourceType, filterConfig)
			if result != tc.shouldPass {
				t.Errorf("Resource type %q: expected %v, got %v", tc.resourceType, tc.shouldPass, result)
			}
		})
	}
}

// TestNameFilteringWithCompartmentNames tests that name pattern filtering works correctly with compartment names
func TestNameFilteringWithCompartmentNames(t *testing.T) {
	filterConfig := FilterConfig{
		NamePattern:        "^prod-.*",
		ExcludeNamePattern: ".*-test$",
	}

	compiledFilters, err := CompileFilters(filterConfig)
	if err != nil {
		t.Fatalf("Failed to compile filters: %v", err)
	}

	testCases := []struct {
		resourceName string
		shouldPass   bool
	}{
		{"prod-web-server", true},
		{"prod-database", true},
		{"prod-server-test", false}, // matches exclude pattern
		{"dev-web-server", false},   // doesn't match include pattern
		{"staging-prod-server", false}, // doesn't match include pattern (must start with prod-)
	}

	for _, tc := range testCases {
		t.Run(tc.resourceName, func(t *testing.T) {
			result := ApplyNameFilter(tc.resourceName, compiledFilters)
			if result != tc.shouldPass {
				t.Errorf("Resource name %q: expected %v, got %v", tc.resourceName, tc.shouldPass, result)
			}
		})
	}
}

// TestCompartmentFilteringIntegration tests that compartment filtering works correctly
func TestCompartmentFilteringIntegration(t *testing.T) {
	// Create mock compartments
	compartments := []identity.Compartment{
		{
			Id:             common.String("ocid1.compartment.oc1..prod123"),
			Name:           common.String("prod-compartment"),
			LifecycleState: identity.CompartmentLifecycleStateActive,
		},
		{
			Id:             common.String("ocid1.compartment.oc1..dev456"),
			Name:           common.String("dev-compartment"),
			LifecycleState: identity.CompartmentLifecycleStateActive,
		},
		{
			Id:             common.String("ocid1.compartment.oc1..test789"),
			Name:           common.String("test-compartment"),
			LifecycleState: identity.CompartmentLifecycleStateActive,
		},
	}

	// Test include filter
	filterConfig := FilterConfig{
		IncludeCompartments: []string{"ocid1.compartment.oc1..prod123", "ocid1.compartment.oc1..dev456"},
	}

	filtered := ApplyCompartmentFilter(compartments, filterConfig)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 compartments after include filter, got %d", len(filtered))
	}

	// Verify correct compartments are included
	expectedIDs := map[string]bool{
		"ocid1.compartment.oc1..prod123": false,
		"ocid1.compartment.oc1..dev456":  false,
	}

	for _, comp := range filtered {
		if comp.Id != nil {
			if _, exists := expectedIDs[*comp.Id]; exists {
				expectedIDs[*comp.Id] = true
			} else {
				t.Errorf("Unexpected compartment in filtered results: %s", *comp.Id)
			}
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("Expected compartment %s not found in filtered results", id)
		}
	}

	// Test exclude filter
	filterConfig2 := FilterConfig{
		ExcludeCompartments: []string{"ocid1.compartment.oc1..test789"},
	}

	filtered2 := ApplyCompartmentFilter(compartments, filterConfig2)
	if len(filtered2) != 2 {
		t.Errorf("Expected 2 compartments after exclude filter, got %d", len(filtered2))
	}

	// Verify test compartment is excluded
	for _, comp := range filtered2 {
		if comp.Id != nil && *comp.Id == "ocid1.compartment.oc1..test789" {
			t.Error("Test compartment should have been excluded but was found in results")
		}
	}
}