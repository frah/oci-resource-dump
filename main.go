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
	"github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

type Config struct {
	OutputFormat string
}

type OCIClients struct {
	ComputeClient           core.ComputeClient
	VirtualNetworkClient    core.VirtualNetworkClient
	BlockStorageClient      core.BlockstorageClient
	IdentityClient          identity.IdentityClient
	ObjectStorageClient     objectstorage.ObjectStorageClient
	ContainerEngineClient   containerengine.ContainerEngineClient
	LoadBalancerClient      loadbalancer.LoadBalancerClient
	DatabaseClient          database.DatabaseClient
}

type ResourceInfo struct {
	ResourceType   string                 `json:"resource_type"`
	ResourceName   string                 `json:"resource_name"`
	OCID          string                 `json:"ocid"`
	CompartmentID string                 `json:"compartment_id"`
	AdditionalInfo map[string]interface{} `json:"additional_info"`
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

	// Initialize Object Storage client
	osClient, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create object storage client: %w", err)
	}
	clients.ObjectStorageClient = osClient

	// Initialize Container Engine client (OKE)
	ceClient, err := containerengine.NewContainerEngineClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create container engine client: %w", err)
	}
	clients.ContainerEngineClient = ceClient

	// Initialize Load Balancer client
	lbClient, err := loadbalancer.NewLoadBalancerClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer client: %w", err)
	}
	clients.LoadBalancerClient = lbClient

	// Initialize Database client
	dbClient, err := database.NewDatabaseClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create database client: %w", err)
	}
	clients.DatabaseClient = dbClient

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
			
			additionalInfo := make(map[string]interface{})
			
			// Get primary IP address
			if instance.Id != nil {
				vnicReq := core.ListVnicAttachmentsRequest{
					CompartmentId: common.String(compartmentID),
					InstanceId:    instance.Id,
				}
				if vnicResp, err := clients.ComputeClient.ListVnicAttachments(ctx, vnicReq); err == nil {
					for _, vnicAttachment := range vnicResp.Items {
						if vnicAttachment.VnicId != nil {
							vnicDetailReq := core.GetVnicRequest{
								VnicId: vnicAttachment.VnicId,
							}
							if vnicDetailResp, err := clients.VirtualNetworkClient.GetVnic(ctx, vnicDetailReq); err == nil {
								if vnicDetailResp.PrivateIp != nil {
									additionalInfo["primary_ip"] = *vnicDetailResp.PrivateIp
									break
								}
							}
						}
					}
				}
			}
			
			// Add shape information
			if instance.Shape != nil {
				additionalInfo["shape"] = *instance.Shape
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "compute_instance",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
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
			
			additionalInfo := make(map[string]interface{})
			
			// Add CIDR blocks
			if vcn.CidrBlocks != nil && len(vcn.CidrBlocks) > 0 {
				additionalInfo["cidr_blocks"] = vcn.CidrBlocks
			}
			
			// Add DNS label
			if vcn.DnsLabel != nil {
				additionalInfo["dns_label"] = *vcn.DnsLabel
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "vcn",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
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
			
			additionalInfo := make(map[string]interface{})
			
			// Add CIDR information
			if subnet.CidrBlock != nil {
				additionalInfo["cidr"] = *subnet.CidrBlock
			}
			
			// Add availability domain
			if subnet.AvailabilityDomain != nil {
				additionalInfo["availability_domain"] = *subnet.AvailabilityDomain
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "subnet",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
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
			
			additionalInfo := make(map[string]interface{})
			
			// Add volume size
			if volume.SizeInGBs != nil {
				additionalInfo["size_gb"] = *volume.SizeInGBs
			}
			
			// Add volume performance tier
			if volume.VpusPerGB != nil {
				additionalInfo["vpus_per_gb"] = *volume.VpusPerGB
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "block_volume",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
		}
	}

	return resources, nil
}

func discoverObjectStorageBuckets(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	// Get namespace
	namespaceReq := objectstorage.GetNamespaceRequest{}
	namespaceResp, err := clients.ObjectStorageClient.GetNamespace(ctx, namespaceReq)
	if err != nil {
		return nil, err
	}

	req := objectstorage.ListBucketsRequest{
		CompartmentId: common.String(compartmentID),
		NamespaceName: namespaceResp.Value,
	}

	resp, err := clients.ObjectStorageClient.ListBuckets(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, bucket := range resp.Items {
		additionalInfo := make(map[string]interface{})
		// Storage tier is not available in BucketSummary
		
		resources = append(resources, ResourceInfo{
			ResourceType:   "object_storage_bucket",
			ResourceName:   *bucket.Name,
			OCID:          "", // Buckets don't have OCIDs
			CompartmentID: compartmentID,
			AdditionalInfo: additionalInfo,
		})
	}

	return resources, nil
}

func discoverOKEClusters(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	req := containerengine.ListClustersRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.ContainerEngineClient.ListClusters(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, cluster := range resp.Items {
		if cluster.LifecycleState != containerengine.ClusterSummaryLifecycleStateDeleted {
			name := ""
			if cluster.Name != nil {
				name = *cluster.Name
			}
			ocid := ""
			if cluster.Id != nil {
				ocid = *cluster.Id
			}

			additionalInfo := make(map[string]interface{})
			if cluster.KubernetesVersion != nil {
				additionalInfo["kubernetes_version"] = *cluster.KubernetesVersion
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "oke_cluster",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
		}
	}

	return resources, nil
}

func discoverLoadBalancers(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	req := loadbalancer.ListLoadBalancersRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.LoadBalancerClient.ListLoadBalancers(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, lb := range resp.Items {
		if lb.LifecycleState != loadbalancer.LoadBalancerLifecycleStateDeleted {
			name := ""
			if lb.DisplayName != nil {
				name = *lb.DisplayName
			}
			ocid := ""
			if lb.Id != nil {
				ocid = *lb.Id
			}

			additionalInfo := make(map[string]interface{})
			if lb.ShapeName != nil {
				additionalInfo["shape"] = *lb.ShapeName
			}
			if lb.IpAddresses != nil && len(lb.IpAddresses) > 0 {
				var ipAddresses []string
				for _, ip := range lb.IpAddresses {
					if ip.IpAddress != nil {
						ipAddresses = append(ipAddresses, *ip.IpAddress)
					}
				}
				additionalInfo["ip_addresses"] = ipAddresses
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "load_balancer",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
		}
	}

	return resources, nil
}

func discoverDatabases(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	req := database.ListDbSystemsRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.DatabaseClient.ListDbSystems(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, dbSystem := range resp.Items {
		if dbSystem.LifecycleState != database.DbSystemSummaryLifecycleStateTerminated {
			name := ""
			if dbSystem.DisplayName != nil {
				name = *dbSystem.DisplayName
			}
			ocid := ""
			if dbSystem.Id != nil {
				ocid = *dbSystem.Id
			}

			additionalInfo := make(map[string]interface{})
			if dbSystem.Shape != nil {
				additionalInfo["shape"] = *dbSystem.Shape
			}
			// DatabaseEdition is available in DbSystemSummary
			additionalInfo["database_edition"] = string(dbSystem.DatabaseEdition)
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "database_system",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
		}
	}

	return resources, nil
}

func discoverDRGs(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	req := core.ListDrgsRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.VirtualNetworkClient.ListDrgs(ctx, req)
	if err != nil {
		return nil, err
	}

	for _, drg := range resp.Items {
		if drg.LifecycleState != core.DrgLifecycleStateTerminated {
			name := ""
			if drg.DisplayName != nil {
				name = *drg.DisplayName
			}
			ocid := ""
			if drg.Id != nil {
				ocid = *drg.Id
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "drg",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: make(map[string]interface{}),
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
	fmt.Fprintf(os.Stderr, "Getting compartments...\n")
	compartments, err := getCompartments(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("failed to get compartments: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Found %d compartments\n", len(compartments))

	// Discover resources in each compartment
	for i, compartment := range compartments {
		compartmentID := *compartment.Id
		compartmentName := "root"
		if compartment.Name != nil {
			compartmentName = *compartment.Name
		}
		
		fmt.Fprintf(os.Stderr, "Processing compartment %d/%d: %s\n", i+1, len(compartments), compartmentName)

		// Discover compute instances
		fmt.Fprintf(os.Stderr, "  Discovering compute instances...\n")
		if instances, err := discoverComputeInstances(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, instances...)
			fmt.Fprintf(os.Stderr, "  Found %d compute instances\n", len(instances))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover compute instances: %v\n", err)
		}

		// Discover VCNs
		fmt.Fprintf(os.Stderr, "  Discovering VCNs...\n")
		if vcns, err := discoverVCNs(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, vcns...)
			fmt.Fprintf(os.Stderr, "  Found %d VCNs\n", len(vcns))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover VCNs: %v\n", err)
		}

		// Discover subnets
		fmt.Fprintf(os.Stderr, "  Discovering subnets...\n")
		if subnets, err := discoverSubnets(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, subnets...)
			fmt.Fprintf(os.Stderr, "  Found %d subnets\n", len(subnets))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover subnets: %v\n", err)
		}

		// Discover block volumes
		fmt.Fprintf(os.Stderr, "  Discovering block volumes...\n")
		if volumes, err := discoverBlockVolumes(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, volumes...)
			fmt.Fprintf(os.Stderr, "  Found %d block volumes\n", len(volumes))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover block volumes: %v\n", err)
		}

		// Discover Object Storage buckets
		fmt.Fprintf(os.Stderr, "  Discovering Object Storage buckets...\n")
		if buckets, err := discoverObjectStorageBuckets(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, buckets...)
			fmt.Fprintf(os.Stderr, "  Found %d Object Storage buckets\n", len(buckets))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover Object Storage buckets: %v\n", err)
		}

		// Discover OKE clusters
		fmt.Fprintf(os.Stderr, "  Discovering OKE clusters...\n")
		if clusters, err := discoverOKEClusters(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, clusters...)
			fmt.Fprintf(os.Stderr, "  Found %d OKE clusters\n", len(clusters))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover OKE clusters: %v\n", err)
		}

		// Discover Load Balancers
		fmt.Fprintf(os.Stderr, "  Discovering Load Balancers...\n")
		if lbs, err := discoverLoadBalancers(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, lbs...)
			fmt.Fprintf(os.Stderr, "  Found %d Load Balancers\n", len(lbs))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover Load Balancers: %v\n", err)
		}

		// Discover Database Systems
		fmt.Fprintf(os.Stderr, "  Discovering Database Systems...\n")
		if dbs, err := discoverDatabases(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, dbs...)
			fmt.Fprintf(os.Stderr, "  Found %d Database Systems\n", len(dbs))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover Database Systems: %v\n", err)
		}

		// Discover DRGs
		fmt.Fprintf(os.Stderr, "  Discovering DRGs...\n")
		if drgs, err := discoverDRGs(ctx, clients, compartmentID); err == nil {
			allResources = append(allResources, drgs...)
			fmt.Fprintf(os.Stderr, "  Found %d DRGs\n", len(drgs))
		} else if !isRetriableError(err) {
			fmt.Fprintf(os.Stderr, "  Warning: Failed to discover DRGs: %v\n", err)
		}
	}

	fmt.Fprintf(os.Stderr, "Discovery completed. Total resources found: %d\n", len(allResources))
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
	header := []string{"ResourceType", "ResourceName", "OCID", "CompartmentID", "AdditionalInfo"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, resource := range resources {
		additionalInfoJSON, _ := json.Marshal(resource.AdditionalInfo)
		record := []string{
			resource.ResourceType,
			resource.ResourceName,
			resource.OCID,
			resource.CompartmentID,
			string(additionalInfoJSON),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func outputTSV(resources []ResourceInfo) error {
	// Write header
	fmt.Println("ResourceType\tResourceName\tOCID\tCompartmentID\tAdditionalInfo")

	// Write data
	for _, resource := range resources {
		additionalInfoJSON, _ := json.Marshal(resource.AdditionalInfo)
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n",
			resource.ResourceType,
			resource.ResourceName,
			resource.OCID,
			resource.CompartmentID,
			string(additionalInfoJSON),
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