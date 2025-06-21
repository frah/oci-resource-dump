package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

type Config struct {
	OutputFormat string
}

type OCIClients struct {
	ComputeClient  core.ComputeClient
	VirtualNetworkClient core.VirtualNetworkClient
	BlockStorageClient core.BlockstorageClient
	IdentityClient identity.IdentityClient
}

type ResourceInfo struct {
	ResourceType string
	ResourceName string
	OCID         string
	CompartmentID string
}

func initOCIClients() (*OCIClients, error) {
	// Use instance principal authentication
	configProvider, err := auth.InstancePrincipalConfigurationProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to create instance principal config provider: %w", err)
	}

	clients := &OCIClients{}
	
	// Initialize Compute client
	computeClient, err := core.NewComputeClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	clients.ComputeClient = computeClient
	
	// Initialize VirtualNetwork client
	vnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual network client: %w", err)
	}
	clients.VirtualNetworkClient = vnClient
	
	// Initialize BlockStorage client
	bsClient, err := core.NewBlockstorageClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create block storage client: %w", err)
	}
	clients.BlockStorageClient = bsClient
	
	// Initialize Identity client
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity client: %w", err)
	}
	clients.IdentityClient = identityClient

	return clients, nil
}

func getCompartments(ctx context.Context, clients *OCIClients) ([]identity.Compartment, error) {
	// Get tenancy ID from the instance principal
	configProvider, err := auth.InstancePrincipalConfigurationProvider()
	if err != nil {
		return nil, err
	}
	
	tenancyID, err := configProvider.TenancyOCID()
	if err != nil {
		return nil, err
	}

	// List compartments
	req := identity.ListCompartmentsRequest{
		CompartmentId: common.String(tenancyID),
		AccessLevel:   identity.ListCompartmentsAccessLevelAccessible,
	}

	resp, err := clients.IdentityClient.ListCompartments(ctx, req)
	if err != nil {
		return nil, err
	}

	// Include root compartment
	compartments := resp.Items
	rootCompartment := identity.Compartment{
		Id:             common.String(tenancyID),
		Name:           common.String("root"),
		CompartmentId:  common.String(tenancyID),
		LifecycleState: identity.CompartmentLifecycleStateActive,
	}
	compartments = append([]identity.Compartment{rootCompartment}, compartments...)

	return compartments, nil
}

func discoverComputeInstances(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	req := core.ListInstancesRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.ComputeClient.ListInstances(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, instance := range resp.Items {
		if instance.LifecycleState != core.InstanceLifecycleStateTerminated {
			name := ""
			if instance.DisplayName != nil {
				name = *instance.DisplayName
			}
			ocid := ""
			if instance.Id != nil {
				ocid = *instance.Id
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:  "compute_instance",
				ResourceName:  name,
				OCID:          ocid,
				CompartmentID: compartmentID,
			})
		}
	}

	return resources, nil
}

func discoverVCNs(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	req := core.ListVcnsRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.VirtualNetworkClient.ListVcns(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, vcn := range resp.Items {
		if vcn.LifecycleState != core.VcnLifecycleStateTerminated {
			name := ""
			if vcn.DisplayName != nil {
				name = *vcn.DisplayName
			}
			ocid := ""
			if vcn.Id != nil {
				ocid = *vcn.Id
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:  "vcn",
				ResourceName:  name,
				OCID:          ocid,
				CompartmentID: compartmentID,
			})
		}
	}

	return resources, nil
}

func discoverSubnets(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	req := core.ListSubnetsRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.VirtualNetworkClient.ListSubnets(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, subnet := range resp.Items {
		if subnet.LifecycleState != core.SubnetLifecycleStateTerminated {
			name := ""
			if subnet.DisplayName != nil {
				name = *subnet.DisplayName
			}
			ocid := ""
			if subnet.Id != nil {
				ocid = *subnet.Id
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:  "subnet",
				ResourceName:  name,
				OCID:          ocid,
				CompartmentID: compartmentID,
			})
		}
	}

	return resources, nil
}

func discoverBlockVolumes(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	req := core.ListVolumesRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.BlockStorageClient.ListVolumes(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, volume := range resp.Items {
		if volume.LifecycleState != core.VolumeLifecycleStateTerminated {
			name := ""
			if volume.DisplayName != nil {
				name = *volume.DisplayName
			}
			ocid := ""
			if volume.Id != nil {
				ocid = *volume.Id
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:  "block_volume",
				ResourceName:  name,
				OCID:          ocid,
				CompartmentID: compartmentID,
			})
		}
	}

	return resources, nil
}

func isRetriableError(err error) bool {
	// Check if the error is a retriable error (non-existent resource, permission issue, etc.)
	// These should not cause the entire program to fail
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	// Common OCI errors that should be treated as "resource not found" rather than fatal errors
	return strings.Contains(errStr, "NotFound") ||
		   strings.Contains(errStr, "NotAuthorized") ||
		   strings.Contains(errStr, "Forbidden") ||
		   strings.Contains(errStr, "does not exist")
}

func discoverAllResources(ctx context.Context, clients *OCIClients) ([]ResourceInfo, error) {
	var allResources []ResourceInfo

	// Get all compartments
	compartments, err := getCompartments(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("failed to get compartments: %w", err)
	}

	// Discover resources in each compartment
	for _, compartment := range compartments {
		compartmentID := *compartment.Id

		// Discover compute instances
		if instances, err := discoverComputeInstances(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, instances...)
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "Warning: Failed to discover compute instances in compartment %s: %v\n", compartmentID, err)
		}

		// Discover VCNs
		if vcns, err := discoverVCNs(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, vcns...)
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "Warning: Failed to discover VCNs in compartment %s: %v\n", compartmentID, err)
		}

		// Discover subnets
		if subnets, err := discoverSubnets(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, subnets...)
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "Warning: Failed to discover subnets in compartment %s: %v\n", compartmentID, err)
		}

		// Discover block volumes
		if volumes, err := discoverBlockVolumes(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, volumes...)
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "Warning: Failed to discover block volumes in compartment %s: %v\n", compartmentID, err)
		}
	}

	return allResources, nil
}

func outputJSON(resources []ResourceInfo) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(resources)
}

func outputCSV(resources []ResourceInfo) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Write header
	header := []string{"ResourceType", "ResourceName", "OCID", "CompartmentID"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, resource := range resources {
		record := []string{
			resource.ResourceType,
			resource.ResourceName,
			resource.OCID,
			resource.CompartmentID,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func outputTSV(resources []ResourceInfo) error {
	// Write header
	fmt.Println("ResourceType\tResourceName\tOCID\tCompartmentID")

	// Write data
	for _, resource := range resources {
		fmt.Printf("%s\t%s\t%s\t%s\n",
			resource.ResourceType,
			resource.ResourceName,
			resource.OCID,
			resource.CompartmentID,
		)
	}

	return nil
}

func outputResources(resources []ResourceInfo, format string) error {
	switch format {
	case "json":
		return outputJSON(resources)
	case "csv":
		return outputCSV(resources)
	case "tsv":
		return outputTSV(resources)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func main() {
	config := &Config{}
	
	flag.StringVar(&config.OutputFormat, "format", "json", "Output format: csv, tsv, or json")
	flag.StringVar(&config.OutputFormat, "f", "json", "Output format: csv, tsv, or json (shorthand)")
	flag.Parse()

	// Validate output format
	validFormats := []string{"csv", "tsv", "json"}
	config.OutputFormat = strings.ToLower(config.OutputFormat)
	
	isValid := false
	for _, format := range validFormats {
		if config.OutputFormat == format {
			isValid = true
			break
		}
	}
	
	if !isValid {
		fmt.Fprintf(os.Stderr, "Error: Invalid output format '%s'. Valid formats are: csv, tsv, json\n", config.OutputFormat)
		flag.Usage()
		os.Exit(1)
	}

	// Initialize OCI clients
	clients, err := initOCIClients()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing OCI clients: %v\n", err)
		os.Exit(1)
	}

	// Discover all resources
	ctx := context.Background()
	resources, err := discoverAllResources(ctx, clients)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering resources: %v\n", err)
		os.Exit(1)
	}

	// Output resources in the specified format
	if err := outputResources(resources, config.OutputFormat); err != nil {
		fmt.Fprintf(os.Stderr, "Error outputting resources: %v\n", err)
		os.Exit(1)
	}
}