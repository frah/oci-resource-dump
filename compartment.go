package main

import (
	"context"
	"fmt"

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

// GetCompartmentName retrieves the compartment name for a given OCID with caching
func (c *CompartmentNameCache) GetCompartmentName(ctx context.Context, compartmentOCID string) string {
	// Check cache first
	c.mu.RLock()
	if name, exists := c.cache[compartmentOCID]; exists {
		c.mu.RUnlock()
		return name
	}
	c.mu.RUnlock()

	// If not in cache, fetch from API
	name := c.fetchCompartmentName(ctx, compartmentOCID)
	
	// Store in cache
	c.mu.Lock()
	c.cache[compartmentOCID] = name
	c.mu.Unlock()

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

// PreloadCompartmentNames fetches compartment names for all compartments in tenancy
// This is useful for improving performance by reducing API calls during resource discovery
func (c *CompartmentNameCache) PreloadCompartmentNames(ctx context.Context, tenancyOCID string) error {
	logger.Debug("Preloading compartment names for tenancy: %s", tenancyOCID)

	// Get all compartments in the tenancy
	compartments, err := c.getAllCompartments(ctx, tenancyOCID)
	if err != nil {
		return fmt.Errorf("failed to get compartments: %w", err)
	}

	logger.Debug("Found %d compartments to preload", len(compartments))

	// Preload names into cache
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, compartment := range compartments {
		if compartment.Id != nil && compartment.Name != nil {
			c.cache[*compartment.Id] = *compartment.Name
		}
	}

	// Also add root compartment
	c.cache[tenancyOCID] = "root"

	logger.Verbose("Preloaded %d compartment names into cache", len(c.cache))
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

// ClearCache clears all cached compartment names
func (c *CompartmentNameCache) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache = make(map[string]string)
}