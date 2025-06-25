package main

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/identity"
)

// TestCompartmentNameCache_GetCompartmentName tests the compartment name cache functionality
func TestCompartmentNameCache_GetCompartmentName(t *testing.T) {
	// Initialize logger for tests
	logger = NewLogger(LogLevelSilent)
	tests := []struct {
		name          string
		setupCache    func(*CompartmentNameCache)
		compartmentID string
		expected      string
		description   string
	}{
		{
			name: "cache_hit",
			setupCache: func(cache *CompartmentNameCache) {
				cache.cache["ocid1.compartment.oc1..test123"] = "prod-compartment"
			},
			compartmentID: "ocid1.compartment.oc1..test123",
			expected:      "prod-compartment",
			description:   "Should return cached compartment name",
		},
		// Note: cache_miss test commented out as it requires OCI API call
		// {
		//	name: "cache_miss_fallback",
		//	setupCache: func(cache *CompartmentNameCache) {
		//		// Empty cache
		//	},
		//	compartmentID: "ocid1.compartment.oc1..unknown456",
		//	expected:     "ocid1.comp...own456", // formatShortOCID result
		//	description:  "Should return short OCID when cache miss occurs",
		// },
		{
			name: "tenancy_root",
			setupCache: func(cache *CompartmentNameCache) {
				cache.cache["ocid1.tenancy.oc1..root789"] = "root"
			},
			compartmentID: "ocid1.tenancy.oc1..root789",
			expected:      "root",
			description:   "Should handle tenancy root compartment",
		},
		{
			name: "empty_compartment_id",
			setupCache: func(cache *CompartmentNameCache) {
				// Empty cache, but will hit cache logic for empty string
				cache.cache[""] = "root"
			},
			compartmentID: "",
			expected:      "root",
			description:   "Should handle empty compartment ID gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock cache without real OCI client
			cache := &CompartmentNameCache{
				cache: make(map[string]string),
				mu:    sync.RWMutex{},
				// client field omitted as we're testing cache behavior
			}

			// Setup cache state
			tt.setupCache(cache)

			// Test compartment name retrieval
			ctx := context.Background()
			result := cache.GetCompartmentName(ctx, tt.compartmentID)

			if result != tt.expected {
				t.Errorf("GetCompartmentName() = %q, want %q\nDescription: %s",
					result, tt.expected, tt.description)
			}
		})
	}
}

// TestCompartmentNameCache_ConcurrentAccess tests thread safety
func TestCompartmentNameCache_ConcurrentAccess(t *testing.T) {
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}

	// Populate cache with test data
	cache.cache["ocid1.compartment.oc1..test1"] = "compartment-1"
	cache.cache["ocid1.compartment.oc1..test2"] = "compartment-2"
	cache.cache["ocid1.compartment.oc1..test3"] = "compartment-3"

	// Test concurrent reads
	numGoroutines := 10
	numReads := 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			ctx := context.Background()
			for j := 0; j < numReads; j++ {
				compartmentID := "ocid1.compartment.oc1..test1"
				result := cache.GetCompartmentName(ctx, compartmentID)

				if result != "compartment-1" {
					errors <- fmt.Errorf("goroutine %d: expected 'compartment-1', got %q",
						goroutineID, result)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors from concurrent access
	for err := range errors {
		t.Error(err)
	}
}

// TestCompartmentNameCache_GetCacheStats tests cache statistics
func TestCompartmentNameCache_GetCacheStats(t *testing.T) {
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}

	// Test empty cache
	totalEntries, hitRate := cache.GetCacheStats()
	if totalEntries != 0 {
		t.Errorf("Empty cache should have 0 entries, got %d", totalEntries)
	}
	if hitRate != 0.0 {
		t.Errorf("Empty cache should have 0.0 hit rate, got %.2f", hitRate)
	}

	// Add some entries
	cache.cache["ocid1.compartment.oc1..test1"] = "compartment-1"
	cache.cache["ocid1.compartment.oc1..test2"] = "compartment-2"

	totalEntries, _ = cache.GetCacheStats()
	if totalEntries != 2 {
		t.Errorf("Cache with 2 entries should return 2, got %d", totalEntries)
	}
}

// TestFormatShortOCID tests OCID shortening functionality
func TestFormatShortOCID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard_compartment_ocid",
			input:    "ocid1.compartment.oc1.ap-tokyo-1.aaaaaaaabbbbbbbcccccccddddddd",
			expected: "ocid1.comp...ddddddd",
		},
		{
			name:     "tenancy_ocid",
			input:    "ocid1.tenancy.oc1..aaaaaaaabbbbbbbcccccccddddddd",
			expected: "ocid1.tena...ddddddd",
		},
		{
			name:     "short_ocid",
			input:    "ocid1.comp.oc1..abc",
			expected: "ocid1.comp...c1..abc", // Should be formatted with comp prefix
		},
		{
			name:     "empty_ocid",
			input:    "",
			expected: "unknown",
		},
		{
			name:     "invalid_ocid_format",
			input:    "not-an-ocid",
			expected: "not-an-ocid", // Return as-is for invalid format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatShortOCID(tt.input)
			if result != tt.expected {
				t.Errorf("formatShortOCID(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}

// TestNewCompartmentNameCache tests cache initialization
func TestNewCompartmentNameCache(t *testing.T) {
	// Create a mock identity client (we can't create a real one without OCI credentials)
	var mockClient identity.IdentityClient

	cache := NewCompartmentNameCache(mockClient)

	if cache == nil {
		t.Fatal("NewCompartmentNameCache() should not return nil")
	}

	if cache.cache == nil {
		t.Error("Cache map should be initialized")
	}
}

// BenchmarkCompartmentNameCache_GetCompartmentName benchmarks cache performance
func BenchmarkCompartmentNameCache_GetCompartmentName(b *testing.B) {
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}

	// Populate cache with test data
	for i := 0; i < 1000; i++ {
		compartmentID := fmt.Sprintf("ocid1.compartment.oc1..test%04d", i)
		compartmentName := fmt.Sprintf("compartment-%04d", i)
		cache.cache[compartmentID] = compartmentName
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			compartmentID := "ocid1.compartment.oc1..test0500"
			_ = cache.GetCompartmentName(ctx, compartmentID)
		}
	})
}

// TestCreateResourceInfo tests the unified resource creation function
func TestCreateResourceInfo(t *testing.T) {
	// Setup mock cache
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}
	cache.cache["ocid1.compartment.oc1..test123"] = "test-compartment"

	ctx := context.Background()
	additionalInfo := map[string]interface{}{
		"shape": "VM.Standard2.1",
		"count": 1,
	}

	result := createResourceInfo(
		ctx,
		"ComputeInstance",
		"web-server-1",
		"ocid1.instance.oc1..instance123",
		"ocid1.compartment.oc1..test123",
		additionalInfo,
		cache,
	)

	// Verify all fields are correctly set
	if result.ResourceType != "ComputeInstance" {
		t.Errorf("ResourceType = %q, want %q", result.ResourceType, "ComputeInstance")
	}

	if result.CompartmentName != "test-compartment" {
		t.Errorf("CompartmentName = %q, want %q", result.CompartmentName, "test-compartment")
	}

	if result.ResourceName != "web-server-1" {
		t.Errorf("ResourceName = %q, want %q", result.ResourceName, "web-server-1")
	}

	if result.OCID != "ocid1.instance.oc1..instance123" {
		t.Errorf("OCID = %q, want %q", result.OCID, "ocid1.instance.oc1..instance123")
	}

	if result.CompartmentID != "ocid1.compartment.oc1..test123" {
		t.Errorf("CompartmentID = %q, want %q", result.CompartmentID, "ocid1.compartment.oc1..test123")
	}

	if len(result.AdditionalInfo) != 2 {
		t.Errorf("AdditionalInfo length = %d, want %d", len(result.AdditionalInfo), 2)
	}

	if result.AdditionalInfo["shape"] != "VM.Standard2.1" {
		t.Errorf("AdditionalInfo[shape] = %q, want %q", result.AdditionalInfo["shape"], "VM.Standard2.1")
	}
}
