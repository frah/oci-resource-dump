package main

import (
	"sync"
	"time"

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

// Config holds the application configuration
type Config struct {
	OutputFormat string
	Timeout      time.Duration
	MaxWorkers   int
	LogLevel     LogLevel
	Logger       *Logger
	ShowProgress bool
	Filters      FilterConfig
}

// OCIClients holds all OCI service clients
type OCIClients struct {
	ComputeClient             core.ComputeClient
	VirtualNetworkClient      core.VirtualNetworkClient
	BlockStorageClient        core.BlockstorageClient
	IdentityClient            identity.IdentityClient
	ObjectStorageClient       objectstorage.ObjectStorageClient
	ContainerEngineClient     containerengine.ContainerEngineClient
	LoadBalancerClient        loadbalancer.LoadBalancerClient
	DatabaseClient            database.DatabaseClient
	APIGatewayClient          apigateway.GatewayClient
	FunctionsClient           functions.FunctionsManagementClient
	FileStorageClient         filestorage.FileStorageClient
	NetworkLoadBalancerClient networkloadbalancer.NetworkLoadBalancerClient
	StreamingClient           streaming.StreamAdminClient
	CompartmentCache          *CompartmentNameCache
}

// ResourceInfo represents a discovered OCI resource
type ResourceInfo struct {
	ResourceType    string                 `json:"resource_type"`
	CompartmentName string                 `json:"compartment_name"`
	ResourceName    string                 `json:"resource_name"`
	OCID            string                 `json:"ocid"`
	CompartmentID   string                 `json:"compartment_id"`
	AdditionalInfo  map[string]interface{} `json:"additional_info"`
}

// CompartmentNameCache provides thread-safe caching for compartment name resolution
type CompartmentNameCache struct {
	mu     sync.RWMutex
	cache  map[string]string // OCID -> Name mapping
	client identity.IdentityClient
}

