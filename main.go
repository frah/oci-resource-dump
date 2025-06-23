package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)








// Global logger instance
var logger *Logger














// Output functions moved to output.go

func main() {
	// CLI argument variables with special default values to detect "not specified"
	var timeoutSeconds *int = flag.Int("timeout", -1, "Timeout in seconds for the entire operation")
	var timeoutShort *int = flag.Int("t", -1, "Timeout in seconds for the entire operation (shorthand)")
	var logLevelStr *string = flag.String("log-level", "NOT_SET", "Log level: silent, normal, verbose, debug")
	var logLevelShort *string = flag.String("l", "NOT_SET", "Log level: silent, normal, verbose, debug (shorthand)")
	var outputFormat *string = flag.String("format", "NOT_SET", "Output format: csv, tsv, or json")
	var outputFormatShort *string = flag.String("f", "NOT_SET", "Output format: csv, tsv, or json (shorthand)")
	var showProgress *bool = flag.Bool("progress", false, "Show progress bar with real-time statistics")
	var noProgress *bool = flag.Bool("no-progress", false, "Disable progress bar (default behavior)")
	var outputFile *string = flag.String("output-file", "NOT_SET", "Output file path (default: stdout)")
	var outputFileShort *string = flag.String("o", "NOT_SET", "Output file path (default: stdout, shorthand)")
	var generateConfig *bool = flag.Bool("generate-config", false, "Generate default configuration file")
	
	// Phase 2B: Filter CLI arguments
	var compartments *string = flag.String("compartments", "", "Comma-separated list of compartment OCIDs to include")
	var excludeCompartments *string = flag.String("exclude-compartments", "", "Comma-separated list of compartment OCIDs to exclude")
	var resourceTypes *string = flag.String("resource-types", "", "Comma-separated list of resource types to include")
	var excludeResourceTypes *string = flag.String("exclude-resource-types", "", "Comma-separated list of resource types to exclude")
	var nameFilter *string = flag.String("name-filter", "", "Regex pattern for resource names to include")
	var excludeNameFilter *string = flag.String("exclude-name-filter", "", "Regex pattern for resource names to exclude")
	
	// Phase 2C: Diff analysis CLI arguments
	var compareFiles *string = flag.String("compare-files", "", "Comma-separated pair of JSON files to compare (old,new)")
	var diffOutput *string = flag.String("diff-output", "", "Output file for diff analysis (default: stdout)")
	var diffFormat *string = flag.String("diff-format", "json", "Diff output format: json, text")
	var diffDetailed *bool = flag.Bool("diff-detailed", false, "Include unchanged resources in diff output")
	
	flag.Parse()

	// Handle configuration file generation
	if *generateConfig {
		if err := GenerateDefaultConfigFile("oci-resource-dump.yaml"); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating configuration file: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Default configuration file generated: oci-resource-dump.yaml")
		return
	}
	
	// Phase 2C: Handle diff analysis mode
	if *compareFiles != "" {
		// Initialize logger for diff mode
		logger = NewLogger(LogLevelNormal)
		
		files := strings.Split(*compareFiles, ",")
		if len(files) != 2 {
			fmt.Fprintf(os.Stderr, "Error: --compare-files requires exactly 2 files separated by comma\n")
			fmt.Fprintf(os.Stderr, "Example: --compare-files old.json,new.json\n")
			os.Exit(1)
		}
		
		oldFile := strings.TrimSpace(files[0])
		newFile := strings.TrimSpace(files[1])
		
		// Configure diff settings
		diffConfig := DiffConfig{
			Format:     *diffFormat,
			Detailed:   *diffDetailed,
			OutputFile: *diffOutput,
		}
		
		// Perform diff analysis
		result, err := CompareDumps(oldFile, newFile, diffConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error performing diff analysis: %v\n", err)
			os.Exit(1)
		}
		
		// Output results
		if err := OutputDiffResult(result, diffConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Error outputting diff results: %v\n", err)
			os.Exit(1)
		}
		
		return
	}

	// Initialize temporary logger for configuration loading
	logger = NewLogger(LogLevelNormal)
	
	// Load configuration from file
	appConfig, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Resolve CLI argument priorities (shorthand overrides long form, only if explicitly set)
	finalTimeout := timeoutSeconds
	if *timeoutShort != -1 {
		finalTimeout = timeoutShort
	}
	
	finalLogLevel := logLevelStr
	if *logLevelShort != "NOT_SET" {
		finalLogLevel = logLevelShort
	}
	
	finalFormat := outputFormat
	if *outputFormatShort != "NOT_SET" {
		finalFormat = outputFormatShort
	}
	
	finalOutputFile := outputFile
	if *outputFileShort != "NOT_SET" {
		finalOutputFile = outputFileShort
	}
	
	// Progress flags handling: only explicit flags override config
	var finalProgress *bool
	if *noProgress {
		finalProgress = func() *bool { b := false; return &b }() // explicit --no-progress
	} else if *showProgress {
		finalProgress = func() *bool { b := true; return &b }() // explicit --progress
	} else {
		finalProgress = nil // not specified, don't override config
	}

	// Merge CLI arguments with configuration file (CLI has higher priority)
	MergeWithCLIArgs(appConfig, finalTimeout, finalLogLevel, finalFormat, finalProgress, finalOutputFile)
	
	// Phase 2B: Parse and merge filter arguments
	if *compartments != "" {
		appConfig.Filters.IncludeCompartments = ParseCompartmentList(*compartments)
	}
	if *excludeCompartments != "" {
		appConfig.Filters.ExcludeCompartments = ParseCompartmentList(*excludeCompartments)
	}
	if *resourceTypes != "" {
		appConfig.Filters.IncludeResourceTypes = ParseResourceTypeList(*resourceTypes)
	}
	if *excludeResourceTypes != "" {
		appConfig.Filters.ExcludeResourceTypes = ParseResourceTypeList(*excludeResourceTypes)
	}
	if *nameFilter != "" {
		appConfig.Filters.NamePattern = *nameFilter
	}
	if *excludeNameFilter != "" {
		appConfig.Filters.ExcludeNamePattern = *excludeNameFilter
	}

	// Validate filter configuration
	if err := ValidateFilterConfig(appConfig.Filters); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid filter configuration: %v\n", err)
		os.Exit(1)
	}
	
	// Convert AppConfig to runtime Config
	config := &Config{}
	config.Timeout = time.Duration(appConfig.General.Timeout) * time.Second
	config.OutputFormat = strings.ToLower(appConfig.General.OutputFormat)
	config.Filters = appConfig.Filters
	
	// Parse and validate log level
	logLevel, err := ParseLogLevel(appConfig.General.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}
	config.LogLevel = logLevel
	
	// Configure progress bar - from config file or CLI
	config.ShowProgress = appConfig.General.Progress
	
	// Re-initialize logger with final log level
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

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Initialize OCI clients
	logger.Debug("Initializing OCI clients with instance principal authentication")
	clients, err := initOCIClients(ctx)
	if err != nil {
		logger.Error("Error initializing OCI clients: %v", err)
		os.Exit(1)
	}
	logger.Verbose("OCI clients initialized successfully")

	// Discover all resources
	logger.Info("Starting resource discovery with %v timeout...", config.Timeout)
	logger.Debug("Discovery configuration - Format: %s, Timeout: %v, LogLevel: %s, Progress: %v", config.OutputFormat, config.Timeout, config.LogLevel, config.ShowProgress)
	resources, err := discoverAllResourcesWithProgress(ctx, clients, config.ProgressTracker, config.Filters)
	if err != nil {
		logger.Error("Error discovering resources: %v", err)
		os.Exit(1)
	}

	// Output resources in the specified format
	logger.Debug("Outputting %d resources in %s format", len(resources), config.OutputFormat)
	
	// Handle file output vs stdout
	if appConfig.Output.File != "" {
		logger.Info("Writing output to file: %s", appConfig.Output.File)
		if err := outputResourcesToFile(resources, config.OutputFormat, appConfig.Output.File); err != nil {
			logger.Error("Error outputting resources to file: %v", err)
			os.Exit(1)
		}
		logger.Verbose("Resource output completed successfully to file: %s", appConfig.Output.File)
	} else {
		if err := outputResources(resources, config.OutputFormat); err != nil {
			logger.Error("Error outputting resources: %v", err)
			os.Exit(1)
		}
		logger.Verbose("Resource output completed successfully to stdout")
	}
}
