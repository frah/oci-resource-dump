package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Global logger instance
var logger *Logger

// Output functions moved to output.go

func main() {
	// Variables for CLI arguments
	var (
		// Basic options
		timeoutSeconds int
		logLevelStr    string
		outputFormat   string
		showProgress   bool
		noProgress     bool
		outputFile     string
		generateConfig bool

		// Filter options
		compartments         string
		excludeCompartments  string
		resourceTypes        string
		excludeResourceTypes string
		nameFilter           string
		excludeNameFilter    string

		// Diff analysis options
		compareFiles string
		diffOutput   string
		diffFormat   string
		diffDetailed bool
	)

	var rootCmd = &cobra.Command{
		Use:   "oci-resource-dump",
		Short: "OCI Resource Dump Tool",
		Long: `OCI Resource Dump Tool - Discover and export OCI resources

This tool connects to your OCI tenancy using instance principal authentication
and discovers various types of resources, outputting their details in JSON, CSV, or TSV format.

The tool supports filtering by compartments, resource types, and name patterns,
as well as diff analysis between two resource dumps.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMainLogic(timeoutSeconds, logLevelStr, outputFormat, showProgress, noProgress,
				outputFile, generateConfig, compartments, excludeCompartments, resourceTypes,
				excludeResourceTypes, nameFilter, excludeNameFilter, compareFiles, diffOutput,
				diffFormat, diffDetailed)
		},
	}

	// Basic Options
	rootCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", -1, "Timeout in seconds for the entire operation")
	rootCmd.Flags().StringVarP(&logLevelStr, "log-level", "l", "NOT_SET", "Log level: silent, normal, verbose, debug")
	rootCmd.Flags().StringVarP(&outputFormat, "format", "f", "NOT_SET", "Output format: csv, tsv, or json")
	rootCmd.Flags().BoolVar(&showProgress, "progress", true, "Show progress bar with real-time statistics (default behavior)")
	rootCmd.Flags().BoolVar(&noProgress, "no-progress", false, "Disable progress bar")
	rootCmd.Flags().StringVarP(&outputFile, "output-file", "o", "NOT_SET", "Output file path (default: stdout)")
	rootCmd.Flags().BoolVar(&generateConfig, "generate-config", false, "Generate default configuration file")

	// Filtering Options
	rootCmd.Flags().StringVar(&compartments, "compartments", "", "Comma-separated list of compartment OCIDs to include")
	rootCmd.Flags().StringVar(&excludeCompartments, "exclude-compartments", "", "Comma-separated list of compartment OCIDs to exclude")
	rootCmd.Flags().StringVar(&resourceTypes, "resource-types", "", "Comma-separated list of resource types to include")
	rootCmd.Flags().StringVar(&excludeResourceTypes, "exclude-resource-types", "", "Comma-separated list of resource types to exclude")
	rootCmd.Flags().StringVar(&nameFilter, "name-filter", "", "Regex pattern for resource names to include")
	rootCmd.Flags().StringVar(&excludeNameFilter, "exclude-name-filter", "", "Regex pattern for resource names to exclude")

	// Diff Analysis Options
	rootCmd.Flags().StringVar(&compareFiles, "compare-files", "", "Comma-separated pair of JSON files to compare (old,new)")
	rootCmd.Flags().StringVar(&diffOutput, "diff-output", "", "Output file for diff analysis (default: stdout)")
	rootCmd.Flags().StringVar(&diffFormat, "diff-format", "json", "Diff output format: json, text")
	rootCmd.Flags().BoolVar(&diffDetailed, "diff-detailed", false, "Include unchanged resources in diff output")

	// Configuration Options - separate group
	// (generateConfig is already defined above)

	// Group annotations for better help display
	rootCmd.Flags().SetAnnotation("timeout", "group", []string{"basic"})
	rootCmd.Flags().SetAnnotation("log-level", "group", []string{"basic"})
	rootCmd.Flags().SetAnnotation("format", "group", []string{"basic"})
	rootCmd.Flags().SetAnnotation("progress", "group", []string{"basic"})
	rootCmd.Flags().SetAnnotation("no-progress", "group", []string{"basic"})
	rootCmd.Flags().SetAnnotation("output-file", "group", []string{"basic"})

	rootCmd.Flags().SetAnnotation("compartments", "group", []string{"filtering"})
	rootCmd.Flags().SetAnnotation("exclude-compartments", "group", []string{"filtering"})
	rootCmd.Flags().SetAnnotation("resource-types", "group", []string{"filtering"})
	rootCmd.Flags().SetAnnotation("exclude-resource-types", "group", []string{"filtering"})
	rootCmd.Flags().SetAnnotation("name-filter", "group", []string{"filtering"})
	rootCmd.Flags().SetAnnotation("exclude-name-filter", "group", []string{"filtering"})

	rootCmd.Flags().SetAnnotation("compare-files", "group", []string{"diff"})
	rootCmd.Flags().SetAnnotation("diff-output", "group", []string{"diff"})
	rootCmd.Flags().SetAnnotation("diff-format", "group", []string{"diff"})
	rootCmd.Flags().SetAnnotation("diff-detailed", "group", []string{"diff"})

	rootCmd.Flags().SetAnnotation("generate-config", "group", []string{"config"})

	// Custom help function to group flags
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n\n", cmd.Short)
		fmt.Printf("%s\n\n", cmd.Long)
		fmt.Printf("Usage:\n  %s [flags]\n\n", cmd.Use)

		// Basic Options
		fmt.Printf("BASIC OPTIONS:\n")
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if annotations, ok := flag.Annotations["group"]; ok && len(annotations) > 0 && annotations[0] == "basic" {
				if flag.Shorthand != "" {
					fmt.Printf("  -%s, --%-17s %s\n", flag.Shorthand, flag.Name, flag.Usage)
				} else {
					fmt.Printf("      --%-20s %s\n", flag.Name, flag.Usage)
				}
			}
		})

		// Filtering Options
		fmt.Printf("\nFILTERING OPTIONS:\n")
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if annotations, ok := flag.Annotations["group"]; ok && len(annotations) > 0 && annotations[0] == "filtering" {
				if flag.Shorthand != "" {
					fmt.Printf("  -%s, --%-17s %s\n", flag.Shorthand, flag.Name, flag.Usage)
				} else {
					fmt.Printf("      --%-20s %s\n", flag.Name, flag.Usage)
				}
			}
		})

		// Diff Analysis Options
		fmt.Printf("\nDIFF ANALYSIS OPTIONS:\n")
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if annotations, ok := flag.Annotations["group"]; ok && len(annotations) > 0 && annotations[0] == "diff" {
				if flag.Shorthand != "" {
					fmt.Printf("  -%s, --%-17s %s\n", flag.Shorthand, flag.Name, flag.Usage)
				} else {
					fmt.Printf("      --%-20s %s\n", flag.Name, flag.Usage)
				}
			}
		})

		// Configuration Options
		fmt.Printf("\nCONFIGURATION OPTIONS:\n")
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if annotations, ok := flag.Annotations["group"]; ok && len(annotations) > 0 && annotations[0] == "config" {
				if flag.Shorthand != "" {
					fmt.Printf("  -%s, --%-17s %s\n", flag.Shorthand, flag.Name, flag.Usage)
				} else {
					fmt.Printf("      --%-20s %s\n", flag.Name, flag.Usage)
				}
			}
		})

		fmt.Printf("\nEXAMPLES:\n")
		fmt.Printf("  # Basic usage with CSV output\n")
		fmt.Printf("  %s --format csv\n\n", cmd.Use)
		fmt.Printf("  # Filter specific compartments with progress\n")
		fmt.Printf("  %s --compartments ocid1.compartment.oc1..prod --progress\n\n", cmd.Use)
		fmt.Printf("  # Compare two resource dumps\n")
		fmt.Printf("  %s --compare-files old.json,new.json --diff-format text\n\n", cmd.Use)
		fmt.Printf("  # Generate configuration file\n")
		fmt.Printf("  %s --generate-config\n", cmd.Use)
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runMainLogic(timeoutSeconds int, logLevelStr, outputFormat string, showProgress, noProgress bool,
	outputFile string, generateConfig bool, compartments, excludeCompartments, resourceTypes,
	excludeResourceTypes, nameFilter, excludeNameFilter, compareFiles, diffOutput,
	diffFormat string, diffDetailed bool) error {

	// Handle configuration file generation
	if generateConfig {
		if err := GenerateDefaultConfigFile("oci-resource-dump.yaml"); err != nil {
			return fmt.Errorf("error generating configuration file: %v", err)
		}
		fmt.Println("Default configuration file generated: oci-resource-dump.yaml")
		return nil
	}

	// Phase 2C: Handle diff analysis mode
	if compareFiles != "" {
		// Initialize logger for diff mode
		logger = NewLogger(LogLevelNormal)

		files := strings.Split(compareFiles, ",")
		if len(files) != 2 {
			return fmt.Errorf("--compare-files requires exactly 2 files separated by comma\nExample: --compare-files old.json,new.json")
		}

		oldFile := strings.TrimSpace(files[0])
		newFile := strings.TrimSpace(files[1])

		// Configure diff settings
		diffConfig := DiffConfig{
			Format:     diffFormat,
			Detailed:   diffDetailed,
			OutputFile: diffOutput,
		}

		// Perform diff analysis
		result, err := CompareDumps(oldFile, newFile, diffConfig)
		if err != nil {
			return fmt.Errorf("error performing diff analysis: %v", err)
		}

		// Output results
		if err := OutputDiffResult(result, diffConfig); err != nil {
			return fmt.Errorf("error outputting diff results: %v", err)
		}

		return nil
	}

	// Initialize temporary logger for configuration loading
	logger = NewLogger(LogLevelNormal)

	// Load configuration from file
	appConfig, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading configuration: %v", err)
	}

	// Create CLI argument pointers to match the expected interface
	var finalTimeout *int
	var finalLogLevel *string
	var finalFormat *string
	var finalOutputFile *string

	if timeoutSeconds != -1 {
		finalTimeout = &timeoutSeconds
	}
	if logLevelStr != "NOT_SET" {
		finalLogLevel = &logLevelStr
	}
	if outputFormat != "NOT_SET" {
		finalFormat = &outputFormat
	}
	if outputFile != "NOT_SET" {
		finalOutputFile = &outputFile
	}

	// Progress flags handling: only explicit flags override config
	var finalProgress *bool
	if noProgress {
		finalProgress = func() *bool { b := false; return &b }() // explicit --no-progress
	} else if showProgress {
		finalProgress = func() *bool { b := true; return &b }() // explicit --progress
	} else {
		finalProgress = nil // not specified, don't override config
	}

	// Merge CLI arguments with configuration file (CLI has higher priority)
	MergeWithCLIArgs(appConfig, finalTimeout, finalLogLevel, finalFormat, finalProgress, finalOutputFile)

	// Phase 2B: Parse and merge filter arguments
	if compartments != "" {
		appConfig.Filters.IncludeCompartments = ParseCompartmentList(compartments)
	}
	if excludeCompartments != "" {
		appConfig.Filters.ExcludeCompartments = ParseCompartmentList(excludeCompartments)
	}
	if resourceTypes != "" {
		appConfig.Filters.IncludeResourceTypes = ParseResourceTypeList(resourceTypes)
	}
	if excludeResourceTypes != "" {
		appConfig.Filters.ExcludeResourceTypes = ParseResourceTypeList(excludeResourceTypes)
	}
	if nameFilter != "" {
		appConfig.Filters.NamePattern = nameFilter
	}
	if excludeNameFilter != "" {
		appConfig.Filters.ExcludeNamePattern = excludeNameFilter
	}

	// Validate filter configuration
	if err := ValidateFilterConfig(appConfig.Filters); err != nil {
		return fmt.Errorf("invalid filter configuration: %v", err)
	}

	// Convert AppConfig to runtime Config
	config := &Config{}
	config.Timeout = time.Duration(appConfig.General.Timeout) * time.Second
	config.OutputFormat = strings.ToLower(appConfig.General.OutputFormat)
	config.Filters = appConfig.Filters

	// Parse and validate log level
	logLevel, err := ParseLogLevel(appConfig.General.LogLevel)
	if err != nil {
		return fmt.Errorf("invalid log level: %v", err)
	}
	config.LogLevel = logLevel

	// Configure progress bar - from config file or CLI
	config.ShowProgress = appConfig.General.Progress
	
	// CLI flags override config file
	if showProgress {
		config.ShowProgress = true
	}
	if noProgress {
		config.ShowProgress = false
	}

	// Re-initialize logger with final log level
	logger = NewLogger(logLevel)
	config.Logger = logger

	// Progress tracking is now handled directly in discovery.go with uiprogress

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
		return fmt.Errorf("invalid output format '%s'. Valid formats are: csv, tsv, json", config.OutputFormat)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Initialize OCI clients
	logger.Debug("Initializing OCI clients with instance principal authentication")
	clients, err := initOCIClients(ctx)
	if err != nil {
		return fmt.Errorf("error initializing OCI clients: %v", err)
	}
	logger.Verbose("OCI clients initialized successfully")

	// Preload compartment names for better performance
	logger.Debug("Preloading compartment names...")

	// Get tenancy ID for preloading
	provider, err := auth.InstancePrincipalConfigurationProvider()
	if err != nil {
		return fmt.Errorf("error getting configuration provider: %v", err)
	}
	tenancyID, err := provider.TenancyOCID()
	if err != nil {
		return fmt.Errorf("error getting tenancy ID: %v", err)
	}

	err = clients.CompartmentCache.PreloadCompartmentNames(ctx, tenancyID)
	if err != nil {
		logger.Verbose("Warning: Could not preload all compartment names: %v", err)
		// Continue execution - individual lookups will still work
	} else {
		totalEntries, _ := clients.CompartmentCache.GetCacheStats()
		logger.Verbose("Preloaded %d compartment names into cache", totalEntries)
	}

	// Discover all resources
	logger.Info("Starting resource discovery with %v timeout...", config.Timeout)
	logger.Debug("Discovery configuration - Format: %s, Timeout: %v, LogLevel: %s, Progress: %v", config.OutputFormat, config.Timeout, config.LogLevel, config.ShowProgress)
	resources, err := discoverAllResourcesWithProgress(ctx, clients, config.ShowProgress, config.Filters)
	if err != nil {
		return fmt.Errorf("error discovering resources: %v", err)
	}

	// Output resources in the specified format
	logger.Debug("Outputting %d resources in %s format", len(resources), config.OutputFormat)

	// Handle file output vs stdout
	if appConfig.Output.File != "" {
		logger.Info("Writing output to file: %s", appConfig.Output.File)
		if err := outputResourcesToFile(resources, config.OutputFormat, appConfig.Output.File); err != nil {
			return fmt.Errorf("error outputting resources to file: %v", err)
		}
		logger.Verbose("Resource output completed successfully to file: %s", appConfig.Output.File)
	} else {
		if err := outputResources(resources, config.OutputFormat); err != nil {
			return fmt.Errorf("error outputting resources: %v", err)
		}
		logger.Verbose("Resource output completed successfully to stdout")
	}

	return nil
}
