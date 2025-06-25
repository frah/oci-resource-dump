package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

// NewCompartmentNameCache creates a new compartment name cache instance
func NewCompartmentNameCache(identityClient identity.IdentityClient) *CompartmentNameCache {
	return &CompartmentNameCache{
		cache:  make(map[string]string),
		client: identityClient,
	}
}

// GetCompartmentName retrieves the compartment name for a given OCID with optimized caching
func (c *CompartmentNameCache) GetCompartmentName(ctx context.Context, compartmentOCID string) string {
	// Fast path: check cache with read lock
	c.mu.RLock()
	if name, exists := c.cache[compartmentOCID]; exists {
		c.mu.RUnlock()
		return name
	}
	c.mu.RUnlock()

	// Slow path: fetch from API with double-checked locking
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine might have fetched it
	if name, exists := c.cache[compartmentOCID]; exists {
		return name
	}

	// Fetch with timeout context for performance
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	name := c.fetchCompartmentName(ctxWithTimeout, compartmentOCID)
	c.cache[compartmentOCID] = name

	return name
}

// fetchCompartmentName retrieves compartment name from OCI API
func (c *CompartmentNameCache) fetchCompartmentName(ctx context.Context, compartmentOCID string) string {
	// Handle root compartment (tenancy)
	if compartmentOCID == "" {
		return "root"
	}

	request := identity.GetCompartmentRequest{
		CompartmentId: common.String(compartmentOCID),
	}

	response, err := c.client.GetCompartment(ctx, request)
	if err != nil {
		logger.Debug("Failed to get compartment name for OCID %s: %v", compartmentOCID, err)
		// Return short OCID as fallback
		return c.formatShortOCID(compartmentOCID)
	}

	if response.Name != nil {
		return *response.Name
	}

	// Fallback to short OCID if name is not available
	return c.formatShortOCID(compartmentOCID)
}

// formatShortOCID creates a short, readable version of an OCID for fallback display
func (c *CompartmentNameCache) formatShortOCID(ocid string) string {
	if len(ocid) <= 8 {
		return ocid
	}

	// Extract the last 8 characters for short display
	shortOCID := ocid[len(ocid)-8:]
	return fmt.Sprintf("ocid-...%s", shortOCID)
}

// PreloadCompartmentNames fetches compartment names with optimized concurrent processing
// This dramatically improves performance by reducing API calls during resource discovery
func (c *CompartmentNameCache) PreloadCompartmentNames(ctx context.Context, tenancyOCID string) error {
	logger.Debug("Preloading compartment names for tenancy: %s", tenancyOCID)
	startTime := time.Now()

	// Get all compartments in the tenancy with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	compartments, err := c.getAllCompartments(ctxWithTimeout, tenancyOCID)
	if err != nil {
		return fmt.Errorf("failed to get compartments: %w", err)
	}

	logger.Debug("Found %d compartments to preload", len(compartments))

	// Use batch processing only for very large tenancies where overhead is justified
	// Based on performance testing, simple approach is faster for smaller tenancies
	if len(compartments) > 200 {
		err = c.batchPreloadCompartments(compartments, tenancyOCID)
		logger.Debug("Using batch preload for %d compartments", len(compartments))
	} else {
		err = c.simplePreloadCompartments(compartments, tenancyOCID)
		logger.Debug("Using simple preload for %d compartments", len(compartments))
	}

	if err != nil {
		return err
	}

	elapsed := time.Since(startTime)
	cacheSize := len(c.cache)
	logger.Verbose("Preloaded %d compartment names into cache in %v", cacheSize, elapsed)

	// Log performance metrics for optimization tracking
	if cacheSize > 0 {
		avgTimePerCompartment := elapsed / time.Duration(cacheSize)
		logger.Debug("Average preload time per compartment: %v", avgTimePerCompartment)
	}

	return nil
}

// getAllCompartments recursively retrieves all compartments in the tenancy
func (c *CompartmentNameCache) getAllCompartments(ctx context.Context, compartmentOCID string) ([]identity.Compartment, error) {
	var allCompartments []identity.Compartment

	request := identity.ListCompartmentsRequest{
		CompartmentId:          common.String(compartmentOCID),
		AccessLevel:            identity.ListCompartmentsAccessLevelAccessible,
		CompartmentIdInSubtree: common.Bool(true),
	}

	response, err := c.client.ListCompartments(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to list compartments: %w", err)
	}

	allCompartments = append(allCompartments, response.Items...)

	// Handle pagination
	for response.OpcNextPage != nil {
		request.Page = response.OpcNextPage
		response, err = c.client.ListCompartments(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list compartments (pagination): %w", err)
		}
		allCompartments = append(allCompartments, response.Items...)
	}

	return allCompartments, nil
}

// GetCacheStats returns statistics about the compartment name cache
func (c *CompartmentNameCache) GetCacheStats() (totalEntries int, cacheHitRate float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalEntries = len(c.cache)
	// For now, return basic stats. Hit rate calculation would require
	// tracking hits/misses which can be added if needed.
	cacheHitRate = 0.0

	return totalEntries, cacheHitRate
}

// batchPreloadCompartments handles concurrent preloading for large tenancies
func (c *CompartmentNameCache) batchPreloadCompartments(compartments []identity.Compartment, tenancyOCID string) error {
	logger.Debug("Using batch preload for %d compartments", len(compartments))

	// Process in batches of 20 compartments with 3 concurrent workers
	batchSize := 20
	maxWorkers := 3

	// Create worker pool
	jobs := make(chan []identity.Compartment, maxWorkers)
	results := make(chan map[string]string, maxWorkers)
	errors := make(chan error, maxWorkers)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				batchCache := make(map[string]string)
				for _, compartment := range batch {
					if compartment.Id != nil && compartment.Name != nil {
						batchCache[*compartment.Id] = *compartment.Name
					}
				}
				results <- batchCache
			}
		}()
	}

	// Send batches to workers
	go func() {
		defer close(jobs)
		for i := 0; i < len(compartments); i += batchSize {
			end := i + batchSize
			if end > len(compartments) {
				end = len(compartments)
			}
			batch := compartments[i:end]
			jobs <- batch
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Merge results into cache
	c.mu.Lock()
	defer c.mu.Unlock()

	for batchResult := range results {
		for ocid, name := range batchResult {
			c.cache[ocid] = name
		}
	}

	// Add root compartment
	c.cache[tenancyOCID] = "root"

	return nil
}

// simplePreloadCompartments handles sequential preloading for small tenancies
func (c *CompartmentNameCache) simplePreloadCompartments(compartments []identity.Compartment, tenancyOCID string) error {
	logger.Debug("Using simple preload for %d compartments", len(compartments))

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, compartment := range compartments {
		if compartment.Id != nil && compartment.Name != nil {
			c.cache[*compartment.Id] = *compartment.Name
		}
	}

	// Add root compartment
	c.cache[tenancyOCID] = "root"

	return nil
}

// ClearCache clears all cached compartment names
func (c *CompartmentNameCache) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]string)
}

// formatShortOCID creates a short, readable version of an OCID for fallback display (global function for testing)
func formatShortOCID(ocid string) string {
	if ocid == "" {
		return "unknown"
	}

	if len(ocid) <= 15 {
		return ocid
	}

	// Extract resource type and last 7 characters for short display
	parts := strings.Split(ocid, ".")
	if len(parts) >= 2 {
		resourceType := parts[1]
		if len(resourceType) > 4 {
			resourceType = resourceType[:4]
		}
		shortEnd := ocid[len(ocid)-7:]
		return fmt.Sprintf("ocid1.%s...%s", resourceType, shortEnd)
	}

	// Fallback to simple truncation
	return fmt.Sprintf("%s...%s", ocid[:11], ocid[len(ocid)-7:])
}
