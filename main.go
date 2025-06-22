package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/apigateway"
	"github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/database"
	"github.com/oracle/oci-go-sdk/v65/functions"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/loadbalancer"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/oracle/oci-go-sdk/v65/filestorage"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
	"github.com/oracle/oci-go-sdk/v65/streaming"
)

// LogLevel represents the logging verbosity level
type LogLevel int

const (
	LogLevelSilent LogLevel = iota // Only errors
	LogLevelNormal                 // Basic progress info (default)
	LogLevelVerbose                // Detailed operational info
	LogLevelDebug                  // Full diagnostic info
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelSilent:
		return "silent"
	case LogLevelNormal:
		return "normal"
	case LogLevelVerbose:
		return "verbose"
	case LogLevelDebug:
		return "debug"
	default:
		return "unknown"
	}
}

// ParseLogLevel parses a string into a LogLevel
func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToLower(s) {
	case "silent":
		return LogLevelSilent, nil
	case "normal":
		return LogLevelNormal, nil
	case "verbose":
		return LogLevelVerbose, nil
	case "debug":
		return LogLevelDebug, nil
	default:
		return LogLevelNormal, fmt.Errorf("invalid log level: %s (valid: silent, normal, verbose, debug)", s)
	}
}

// Logger provides structured logging with multiple levels
type Logger struct {
	level    LogLevel
	errorLog *log.Logger
	infoLog  *log.Logger
	debugLog *log.Logger
	mu       sync.RWMutex
}

// NewLogger creates a new logger with the specified level
func NewLogger(level LogLevel) *Logger {
	logger := &Logger{
		level: level,
	}
	
	// Always create error logger (goes to stderr)
	logger.errorLog = log.New(os.Stderr, "ERROR: ", log.LstdFlags)
	
	// Create info logger based on level (goes to stderr for progress info)
	if level >= LogLevelNormal {
		logger.infoLog = log.New(os.Stderr, "", log.LstdFlags)
	} else {
		logger.infoLog = log.New(io.Discard, "", 0)
	}
	
	// Create debug logger based on level
	if level >= LogLevelDebug {
		logger.debugLog = log.New(os.Stderr, "DEBUG: ", log.LstdFlags|log.Lshortfile)
	} else {
		logger.debugLog = log.New(io.Discard, "", 0)
	}
	
	return logger
}

// Error logs error messages (always visible except in silent mode)
func (l *Logger) Error(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.errorLog.Printf(format, args...)
}

// Info logs informational messages (visible in normal, verbose, debug)
func (l *Logger) Info(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.level >= LogLevelNormal {
		l.infoLog.Printf(format, args...)
	}
}

// Verbose logs detailed operational messages (visible in verbose, debug)
func (l *Logger) Verbose(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.level >= LogLevelVerbose {
		l.infoLog.Printf("VERBOSE: "+format, args...)
	}
}

// Debug logs debug messages (visible only in debug mode)
func (l *Logger) Debug(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.level >= LogLevelDebug {
		l.debugLog.Printf(format, args...)
	}
}

// SetLevel updates the logging level dynamically
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
	
	// Recreate loggers based on new level
	if level >= LogLevelNormal {
		l.infoLog = log.New(os.Stderr, "", log.LstdFlags)
	} else {
		l.infoLog = log.New(io.Discard, "", 0)
	}
	
	if level >= LogLevelDebug {
		l.debugLog = log.New(os.Stderr, "DEBUG: ", log.LstdFlags|log.Lshortfile)
	} else {
		l.debugLog = log.New(io.Discard, "", 0)
	}
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}






type Config struct {
	OutputFormat        string
	Timeout             time.Duration
	MaxWorkers          int
	LogLevel            LogLevel
	Logger              *Logger
	ShowProgress        bool
	ProgressTracker     *ProgressTracker
}

// Global logger instance
var logger *Logger




// ProgressTracker provides thread-safe progress tracking with ETA calculation
type ProgressTracker struct {
	mu                    sync.RWMutex
	startTime            time.Time
	lastUpdateTime       time.Time
	totalCompartments    int64
	processedCompartments int64
	totalResourceTypes   int64
	processedResourceTypes int64
	totalResources       int64
	errorCount          int64
	retryCount          int64
	currentOperation     string
	currentCompartment   string
	enabled             bool
	speedSamples        []float64
	maxSamples          int
	refreshInterval     time.Duration
	done                chan struct{}
	updateChannel       chan ProgressUpdate
}

// ProgressUpdate represents a progress update from worker goroutines
type ProgressUpdate struct {
	CompartmentName string
	Operation      string
	ResourceCount  int64
	IsCompartmentComplete bool
	IsError        bool
	IsRetry        bool
}




// NewProgressTracker creates a new progress tracker
func NewProgressTracker(enabled bool, totalCompartments, totalResourceTypes int64) *ProgressTracker {
	if !enabled {
		return &ProgressTracker{enabled: false}
	}
	
	return &ProgressTracker{
		startTime:            time.Now(),
		lastUpdateTime:       time.Now(),
		totalCompartments:    totalCompartments,
		totalResourceTypes:   totalResourceTypes,
		enabled:             true,
		maxSamples:          20,
		refreshInterval:     500 * time.Millisecond,
		done:                make(chan struct{}),
		updateChannel:       make(chan ProgressUpdate, 100),
		speedSamples:        make([]float64, 0, 20),
	}
}

// Start begins the progress tracking display
func (pt *ProgressTracker) Start() {
	if !pt.enabled {
		return
	}
	
	go pt.displayLoop()
	go pt.updateLoop()
}

// Stop terminates the progress tracking
func (pt *ProgressTracker) Stop() {
	if !pt.enabled {
		return
	}
	
	close(pt.done)
	// Clear the progress line
	fmt.Fprint(os.Stderr, "\r\033[K")
}

// Update sends a progress update
func (pt *ProgressTracker) Update(update ProgressUpdate) {
	if !pt.enabled {
		return
	}
	
	select {
	case pt.updateChannel <- update:
	default:
		// Channel full, skip this update
	}
}

// updateLoop processes progress updates from worker goroutines
func (pt *ProgressTracker) updateLoop() {
	for {
		select {
		case <-pt.done:
			return
		case update := <-pt.updateChannel:
			pt.processUpdate(update)
		}
	}
}

// processUpdate handles individual progress updates
func (pt *ProgressTracker) processUpdate(update ProgressUpdate) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	
	if update.IsError {
		atomic.AddInt64(&pt.errorCount, 1)
	}
	if update.IsRetry {
		atomic.AddInt64(&pt.retryCount, 1)
	}
	if update.ResourceCount > 0 {
		atomic.AddInt64(&pt.totalResources, update.ResourceCount)
	}
	if update.IsCompartmentComplete {
		atomic.AddInt64(&pt.processedCompartments, 1)
	}
	if update.Operation != "" {
		pt.currentOperation = update.Operation
		pt.currentCompartment = update.CompartmentName
		atomic.AddInt64(&pt.processedResourceTypes, 1)
	}
}

// displayLoop handles the progress bar display
func (pt *ProgressTracker) displayLoop() {
	ticker := time.NewTicker(pt.refreshInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-pt.done:
			return
		case <-ticker.C:
			pt.updateDisplay()
		}
	}
}

// updateDisplay renders the progress bar
func (pt *ProgressTracker) updateDisplay() {
	pt.mu.RLock()
	
	elapsed := time.Since(pt.startTime)
	totalOps := pt.totalCompartments * pt.totalResourceTypes
	processedOps := atomic.LoadInt64(&pt.processedResourceTypes)
	totalResources := atomic.LoadInt64(&pt.totalResources)
	errors := atomic.LoadInt64(&pt.errorCount)
	retries := atomic.LoadInt64(&pt.retryCount)
	processedCompartments := atomic.LoadInt64(&pt.processedCompartments)
	
	currentOp := pt.currentOperation
	currentComp := pt.currentCompartment
	
	pt.mu.RUnlock()
	
	// Calculate progress percentage
	var progress float64
	if totalOps > 0 {
		progress = float64(processedOps) / float64(totalOps) * 100
	}
	
	// Calculate speed and ETA
	speed := pt.calculateSpeed(totalResources, elapsed)
	eta := pt.calculateETA(progress, elapsed)
	
	// Create progress bar
	barWidth := 30
	filled := int(progress / 100 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	
	// Format current operation
	currentInfo := ""
	if currentOp != "" && currentComp != "" {
		currentInfo = fmt.Sprintf(" | %s in %s", currentOp, currentComp)
		if len(currentInfo) > 50 {
			currentInfo = currentInfo[:47] + "..."
		}
	}
	
	// Build progress line
	progressLine := fmt.Sprintf(
		"\r[%s] %5.1f%% | %5.1f res/s | ETA: %s | Elapsed: %s | Comp: %d/%d | Res: %d",
		bar,
		progress,
		speed,
		eta,
		pt.formatDuration(elapsed),
		processedCompartments,
		pt.totalCompartments,
		totalResources,
	)
	
	if errors > 0 || retries > 0 {
		progressLine += fmt.Sprintf(" | Err: %d | Retry: %d", errors, retries)
	}
	
	progressLine += currentInfo
	
	// Ensure the line doesn't exceed terminal width (assume 120 chars)
	if len(progressLine) > 120 {
		progressLine = progressLine[:117] + "..."
	}
	
	fmt.Fprint(os.Stderr, progressLine)
}

// calculateSpeed computes the current processing speed
func (pt *ProgressTracker) calculateSpeed(totalResources int64, elapsed time.Duration) float64 {
	if elapsed.Seconds() <= 0 {
		return 0
	}
	
	currentSpeed := float64(totalResources) / elapsed.Seconds()
	
	// Update speed samples for EMA calculation
	pt.speedSamples = append(pt.speedSamples, currentSpeed)
	if len(pt.speedSamples) > pt.maxSamples {
		pt.speedSamples = pt.speedSamples[1:]
	}
	
	// Calculate exponential moving average
	if len(pt.speedSamples) == 0 {
		return currentSpeed
	}
	
	ema := pt.speedSamples[0]
	alpha := 0.1
	for i := 1; i < len(pt.speedSamples); i++ {
		ema = alpha*pt.speedSamples[i] + (1-alpha)*ema
	}
	
	return ema
}

// calculateETA estimates time to completion
func (pt *ProgressTracker) calculateETA(progress float64, elapsed time.Duration) string {
	if progress <= 0 || progress >= 100 {
		return "00:00:00"
	}
	
	// Estimate based on current progress rate
	remainingPercent := 100 - progress
	timePerPercent := elapsed.Seconds() / progress
	etaSeconds := remainingPercent * timePerPercent
	
	if etaSeconds > 3600*24 { // More than 24 hours
		return "24:00:00+"
	}
	
	return pt.formatDuration(time.Duration(etaSeconds) * time.Second)
}

// formatDuration formats a duration as HH:MM:SS
func (pt *ProgressTracker) formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
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
	APIGatewayClient        apigateway.GatewayClient
	FunctionsClient         functions.FunctionsManagementClient
	FileStorageClient       filestorage.FileStorageClient
	NetworkLoadBalancerClient networkloadbalancer.NetworkLoadBalancerClient
	StreamingClient         streaming.StreamAdminClient
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

	// Initialize API Gateway client
	apiGatewayClient, err := apigateway.NewGatewayClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create api gateway client: %w", err)
	}
	clients.APIGatewayClient = apiGatewayClient

	// Initialize Functions client
	functionsClient, err := functions.NewFunctionsManagementClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create functions client: %w", err)
	}
	clients.FunctionsClient = functionsClient

	// Initialize File Storage client
	fileStorageClient, err := filestorage.NewFileStorageClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create file storage client: %w", err)
	}
	clients.FileStorageClient = fileStorageClient

	// Initialize Network Load Balancer client
	nlbClient, err := networkloadbalancer.NewNetworkLoadBalancerClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create network load balancer client: %w", err)
	}
	clients.NetworkLoadBalancerClient = nlbClient

	// Initialize Streaming client
	streamingClient, err := streaming.NewStreamAdminClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming client: %w", err)
	}
	clients.StreamingClient = streamingClient

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

	logger.Debug("Completed compute instances discovery for compartment %s: found %d instances across %d pages", compartmentID, len(resources), pageCount)
	return resources, nil
}


func discoverVCNs(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allVCNs []core.Vcn

	// Implement pagination to get all VCNs
	var page *string
	for {
		req := core.ListVcnsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.VirtualNetworkClient.ListVcns(ctx, req)
		if err != nil {
			return nil, err
		}

		allVCNs = append(allVCNs, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, vcn := range allVCNs {
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
			
			// Add CIDR information
			if vcn.CidrBlock != nil {
				additionalInfo["cidr"] = *vcn.CidrBlock
			}
			
			// Add DNS settings
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
	var allSubnets []core.Subnet

	// Implement pagination to get all subnets
	var page *string
	for {
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
	var allVolumes []core.Volume

	// Implement pagination to get all block volumes
	var page *string
	for {
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
	var allBuckets []objectstorage.BucketSummary

	// Get namespace
	namespaceReq := objectstorage.GetNamespaceRequest{}
	namespaceResp, err := clients.ObjectStorageClient.GetNamespace(ctx, namespaceReq)
	if err != nil {
		return nil, err
	}

	// Implement pagination to get all buckets
	var page *string
	for {
		req := objectstorage.ListBucketsRequest{
			CompartmentId: common.String(compartmentID),
			NamespaceName: namespaceResp.Value,
			Page:         page,
		}

		resp, err := clients.ObjectStorageClient.ListBuckets(ctx, req)
		if err != nil {
			return nil, err
		}

		allBuckets = append(allBuckets, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, bucket := range allBuckets {
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
	var allClusters []containerengine.ClusterSummary

	// Implement pagination to get all OKE clusters
	var page *string
	for {
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
	var allLoadBalancers []loadbalancer.LoadBalancer

	// Implement pagination to get all load balancers
	var page *string
	for {
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
	var allDbSystems []database.DbSystemSummary

	// Implement pagination to get all database systems
	var page *string
	for {
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
	var allDrgs []core.Drg

	// Implement pagination to get all DRGs
	var page *string
	for {
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

func discoverAutonomousDatabases(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allADBs []database.AutonomousDatabaseSummary

	// Implement pagination to get all autonomous databases
	var page *string
	for {
		req := database.ListAutonomousDatabasesRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.DatabaseClient.ListAutonomousDatabases(ctx, req)
		if err != nil {
			return nil, err
		}

		allADBs = append(allADBs, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, adb := range allADBs {
		if adb.LifecycleState != database.AutonomousDatabaseSummaryLifecycleStateTerminated {
			name := ""
			if adb.DisplayName != nil {
				name = *adb.DisplayName
			}
			ocid := ""
			if adb.Id != nil {
				ocid = *adb.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			// Add workload type
			additionalInfo["workload_type"] = string(adb.DbWorkload)
			
			// Add CPU core count
			if adb.CpuCoreCount != nil {
				additionalInfo["cpu_core_count"] = *adb.CpuCoreCount
			}
			
			// Add data storage size
			if adb.DataStorageSizeInTBs != nil {
				additionalInfo["data_storage_size_tb"] = *adb.DataStorageSizeInTBs
			}
			
			// Add database version
			if adb.DbVersion != nil {
				additionalInfo["db_version"] = *adb.DbVersion
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "autonomous_database",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
		}
	}

	return resources, nil
}


func discoverFunctions(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allApplications []functions.ApplicationSummary

	// First, get all function applications
	var page *string
	for {
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

	// Add function applications to resources
	for _, app := range allApplications {
		if app.LifecycleState != functions.ApplicationLifecycleStateDeleted {
			name := ""
			if app.DisplayName != nil {
				name = *app.DisplayName
			}
			ocid := ""
			if app.Id != nil {
				ocid = *app.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			// Add subnet IDs
			if app.SubnetIds != nil {
				additionalInfo["subnet_ids"] = app.SubnetIds
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "function_application",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
			
			// Now get functions for this application
			if app.Id != nil {
				var funcPage *string
				for {
					funcReq := functions.ListFunctionsRequest{
						ApplicationId: app.Id,
						Page:         funcPage,
					}

					funcResp, err := clients.FunctionsClient.ListFunctions(ctx, funcReq)
					if err != nil {
						continue // Skip if we can't list functions for this app
					}

					for _, fn := range funcResp.Items {
						if fn.LifecycleState != functions.FunctionLifecycleStateDeleted {
							fnName := ""
							if fn.DisplayName != nil {
								fnName = *fn.DisplayName
							}
							fnOcid := ""
							if fn.Id != nil {
								fnOcid = *fn.Id
							}
							
							fnAdditionalInfo := make(map[string]interface{})
							
							// Add function-specific info
							if fn.Image != nil {
								fnAdditionalInfo["image"] = *fn.Image
							}
							if fn.MemoryInMBs != nil {
								fnAdditionalInfo["memory_mb"] = *fn.MemoryInMBs
							}
							if fn.TimeoutInSeconds != nil {
								fnAdditionalInfo["timeout_seconds"] = *fn.TimeoutInSeconds
							}
							
							resources = append(resources, ResourceInfo{
								ResourceType:   "function",
								ResourceName:   fnName,
								OCID:          fnOcid,
								CompartmentID: compartmentID,
								AdditionalInfo: fnAdditionalInfo,
							})
						}
					}

					if funcResp.OpcNextPage == nil {
						break
					}
					funcPage = funcResp.OpcNextPage
				}
			}
		}
	}

	return resources, nil
}


func discoverAPIGateways(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allGateways []apigateway.GatewaySummary

	// Implement pagination to get all API gateways
	var page *string
	for {
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
			
			// Add endpoint type
			additionalInfo["endpoint_type"] = string(gateway.EndpointType)
			
			// Add hostname
			if gateway.Hostname != nil {
				additionalInfo["hostname"] = *gateway.Hostname
			}
			
			// Add subnet ID
			if gateway.SubnetId != nil {
				additionalInfo["subnet_id"] = *gateway.SubnetId
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "api_gateway",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
		}
	}

	return resources, nil
}


func discoverFileStorageSystems(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allFileSystems []filestorage.FileSystemSummary

	// Implement pagination to get all file systems
	var page *string
	for {
		req := filestorage.ListFileSystemsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		resp, err := clients.FileStorageClient.ListFileSystems(ctx, req)
		if err != nil {
			return nil, err
		}

		allFileSystems = append(allFileSystems, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, fs := range allFileSystems {
		if fs.LifecycleState != filestorage.FileSystemSummaryLifecycleStateDeleted {
			name := ""
			if fs.DisplayName != nil {
				name = *fs.DisplayName
			}
			ocid := ""
			if fs.Id != nil {
				ocid = *fs.Id
			}
			
			additionalInfo := make(map[string]interface{})
			
			// Add availability domain
			if fs.AvailabilityDomain != nil {
				additionalInfo["availability_domain"] = *fs.AvailabilityDomain
			}
			
			// Add metered bytes
			if fs.MeteredBytes != nil {
				additionalInfo["metered_bytes"] = *fs.MeteredBytes
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "file_storage_system",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
		}
	}

	return resources, nil
}


func discoverNetworkLoadBalancers(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allNLBs []networkloadbalancer.NetworkLoadBalancerSummary

	// Implement pagination to get all network load balancers
	var page *string
	for {
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
			
			// Add IP addresses
			if nlb.IpAddresses != nil && len(nlb.IpAddresses) > 0 {
				var ipAddresses []string
				for _, ip := range nlb.IpAddresses {
					if ip.IpAddress != nil {
						ipAddresses = append(ipAddresses, *ip.IpAddress)
					}
				}
				additionalInfo["ip_addresses"] = ipAddresses
			}
			
			// Add is private
			if nlb.IsPrivate != nil {
				additionalInfo["is_private"] = *nlb.IsPrivate
			}
			
			// Add subnet ID
			if nlb.SubnetId != nil {
				additionalInfo["subnet_id"] = *nlb.SubnetId
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "network_load_balancer",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
			})
		}
	}

	return resources, nil
}


func discoverStreams(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allStreams []streaming.StreamSummary

	// Implement pagination to get all streams
	var page *string
	for {
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
			
			// Note: Retention hours not available in StreamSummary
			
			// Add stream pool ID
			if stream.StreamPoolId != nil {
				additionalInfo["stream_pool_id"] = *stream.StreamPoolId
			}
			
			resources = append(resources, ResourceInfo{
				ResourceType:   "stream",
				ResourceName:   name,
				OCID:          ocid,
				CompartmentID: compartmentID,
				AdditionalInfo: additionalInfo,
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

func isTransientError(err error) bool {
	// Check if the error is transient and should be retried
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

// Keep the old function for backward compatibility
func withRetry(ctx context.Context, operation func() error, maxRetries int, operationName string) error {
	return withRetryAndProgress(ctx, operation, maxRetries, operationName, nil)
}

func discoverAllResourcesWithProgress(ctx context.Context, clients *OCIClients, progressTracker *ProgressTracker) ([]ResourceInfo, error) {
	var allResources []ResourceInfo

	// Get list of compartments
	compartments, err := getCompartments(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("failed to get compartments: %w", err)
	}

	logger.Info("Found %d compartments to process", len(compartments))

	// Update progress tracker with compartment count
	if progressTracker != nil {
		progressTracker.totalCompartments = int64(len(compartments))
		progressTracker.totalResourceTypes = 15 // Number of resource types we discover
		progressTracker.Start()
		defer progressTracker.Stop()
	}

	// Discovery functions map
	discoveryFunctions := map[string]func(context.Context, *OCIClients, string) ([]ResourceInfo, error){
		"compute_instance":       discoverComputeInstances,
		"vcn":                   discoverVCNs,
		"subnet":                discoverSubnets,
		"block_volume":          discoverBlockVolumes,
		"object_storage_bucket": discoverObjectStorageBuckets,
		"oke_cluster":           discoverOKEClusters,
		"load_balancer":         discoverLoadBalancers,
		"database_system":       discoverDatabases,
		"drg":                   discoverDRGs,
		"autonomous_database":   discoverAutonomousDatabases,
		"function":              discoverFunctions,
		"api_gateway":           discoverAPIGateways,
		"file_storage_system":   discoverFileStorageSystems,
		"network_load_balancer": discoverNetworkLoadBalancers,
		"stream":                discoverStreams,
	}

	// Use semaphore to limit concurrent compartment processing
	maxWorkers := 5
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, compartment := range compartments {
		if compartment.LifecycleState != identity.CompartmentLifecycleStateActive {
			continue
		}

		wg.Add(1)
		go func(comp identity.Compartment) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			compartmentID := *comp.Id
			compartmentName := *comp.Name

			logger.Debug("Processing compartment: %s (%s)", compartmentName, compartmentID)

			// Discover resources in this compartment
			for resourceType, discoveryFunc := range discoveryFunctions {
				operation := func() error {
					if progressTracker != nil {
						progressTracker.Update(ProgressUpdate{
							CompartmentName: compartmentName,
							Operation:      resourceType,
						})
					}

					resources, err := discoveryFunc(ctx, clients, compartmentID)
					if err != nil {
						if isRetriableError(err) {
							logger.Debug("Skipping %s in compartment %s due to retriable error: %v", resourceType, compartmentName, err)
							return nil
						}
						return err
					}

					if len(resources) > 0 {
						logger.Verbose("Found %d %s resources in compartment %s", len(resources), resourceType, compartmentName)
						mu.Lock()
						allResources = append(allResources, resources...)
						mu.Unlock()

						if progressTracker != nil {
							progressTracker.Update(ProgressUpdate{
								ResourceCount: int64(len(resources)),
							})
						}
					}
					return nil
				}

				err := withRetryAndProgress(ctx, operation, 3, fmt.Sprintf("%s in %s", resourceType, compartmentName), progressTracker)
				if err != nil {
					logger.Error("Failed to discover %s in compartment %s after retries: %v", resourceType, compartmentName, err)
					if progressTracker != nil {
						progressTracker.Update(ProgressUpdate{IsError: true})
					}
				}
			}

			if progressTracker != nil {
				progressTracker.Update(ProgressUpdate{IsCompartmentComplete: true})
			}
			logger.Debug("Completed processing compartment: %s", compartmentName)
		}(compartment)
	}

	wg.Wait()

	logger.Info("Resource discovery completed. Found %d total resources across %d compartments", len(allResources), len(compartments))
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
	var timeoutMinutes int
	var logLevelStr string
	var showProgress bool
	var noProgress bool
	
	flag.StringVar(&config.OutputFormat, "format", "json", "Output format: csv, tsv, or json")
	flag.StringVar(&config.OutputFormat, "f", "json", "Output format: csv, tsv, or json (shorthand)")
	flag.IntVar(&timeoutMinutes, "timeout", 30, "Timeout in minutes for the entire operation")
	flag.IntVar(&timeoutMinutes, "t", 30, "Timeout in minutes for the entire operation (shorthand)")
	flag.StringVar(&logLevelStr, "log-level", "normal", "Log level: silent, normal, verbose, debug")
	flag.StringVar(&logLevelStr, "l", "normal", "Log level: silent, normal, verbose, debug (shorthand)")
	flag.BoolVar(&showProgress, "progress", false, "Show progress bar with real-time statistics")
	flag.BoolVar(&noProgress, "no-progress", false, "Disable progress bar (default behavior)")
	flag.Parse()

	// Set timeout duration
	config.Timeout = time.Duration(timeoutMinutes) * time.Minute
	
	// Parse and validate log level
	logLevel, err := ParseLogLevel(logLevelStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}
	config.LogLevel = logLevel
	
	// Configure progress bar - default to enabled unless explicitly disabled or silent mode
	config.ShowProgress = showProgress || (!noProgress && logLevel != LogLevelSilent)
	
	// Initialize global logger
	logger = NewLogger(logLevel)
	config.Logger = logger
	
	// Initialize progress tracker
	config.ProgressTracker = NewProgressTracker(config.ShowProgress, 0, 0)

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
	logger.Debug("Initializing OCI clients with instance principal authentication")
	clients, err := initOCIClients()
	if err != nil {
		logger.Error("Error initializing OCI clients: %v", err)
		os.Exit(1)
	}
	logger.Verbose("OCI clients initialized successfully")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Discover all resources
	logger.Info("Starting resource discovery with %v timeout...", config.Timeout)
	logger.Debug("Discovery configuration - Format: %s, Timeout: %v, LogLevel: %s, Progress: %v", config.OutputFormat, config.Timeout, config.LogLevel, config.ShowProgress)
	resources, err := discoverAllResourcesWithProgress(ctx, clients, config.ProgressTracker)
	if err != nil {
		logger.Error("Error discovering resources: %v", err)
		os.Exit(1)
	}

	// Output resources in the specified format
	logger.Debug("Outputting %d resources in %s format", len(resources), config.OutputFormat)
	if err := outputResources(resources, config.OutputFormat); err != nil {
		logger.Error("Error outputting resources: %v", err)
		os.Exit(1)
	}
	logger.Verbose("Resource output completed successfully")
}
