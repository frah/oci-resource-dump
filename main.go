package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)








// Global logger instance
var logger *Logger














// Discovery functions moved to discovery.go
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
