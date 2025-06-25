package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/identity"
)

// BenchmarkOptimizedCompartmentNameCache benchmarks the optimized cache performance
func BenchmarkOptimizedCompartmentNameCache(b *testing.B) {
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}

	// Pre-populate cache with test data
	testCompartments := make(map[string]string)
	for i := 0; i < 1000; i++ {
		ocid := generateTestOCID("compartment", i)
		name := generateTestCompartmentName(i)
		testCompartments[ocid] = name
		cache.cache[ocid] = name
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ocid := generateTestOCID("compartment", i%1000)
			_ = cache.GetCompartmentName(ctx, ocid)
			i++
		}
	})
}

// BenchmarkBatchPreload benchmarks the batch preloading performance
func BenchmarkBatchPreload(b *testing.B) {
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}

	// Create test compartments
	compartments := make([]identity.Compartment, 100)
	for i := 0; i < 100; i++ {
		ocid := generateTestOCID("compartment", i)
		name := generateTestCompartmentName(i)
		compartments[i] = identity.Compartment{
			Id:   &ocid,
			Name: &name,
		}
	}

	tenancyOCID := "ocid1.tenancy.oc1..test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.ClearCache()
		_ = cache.batchPreloadCompartments(compartments, tenancyOCID)
	}
}

// BenchmarkSimplePreload benchmarks the simple preloading performance
func BenchmarkSimplePreload(b *testing.B) {
	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}

	// Create test compartments
	compartments := make([]identity.Compartment, 100)
	for i := 0; i < 100; i++ {
		ocid := generateTestOCID("compartment", i)
		name := generateTestCompartmentName(i)
		compartments[i] = identity.Compartment{
			Id:   &ocid,
			Name: &name,
		}
	}

	tenancyOCID := "ocid1.tenancy.oc1..test"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.ClearCache()
		_ = cache.simplePreloadCompartments(compartments, tenancyOCID)
	}
}

// TestPerformanceOptimizations tests that optimizations work correctly
func TestPerformanceOptimizations(t *testing.T) {
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)

	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}

	// Test double-checked locking
	ctx := context.Background()
	testOCID := "ocid1.compartment.oc1..test123"

	// First call should miss cache and trigger fetch
	start := time.Now()
	name1 := cache.GetCompartmentName(ctx, testOCID)
	firstCallDuration := time.Since(start)

	// Second call should hit cache and be much faster
	start = time.Now()
	name2 := cache.GetCompartmentName(ctx, testOCID)
	secondCallDuration := time.Since(start)

	if name1 != name2 {
		t.Errorf("Cache inconsistency: first call returned %q, second call returned %q", name1, name2)
	}

	// Cache hit should be significantly faster (allow some variation for timing)
	if secondCallDuration >= firstCallDuration {
		t.Logf("Warning: Cache hit (%v) was not faster than cache miss (%v)", secondCallDuration, firstCallDuration)
	}

	t.Logf("Cache miss: %v, Cache hit: %v", firstCallDuration, secondCallDuration)
}

// TestConcurrentCacheAccess tests the thread safety of optimized cache
func TestConcurrentCacheAccess(t *testing.T) {
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)

	cache := &CompartmentNameCache{
		cache: make(map[string]string),
		mu:    sync.RWMutex{},
	}

	ctx := context.Background()
	numGoroutines := 50
	numOpsPerGoroutine := 100

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				ocid := generateTestOCID("compartment", (goroutineID*numOpsPerGoroutine+j)%10)
				_ = cache.GetCompartmentName(ctx, ocid)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalOps := numGoroutines * numOpsPerGoroutine
	opsPerSecond := float64(totalOps) / elapsed.Seconds()

	t.Logf("Concurrent access test: %d ops in %v (%.2f ops/sec)", totalOps, elapsed, opsPerSecond)

	// Verify cache integrity
	cacheSize, _ := cache.GetCacheStats()
	if cacheSize > 10 {
		t.Errorf("Expected at most 10 unique compartments in cache, got %d", cacheSize)
	}
}

// TestBatchVsSimplePreload compares batch and simple preload performance
func TestBatchVsSimplePreload(t *testing.T) {
	// Initialize logger for testing
	logger = NewLogger(LogLevelSilent)

	// Test with different compartment counts
	testSizes := []int{10, 50, 100, 200}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			// Create test compartments
			compartments := make([]identity.Compartment, size)
			for i := 0; i < size; i++ {
				ocid := generateTestOCID("compartment", i)
				name := generateTestCompartmentName(i)
				compartments[i] = identity.Compartment{
					Id:   &ocid,
					Name: &name,
				}
			}

			tenancyOCID := "ocid1.tenancy.oc1..test"

			// Test simple preload
			simpleCache := &CompartmentNameCache{
				cache: make(map[string]string),
				mu:    sync.RWMutex{},
			}

			start := time.Now()
			err := simpleCache.simplePreloadCompartments(compartments, tenancyOCID)
			simpleTime := time.Since(start)

			if err != nil {
				t.Fatalf("Simple preload failed: %v", err)
			}

			// Test batch preload
			batchCache := &CompartmentNameCache{
				cache: make(map[string]string),
				mu:    sync.RWMutex{},
			}

			start = time.Now()
			err = batchCache.batchPreloadCompartments(compartments, tenancyOCID)
			batchTime := time.Since(start)

			if err != nil {
				t.Fatalf("Batch preload failed: %v", err)
			}

			// Verify both methods produce same results
			simpleSize, _ := simpleCache.GetCacheStats()
			batchSize, _ := batchCache.GetCacheStats()

			if simpleSize != batchSize {
				t.Errorf("Cache size mismatch: simple=%d, batch=%d", simpleSize, batchSize)
			}

			t.Logf("Size %d: Simple=%v, Batch=%v (speedup=%.2fx)",
				size, simpleTime, batchTime, simpleTime.Seconds()/batchTime.Seconds())
		})
	}
}

// Helper functions for test data generation

func generateTestOCID(resourceType string, id int) string {
	return fmt.Sprintf("ocid1.%s.oc1..test%08d", resourceType, id)
}

func generateTestCompartmentName(id int) string {
	switch id % 5 {
	case 0:
		return fmt.Sprintf("prod-compartment-%d", id)
	case 1:
		return fmt.Sprintf("dev-compartment-%d", id)
	case 2:
		return fmt.Sprintf("test-compartment-%d", id)
	case 3:
		return fmt.Sprintf("staging-compartment-%d", id)
	default:
		return fmt.Sprintf("other-compartment-%d", id)
	}
}
