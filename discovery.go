package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/apigateway"
	"github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/filestorage"
	"github.com/oracle/oci-go-sdk/v65/functions"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/oracle/oci-go-sdk/v65/streaming"
)

// createResourceInfo creates a ResourceInfo with optimized compartment name resolution
func createResourceInfo(ctx context.Context, resourceType, resourceName, ocid, compartmentID string, additionalInfo map[string]interface{}, cache *CompartmentNameCache) ResourceInfo {
	// Optimized compartment name lookup with context timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	compartmentName := cache.GetCompartmentName(ctxWithTimeout, compartmentID)
	
	return ResourceInfo{
		ResourceType:     resourceType,
		CompartmentName:  compartmentName,
		ResourceName:     resourceName,
		OCID:            ocid,
		CompartmentID:   compartmentID,
		AdditionalInfo:  additionalInfo,
	}
}

// isRetriableError checks if the error is a retriable error (non-existent resource, permission issue, etc.)
func isRetriableError(err error) bool {
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

// isTransientError checks if the error is transient and should be retried
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		   strings.Contains(errStr, "connection reset") ||
		   strings.Contains(errStr, "temporary failure") ||
		   strings.Contains(errStr, "service unavailable") ||
		   strings.Contains(errStr, "too many requests") ||
		   strings.Contains(errStr, "rate limit") ||
		   strings.Contains(errStr, "internal server error") ||
		   strings.Contains(errStr, "502") ||
		   strings.Contains(errStr, "503") ||
		   strings.Contains(errStr, "504")
}

// withRetryAndProgress executes an operation with retry logic and progress tracking
func withRetryAndProgress(ctx context.Context, operation func() error, maxRetries int, operationName string, progressTracker *ProgressTracker) error {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		
		// Don't retry if the error is not transient
		if !isTransientError(err) {
			return err
		}
		
		if attempt == maxRetries {
			return fmt.Errorf("operation '%s' failed after %d attempts: %w", operationName, maxRetries+1, err)
		}
		
		// Increment retry counter
		if progressTracker != nil {
			progressTracker.Update(ProgressUpdate{IsRetry: true})
		}
		
		// Exponential backoff with jitter (up to 30 seconds max)
		backoff := time.Duration(math.Min(math.Pow(2, float64(attempt)), 30)) * time.Second
		jitter := time.Duration(float64(backoff) * 0.1 * (2*rand.Float64() - 1))
		sleepTime := backoff + jitter
		if sleepTime < 0 {
			sleepTime = backoff
		}
		
		logger.Verbose("Retrying %s in %v (attempt %d/%d): %v", operationName, sleepTime, attempt+1, maxRetries+1, err)
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepTime):
		}
	}
	return nil
}

// withRetry executes an operation with retry logic for backward compatibility
func withRetry(ctx context.Context, operation func() error, maxRetries int, operationName string) error {
	return withRetryAndProgress(ctx, operation, maxRetries, operationName, nil)
}

// discoverComputeInstances discovers all compute instances in a compartment
func discoverComputeInstances(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allInstances []core.Instance

	logger.Debug("Starting compute instances discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all instances
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching compute instances page %d for compartment: %s", pageCount, compartmentID)
		req := core.ListInstancesRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.ComputeClient.ListInstances(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allInstances = append(allInstances, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, instance := range allInstances {
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
				
				vnicResp, err := clients.ComputeClient.ListVnicAttachments(ctx, vnicReq)
				if err == nil && len(vnicResp.Items) > 0 {
					for _, vnicAttachment := range vnicResp.Items {
						if vnicAttachment.VnicId != nil && vnicAttachment.LifecycleState == core.VnicAttachmentLifecycleStateAttached {
							vnicDetailsReq := core.GetVnicRequest{
								VnicId: vnicAttachment.VnicId,
							}
							vnicDetailsResp, err := clients.VirtualNetworkClient.GetVnic(ctx, vnicDetailsReq)
							if err == nil && vnicDetailsResp.Vnic.IsPrimary != nil && *vnicDetailsResp.Vnic.IsPrimary {
								if vnicDetailsResp.Vnic.PrivateIp != nil {
									additionalInfo["primary_ip"] = *vnicDetailsResp.Vnic.PrivateIp
								}
								break
							}
						}
					}
				}
			}
			
			// Add shape information
			if instance.Shape != nil {
				additionalInfo["shape"] = *instance.Shape
			}
			
			resources = append(resources, createResourceInfo(ctx, "ComputeInstance", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d compute instances in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverVCNs discovers all Virtual Cloud Networks in a compartment
func discoverVCNs(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allVcns []core.Vcn

	logger.Debug("Starting VCN discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all VCNs
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching VCNs page %d for compartment: %s", pageCount, compartmentID)
		req := core.ListVcnsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.VirtualNetworkClient.ListVcns(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allVcns = append(allVcns, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, vcn := range allVcns {
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
			if len(vcn.CidrBlocks) > 0 {
				additionalInfo["cidr_blocks"] = vcn.CidrBlocks
			}
			
			// Add DNS label
			if vcn.DnsLabel != nil {
				additionalInfo["dns_label"] = *vcn.DnsLabel
			}
			
			resources = append(resources, createResourceInfo(ctx, "VCN", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d VCNs in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverSubnets discovers all subnets in a compartment
func discoverSubnets(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allSubnets []core.Subnet

	logger.Debug("Starting subnet discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all subnets
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching subnets page %d for compartment: %s", pageCount, compartmentID)
		req := core.ListSubnetsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.VirtualNetworkClient.ListSubnets(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allSubnets = append(allSubnets, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, subnet := range allSubnets {
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
			
			// Add CIDR block
			if subnet.CidrBlock != nil {
				additionalInfo["cidr_block"] = *subnet.CidrBlock
			}
			
			// Add availability domain
			if subnet.AvailabilityDomain != nil {
				additionalInfo["availability_domain"] = *subnet.AvailabilityDomain
			}
			
			resources = append(resources, createResourceInfo(ctx, "Subnet", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d subnets in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverBlockVolumes discovers all block volumes in a compartment
func discoverBlockVolumes(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allVolumes []core.Volume

	logger.Debug("Starting block volume discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all volumes
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching block volumes page %d for compartment: %s", pageCount, compartmentID)
		req := core.ListVolumesRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.BlockStorageClient.ListVolumes(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allVolumes = append(allVolumes, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, volume := range allVolumes {
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
			
			// Add size in GBs
			if volume.SizeInGBs != nil {
				additionalInfo["size_in_gbs"] = *volume.SizeInGBs
			}
			
			// Add volume performance (VPUs per GB)
			if volume.VpusPerGB != nil {
				additionalInfo["vpus_per_gb"] = *volume.VpusPerGB
			}
			
			resources = append(resources, createResourceInfo(ctx, "BlockVolume", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d block volumes in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverObjectStorageBuckets discovers all object storage buckets in a compartment
func discoverObjectStorageBuckets(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	logger.Debug("Starting object storage bucket discovery for compartment: %s", compartmentID)

	// Get the namespace first
	req := objectstorage.GetNamespaceRequest{}
	resp, err := clients.ObjectStorageClient.GetNamespace(ctx, req)
	if err != nil {
		return nil, err
	}

	namespace := *resp.Value

	// List buckets
	listReq := objectstorage.ListBucketsRequest{
		NamespaceName: common.String(namespace),
		CompartmentId: common.String(compartmentID),
	}

	listResp, err := clients.ObjectStorageClient.ListBuckets(ctx, listReq)
	if err != nil {
		return nil, err
	}

	for _, bucket := range listResp.Items {
		name := ""
		if bucket.Name != nil {
			name = *bucket.Name
		}
		
		additionalInfo := make(map[string]interface{})
		additionalInfo["namespace"] = namespace
		
		// Note: Storage tier is not available in BucketSummary
		
		// Note: Object Storage buckets don't have traditional OCIDs like other resources
		// The bucket name serves as the identifier
		resources = append(resources, createResourceInfo(ctx, "ObjectStorageBucket", name, fmt.Sprintf("bucket:%s:%s", namespace, name), compartmentID, additionalInfo, clients.CompartmentCache))
	}

	logger.Verbose("Found %d object storage buckets in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverOKEClusters discovers all OKE clusters in a compartment
func discoverOKEClusters(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allClusters []containerengine.ClusterSummary

	logger.Debug("Starting OKE cluster discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all clusters
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching OKE clusters page %d for compartment: %s", pageCount, compartmentID)
		req := containerengine.ListClustersRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.ContainerEngineClient.ListClusters(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allClusters = append(allClusters, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, cluster := range allClusters {
		if cluster.LifecycleState != containerengine.ClusterLifecycleStateDeleted {
			name := ""
			if cluster.Name != nil {
				name = *cluster.Name
			}
			ocid := ""
			if cluster.Id != nil {
				ocid = *cluster.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			// Add Kubernetes version
			if cluster.KubernetesVersion != nil {
				additionalInfo["kubernetes_version"] = *cluster.KubernetesVersion
			}
			
			resources = append(resources, createResourceInfo(ctx, "OKECluster", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d OKE clusters in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverLoadBalancers discovers all load balancers in a compartment
func discoverLoadBalancers(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allLoadBalancers []loadbalancer.LoadBalancer

	logger.Debug("Starting load balancer discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all load balancers
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching load balancers page %d for compartment: %s", pageCount, compartmentID)
		req := loadbalancer.ListLoadBalancersRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.LoadBalancerClient.ListLoadBalancers(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allLoadBalancers = append(allLoadBalancers, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, lb := range allLoadBalancers {
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
			
			// Add shape
			if lb.ShapeName != nil {
				additionalInfo["shape"] = *lb.ShapeName
			}
			
			// Add IP addresses
			if len(lb.IpAddresses) > 0 {
				var ipAddresses []string
				for _, ip := range lb.IpAddresses {
					if ip.IpAddress != nil {
						ipAddresses = append(ipAddresses, *ip.IpAddress)
					}
				}
				additionalInfo["ip_addresses"] = ipAddresses
			}
			
			resources = append(resources, createResourceInfo(ctx, "LoadBalancer", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d load balancers in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverDatabases discovers all database systems in a compartment
func discoverDatabases(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allDbSystems []database.DbSystemSummary

	logger.Debug("Starting database system discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all database systems
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching database systems page %d for compartment: %s", pageCount, compartmentID)
		req := database.ListDbSystemsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.DatabaseClient.ListDbSystems(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allDbSystems = append(allDbSystems, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, dbSystem := range allDbSystems {
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
			
			// Add shape
			if dbSystem.Shape != nil {
				additionalInfo["shape"] = *dbSystem.Shape
			}
			
			// Add database edition
			additionalInfo["database_edition"] = string(dbSystem.DatabaseEdition)
			
			resources = append(resources, createResourceInfo(ctx, 
				"DatabaseSystem", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d database systems in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverDRGs discovers all Dynamic Routing Gateways in a compartment
func discoverDRGs(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allDrgs []core.Drg

	logger.Debug("Starting DRG discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all DRGs
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching DRGs page %d for compartment: %s", pageCount, compartmentID)
		req := core.ListDrgsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.VirtualNetworkClient.ListDrgs(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allDrgs = append(allDrgs, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, drg := range allDrgs {
		if drg.LifecycleState != core.DrgLifecycleStateTerminated {
			name := ""
			if drg.DisplayName != nil {
				name = *drg.DisplayName
			}
			ocid := ""
			if drg.Id != nil {
				ocid = *drg.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			resources = append(resources, createResourceInfo(ctx, "DRG", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d DRGs in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverAutonomousDatabases discovers all autonomous databases in a compartment
func discoverAutonomousDatabases(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allAutonomousDBs []database.AutonomousDatabaseSummary

	logger.Debug("Starting autonomous database discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all autonomous databases
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching autonomous databases page %d for compartment: %s", pageCount, compartmentID)
		req := database.ListAutonomousDatabasesRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.DatabaseClient.ListAutonomousDatabases(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allAutonomousDBs = append(allAutonomousDBs, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, autonomousDB := range allAutonomousDBs {
		if autonomousDB.LifecycleState != database.AutonomousDatabaseSummaryLifecycleStateTerminated {
			name := ""
			if autonomousDB.DisplayName != nil {
				name = *autonomousDB.DisplayName
			}
			ocid := ""
			if autonomousDB.Id != nil {
				ocid = *autonomousDB.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			// Add workload type
			additionalInfo["workload_type"] = string(autonomousDB.DbWorkload)
			
			// Add CPU core count
			if autonomousDB.CpuCoreCount != nil {
				additionalInfo["cpu_core_count"] = *autonomousDB.CpuCoreCount
			}
			
			// Add data storage size
			if autonomousDB.DataStorageSizeInTBs != nil {
				additionalInfo["data_storage_size_in_tbs"] = *autonomousDB.DataStorageSizeInTBs
			}
			
			resources = append(resources, createResourceInfo(ctx, "AutonomousDatabase", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d autonomous databases in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverFunctions discovers all functions in a compartment
func discoverFunctions(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	logger.Debug("Starting functions discovery for compartment: %s", compartmentID)

	// First, get all applications
	var allApplications []functions.ApplicationSummary
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching function applications page %d for compartment: %s", pageCount, compartmentID)
		req := functions.ListApplicationsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.FunctionsClient.ListApplications(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allApplications = append(allApplications, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	// Then, get all functions for each application
	for _, app := range allApplications {
		if app.LifecycleState != functions.ApplicationLifecycleStateDeleted {
			var allFunctions []functions.FunctionSummary
			var funcPage *string
			funcPageCount := 0
			for {
				funcPageCount++
				logger.Debug("Fetching functions for application %s, page %d", *app.DisplayName, funcPageCount)
				funcReq := functions.ListFunctionsRequest{
					ApplicationId: app.Id,
					Page:         funcPage,
				}

				funcResp, err := clients.FunctionsClient.ListFunctions(ctx, funcReq)
				
				if err != nil {
					logger.Verbose("Error listing functions for application %s: %v", *app.DisplayName, err)
					break
				}

				allFunctions = append(allFunctions, funcResp.Items...)

				if funcResp.OpcNextPage == nil {
					break
				}
				funcPage = funcResp.OpcNextPage
			}

			for _, function := range allFunctions {
				if function.LifecycleState != functions.FunctionLifecycleStateDeleted {
					name := ""
					if function.DisplayName != nil {
						name = *function.DisplayName
					}
					ocid := ""
					if function.Id != nil {
						ocid = *function.Id
					}
					
					additionalInfo := make(map[string]interface{})
					
					// Add application name
					if app.DisplayName != nil {
						additionalInfo["application_name"] = *app.DisplayName
					}
					
					// Add runtime
					if function.Image != nil {
						additionalInfo["image"] = *function.Image
					}
					
					// Add memory in MBs
					if function.MemoryInMBs != nil {
						additionalInfo["memory_in_mbs"] = *function.MemoryInMBs
					}
					
					resources = append(resources, createResourceInfo(ctx, "Function", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
				}
			}
		}
	}

	logger.Verbose("Found %d functions in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverAPIGateways discovers all API gateways in a compartment
func discoverAPIGateways(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allGateways []apigateway.GatewaySummary

	logger.Debug("Starting API gateway discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all API gateways
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching API gateways page %d for compartment: %s", pageCount, compartmentID)
		req := apigateway.ListGatewaysRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.APIGatewayClient.ListGateways(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allGateways = append(allGateways, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, gateway := range allGateways {
		if gateway.LifecycleState != apigateway.GatewayLifecycleStateDeleted {
			name := ""
			if gateway.DisplayName != nil {
				name = *gateway.DisplayName
			}
			ocid := ""
			if gateway.Id != nil {
				ocid = *gateway.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			// Note: Endpoint is not available in GatewaySummary, would need to fetch gateway details
			
			// Note: Would need to use different API client to get deployment information
			
			resources = append(resources, createResourceInfo(ctx, "APIGateway", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d API gateways in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// getAvailabilityDomains retrieves all availability domains for a compartment
func getAvailabilityDomains(ctx context.Context, clients *OCIClients, compartmentID string) ([]identity.AvailabilityDomain, error) {
	logger.Debug("Getting availability domains for compartment: %s", compartmentID)
	
	req := identity.ListAvailabilityDomainsRequest{
		CompartmentId: common.String(compartmentID),
	}

	resp, err := clients.IdentityClient.ListAvailabilityDomains(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get availability domains: %w", err)
	}

	logger.Debug("Found %d availability domains", len(resp.Items))
	return resp.Items, nil
}

// discoverFileStorageSystems discovers all file storage systems in a compartment
func discoverFileStorageSystems(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo

	logger.Debug("Starting file storage system discovery for compartment: %s", compartmentID)
	
	// Get all availability domains for this compartment
	availabilityDomains, err := getAvailabilityDomains(ctx, clients, compartmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get availability domains: %w", err)
	}

	// Search file systems in each availability domain
	for _, ad := range availabilityDomains {
		if ad.Name == nil {
			continue
		}

		adName := *ad.Name
		logger.Debug("Searching file systems in availability domain: %s", adName)

		var allFileSystems []filestorage.FileSystemSummary
		
		// Implement pagination to get all file systems in this AD
		var page *string
		pageCount := 0
		for {
			pageCount++
			logger.Debug("Fetching file systems page %d for compartment: %s, AD: %s", pageCount, compartmentID, adName)
			req := filestorage.ListFileSystemsRequest{
				CompartmentId:      common.String(compartmentID),
				AvailabilityDomain: common.String(adName),
				Page:              page,
			}

			resp, err := clients.FileStorageClient.ListFileSystems(ctx, req)
			
			if err != nil {
				logger.Verbose("Error listing file systems in AD %s: %v", adName, err)
				break // Continue with next AD instead of failing completely
			}

			allFileSystems = append(allFileSystems, resp.Items...)

			if resp.OpcNextPage == nil {
				break
			}
			page = resp.OpcNextPage
		}

		// Process file systems found in this AD
		for _, fileSystem := range allFileSystems {
			if fileSystem.LifecycleState != filestorage.FileSystemSummaryLifecycleStateDeleted {
				name := ""
				if fileSystem.DisplayName != nil {
					name = *fileSystem.DisplayName
				}
				ocid := ""
				if fileSystem.Id != nil {
					ocid = *fileSystem.Id
				}
				
				additionalInfo := make(map[string]interface{})
				
				// Add metered bytes (current storage usage)
				if fileSystem.MeteredBytes != nil {
					sizeInGB := float64(*fileSystem.MeteredBytes) / (1024 * 1024 * 1024)
					additionalInfo["size_in_gb"] = fmt.Sprintf("%.2f", sizeInGB)
				}
				
				// Add availability domain
				additionalInfo["availability_domain"] = adName
				
				resources = append(resources, createResourceInfo(ctx, "FileStorageSystem", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
			}
		}
	}

	logger.Verbose("Found %d file storage systems in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverNetworkLoadBalancers discovers all network load balancers in a compartment
func discoverNetworkLoadBalancers(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allNLBs []networkloadbalancer.NetworkLoadBalancerSummary

	logger.Debug("Starting network load balancer discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all network load balancers
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching network load balancers page %d for compartment: %s", pageCount, compartmentID)
		req := networkloadbalancer.ListNetworkLoadBalancersRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.NetworkLoadBalancerClient.ListNetworkLoadBalancers(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allNLBs = append(allNLBs, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, nlb := range allNLBs {
		if nlb.LifecycleState != networkloadbalancer.LifecycleStateDeleted {
			name := ""
			if nlb.DisplayName != nil {
				name = *nlb.DisplayName
			}
			ocid := ""
			if nlb.Id != nil {
				ocid = *nlb.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			// Note: Health status not available in NetworkLoadBalancerSummary
			
			// Add IP addresses
			if len(nlb.IpAddresses) > 0 {
				var ipAddresses []string
				for _, ip := range nlb.IpAddresses {
					if ip.IpAddress != nil {
						ipAddresses = append(ipAddresses, *ip.IpAddress)
					}
				}
				additionalInfo["ip_addresses"] = ipAddresses
			}
			
			resources = append(resources, createResourceInfo(ctx, "NetworkLoadBalancer", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d network load balancers in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverStreams discovers all streams in a compartment
func discoverStreams(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allStreams []streaming.StreamSummary

	logger.Debug("Starting stream discovery for compartment: %s", compartmentID)
	
	// Implement pagination to get all streams
	var page *string
	pageCount := 0
	for {
		pageCount++
		logger.Debug("Fetching streams page %d for compartment: %s", pageCount, compartmentID)
		req := streaming.ListStreamsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.StreamingClient.ListStreams(ctx, req)
		
		if err != nil {
			return nil, err
		}

		allStreams = append(allStreams, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, stream := range allStreams {
		if stream.LifecycleState != streaming.StreamSummaryLifecycleStateDeleted {
			name := ""
			if stream.Name != nil {
				name = *stream.Name
			}
			ocid := ""
			if stream.Id != nil {
				ocid = *stream.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			// Add partitions
			if stream.Partitions != nil {
				additionalInfo["partitions"] = *stream.Partitions
			}
			
			// Get stream details for more information
			if stream.Id != nil {
				getReq := streaming.GetStreamRequest{
					StreamId: stream.Id,
				}
				getResp, err := clients.StreamingClient.GetStream(ctx, getReq)
				if err == nil {
					// Add retention in hours
					if getResp.Stream.RetentionInHours != nil {
						additionalInfo["retention_in_hours"] = *getResp.Stream.RetentionInHours
					}
				}
			}
			
			resources = append(resources, createResourceInfo(ctx, "Stream", name, ocid, compartmentID, additionalInfo, clients.CompartmentCache))
		}
	}

	logger.Verbose("Found %d streams in compartment %s", len(resources), compartmentID)
	return resources, nil
}

// discoverAllResourcesWithProgress coordinates the discovery of all resource types with progress tracking
func discoverAllResourcesWithProgress(ctx context.Context, clients *OCIClients, progressTracker *ProgressTracker, filters FilterConfig) ([]ResourceInfo, error) {
	var allResources []ResourceInfo

	// Get list of compartments
	compartments, err := getCompartments(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("failed to get compartments: %w", err)
	}

	// Apply compartment filters
	filteredCompartments := ApplyCompartmentFilter(compartments, filters)
	logger.Info("Found %d compartments to process (filtered from %d)", len(filteredCompartments), len(compartments))

	// Compile filter regex patterns for efficient matching
	compiledFilters, err := CompileFilters(filters)
	if err != nil {
		return nil, fmt.Errorf("failed to compile filter patterns: %w", err)
	}

	// Update progress tracker with compartment count
	if progressTracker != nil {
		progressTracker.totalCompartments = int64(len(filteredCompartments))
		progressTracker.totalResourceTypes = 15 // Number of resource types we discover
		progressTracker.Start()
		defer progressTracker.Stop()
	}

	// Use a semaphore to limit concurrent compartments (max 5)
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var discoveryErrors []string

	// Discovery functions map
	discoveryFuncs := map[string]func(context.Context, *OCIClients, string) ([]ResourceInfo, error){
		"ComputeInstances":      discoverComputeInstances,
		"VCNs":                  discoverVCNs,
		"Subnets":               discoverSubnets,
		"BlockVolumes":          discoverBlockVolumes,
		"ObjectStorageBuckets":  discoverObjectStorageBuckets,
		"OKEClusters":           discoverOKEClusters,
		"LoadBalancers":         discoverLoadBalancers,
		"DatabaseSystems":       discoverDatabases,
		"DRGs":                  discoverDRGs,
		"AutonomousDatabases":   discoverAutonomousDatabases,
		"Functions":             discoverFunctions,
		"APIGateways":           discoverAPIGateways,
		"FileStorageSystems":    discoverFileStorageSystems,
		"NetworkLoadBalancers":  discoverNetworkLoadBalancers,
		"Streams":               discoverStreams,
	}

	for _, compartment := range filteredCompartments {
		if compartment.LifecycleState != "ACTIVE" {
			continue
		}

		wg.Add(1)
		go func(comp string, compName string) {
			defer wg.Done()
			
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			logger.Verbose("Processing compartment: %s (%s)", compName, comp)

			// Process each resource type for this compartment
			for resourceType, discoveryFunc := range discoveryFuncs {
				// Apply resource type filter
				if !ApplyResourceTypeFilter(resourceType, filters) {
					logger.Debug("Skipping resource type %s due to filters", resourceType)
					continue
				}
				// Update progress
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compName,
						Operation:      resourceType,
					})
				}

				var resources []ResourceInfo
				var err error

				// Execute discovery with retry
				operation := func() error {
					resources, err = discoveryFunc(ctx, clients, comp)
					return err
				}

				retryErr := withRetryAndProgress(ctx, operation, 3, fmt.Sprintf("%s in %s", resourceType, compName), progressTracker)
				
				if retryErr != nil {
					if isRetriableError(retryErr) {
						logger.Verbose("Skipping %s in compartment %s due to retriable error: %v", resourceType, compName, retryErr)
						if progressTracker != nil {
							progressTracker.Update(ProgressUpdate{IsError: true})
						}
					} else {
						errorMsg := fmt.Sprintf("Error discovering %s in compartment %s: %v", resourceType, compName, retryErr)
						logger.Verbose(errorMsg)
						mu.Lock()
						discoveryErrors = append(discoveryErrors, errorMsg)
						mu.Unlock()
						if progressTracker != nil {
							progressTracker.Update(ProgressUpdate{IsError: true})
						}
					}
					continue
				}

				// Apply name filters to discovered resources
				filteredResources := make([]ResourceInfo, 0, len(resources))
				for _, resource := range resources {
					if ApplyNameFilter(resource.ResourceName, compiledFilters) {
						filteredResources = append(filteredResources, resource)
					} else {
						logger.Debug("Filtering out resource %s due to name filters", resource.ResourceName)
					}
				}

				// Add filtered resources to the global list
				if len(filteredResources) > 0 {
					mu.Lock()
					allResources = append(allResources, filteredResources...)
					mu.Unlock()
					
					if progressTracker != nil {
						progressTracker.Update(ProgressUpdate{ResourceCount: int64(len(filteredResources))})
					}
				}
				
				if len(resources) > len(filteredResources) {
					logger.Verbose("Filtered %d resources by name in %s %s", len(resources)-len(filteredResources), resourceType, compName)
				}
			}

			// Mark compartment as complete
			if progressTracker != nil {
				progressTracker.Update(ProgressUpdate{
					CompartmentName:       compName,
					IsCompartmentComplete: true,
				})
			}

			logger.Verbose("Completed processing compartment: %s", compName)
		}(*compartment.Id, *compartment.Name)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Report discovery summary
	if len(discoveryErrors) > 0 {
		logger.Verbose("Discovery completed with %d errors:", len(discoveryErrors))
		for i, err := range discoveryErrors {
			if i < 5 { // Limit to first 5 errors
				logger.Verbose("  %s", err)
			}
		}
		if len(discoveryErrors) > 5 {
			logger.Verbose("  ... and %d more errors", len(discoveryErrors)-5)
		}
	}

	logger.Info("Resource discovery completed. Found %d resources across %d compartments", len(allResources), len(compartments))
	
	return allResources, nil
}