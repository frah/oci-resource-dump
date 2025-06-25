package main

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/apigateway"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
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

// initOCIClients initializes all required OCI service clients with context support
func initOCIClients(ctx context.Context) (*OCIClients, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use instance principal authentication with timeout control
	type configProviderResult struct {
		provider common.ConfigurationProvider
		err      error
	}
	configProviderChan := make(chan configProviderResult, 1)

	go func() {
		provider, err := auth.InstancePrincipalConfigurationProvider()
		configProviderChan <- configProviderResult{provider: provider, err: err}
	}()

	var configProvider common.ConfigurationProvider
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-configProviderChan:
		if result.err != nil {
			return nil, fmt.Errorf("failed to create instance principal config provider: %w", result.err)
		}
		configProvider = result.provider
	}

	clients := &OCIClients{}

	// Helper function to initialize client with timeout
	initClientWithTimeout := func(clientName string, initFunc func() (interface{}, error)) (interface{}, error) {
		type clientResult struct {
			client interface{}
			err    error
		}
		clientChan := make(chan clientResult, 1)

		go func() {
			client, err := initFunc()
			clientChan <- clientResult{client: client, err: err}
		}()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-clientChan:
			if result.err != nil {
				return nil, fmt.Errorf("failed to create %s client: %w", clientName, result.err)
			}
			return result.client, nil
		}
	}

	// Initialize Compute client
	computeInterface, err := initClientWithTimeout("compute", func() (interface{}, error) {
		return core.NewComputeClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.ComputeClient = computeInterface.(core.ComputeClient)

	// Initialize VirtualNetwork client
	vnInterface, err := initClientWithTimeout("virtual network", func() (interface{}, error) {
		return core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.VirtualNetworkClient = vnInterface.(core.VirtualNetworkClient)

	// Initialize BlockStorage client
	bsInterface, err := initClientWithTimeout("block storage", func() (interface{}, error) {
		return core.NewBlockstorageClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.BlockStorageClient = bsInterface.(core.BlockstorageClient)

	// Initialize Identity client
	identityInterface, err := initClientWithTimeout("identity", func() (interface{}, error) {
		return identity.NewIdentityClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.IdentityClient = identityInterface.(identity.IdentityClient)

	// Initialize Object Storage client
	osInterface, err := initClientWithTimeout("object storage", func() (interface{}, error) {
		return objectstorage.NewObjectStorageClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.ObjectStorageClient = osInterface.(objectstorage.ObjectStorageClient)

	// Initialize Container Engine client (OKE)
	ceInterface, err := initClientWithTimeout("container engine", func() (interface{}, error) {
		return containerengine.NewContainerEngineClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.ContainerEngineClient = ceInterface.(containerengine.ContainerEngineClient)

	// Initialize Load Balancer client
	lbInterface, err := initClientWithTimeout("load balancer", func() (interface{}, error) {
		return loadbalancer.NewLoadBalancerClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.LoadBalancerClient = lbInterface.(loadbalancer.LoadBalancerClient)

	// Initialize Database client
	dbInterface, err := initClientWithTimeout("database", func() (interface{}, error) {
		return database.NewDatabaseClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.DatabaseClient = dbInterface.(database.DatabaseClient)

	// Initialize API Gateway client
	apiGatewayInterface, err := initClientWithTimeout("api gateway", func() (interface{}, error) {
		return apigateway.NewGatewayClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.APIGatewayClient = apiGatewayInterface.(apigateway.GatewayClient)

	// Initialize Functions client
	functionsInterface, err := initClientWithTimeout("functions", func() (interface{}, error) {
		return functions.NewFunctionsManagementClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.FunctionsClient = functionsInterface.(functions.FunctionsManagementClient)

	// Initialize File Storage client
	fileStorageInterface, err := initClientWithTimeout("file storage", func() (interface{}, error) {
		return filestorage.NewFileStorageClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.FileStorageClient = fileStorageInterface.(filestorage.FileStorageClient)

	// Initialize Network Load Balancer client
	nlbInterface, err := initClientWithTimeout("network load balancer", func() (interface{}, error) {
		return networkloadbalancer.NewNetworkLoadBalancerClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.NetworkLoadBalancerClient = nlbInterface.(networkloadbalancer.NetworkLoadBalancerClient)

	// Initialize Streaming client
	streamingInterface, err := initClientWithTimeout("streaming", func() (interface{}, error) {
		return streaming.NewStreamAdminClientWithConfigurationProvider(configProvider)
	})
	if err != nil {
		return nil, err
	}
	clients.StreamingClient = streamingInterface.(streaming.StreamAdminClient)

	// Initialize Compartment Name Cache
	clients.CompartmentCache = NewCompartmentNameCache(clients.IdentityClient)

	// Final context check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return clients, nil
}

// getCompartments retrieves all accessible compartments in the tenancy with aggressive timeout control
func getCompartments(ctx context.Context, clients *OCIClients) ([]identity.Compartment, error) {
	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Get tenancy ID from the instance principal with timeout channel
	type configResult struct {
		provider common.ConfigurationProvider
		err      error
	}
	configChan := make(chan configResult, 1)

	go func() {
		provider, err := auth.InstancePrincipalConfigurationProvider()
		configChan <- configResult{provider: provider, err: err}
	}()

	var configProvider common.ConfigurationProvider
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-configChan:
		if result.err != nil {
			return nil, result.err
		}
		configProvider = result.provider
	}

	// Check context after config provider setup
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Get tenancy ID with timeout channel
	type tenancyResult struct {
		tenancyID string
		err       error
	}
	tenancyChan := make(chan tenancyResult, 1)

	go func() {
		tenancyID, err := configProvider.TenancyOCID()
		tenancyChan <- tenancyResult{tenancyID: tenancyID, err: err}
	}()

	var tenancyID string
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-tenancyChan:
		if result.err != nil {
			return nil, result.err
		}
		tenancyID = result.tenancyID
	}

	// Check context before API call
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// List compartments with explicit context deadline
	req := identity.ListCompartmentsRequest{
		CompartmentId: common.String(tenancyID),
		AccessLevel:   identity.ListCompartmentsAccessLevelAccessible,
	}

	// Execute API call with timeout channel for aggressive control
	type compartmentResult struct {
		resp identity.ListCompartmentsResponse
		err  error
	}
	compartmentChan := make(chan compartmentResult, 1)

	go func() {
		resp, err := clients.IdentityClient.ListCompartments(ctx, req)
		compartmentChan <- compartmentResult{resp: resp, err: err}
	}()

	var resp identity.ListCompartmentsResponse
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-compartmentChan:
		if result.err != nil {
			return nil, result.err
		}
		resp = result.resp
	}

	// Final context check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
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
