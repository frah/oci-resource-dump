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
	"sort"
	"strconv"
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

// Statistics collection structures





type Config struct {
func (sc *StatisticsCollector) getOrCreateCompartmentStats(compartmentID, compartmentName string) *CompartmentStats {
	if stats, exists := sc.compartmentStats[compartmentID]; exists {
		return stats
	}
	
	stats := &CompartmentStats{
		CompartmentID:   compartmentID,
		CompartmentName: compartmentName,
		ResourceTypes:   make(map[string]*ResourceTypeStats),
	}
	sc.compartmentStats[compartmentID] = stats
	sc.compartmentOrder = append(sc.compartmentOrder, compartmentID)
	return stats
}

// getOrCreateResourceTypeStats gets or creates resource type statistics
func (sc *StatisticsCollector) getOrCreateResourceTypeStats(resourceType string) *ResourceTypeStats {
	if stats, exists := sc.resourceTypeStats[resourceType]; exists {
		return stats
	}
	
	stats := &ResourceTypeStats{
		ResourceType: resourceType,
	}
	sc.resourceTypeStats[resourceType] = stats
	return stats
}

// getOrCreateCompartmentResourceTypeStats gets or creates compartment-specific resource type statistics
func (sc *StatisticsCollector) getOrCreateCompartmentResourceTypeStats(compStats *CompartmentStats, resourceType string) *ResourceTypeStats {
	if stats, exists := compStats.ResourceTypes[resourceType]; exists {
		return stats
	}
	
	stats := &ResourceTypeStats{
		ResourceType: resourceType,
	}
	compStats.ResourceTypes[resourceType] = stats
	return stats
}

// recordThroughputSample records a throughput sample
func (sc *StatisticsCollector) recordThroughputSample(throughput float64) {
	sample := ThroughputSample{
		Timestamp:  time.Now(),
		Throughput: throughput,
	}
	
	sc.throughputSamples = append(sc.throughputSamples, sample)
	if len(sc.throughputSamples) > sc.maxSamples {
		sc.throughputSamples = sc.throughputSamples[1:]
	}
}

// GenerateComprehensiveStatistics generates comprehensive statistics report
func (sc *StatisticsCollector) GenerateComprehensiveStatistics() *StatisticsReport {
	if !sc.enabled {
		return nil
	}
	
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	endTime := time.Now()
	totalExecutionTime := endTime.Sub(sc.startTime)
	
	// Calculate throughput statistics
	averageThroughput := float64(0)
	peakThroughput := float64(0)
	if totalExecutionTime.Seconds() > 0 {
		averageThroughput = float64(atomic.LoadInt64(&sc.globalResourceCount)) / totalExecutionTime.Seconds()
	}
	
	for _, sample := range sc.throughputSamples {
		if sample.Throughput > peakThroughput {
			peakThroughput = sample.Throughput
		}
	}
	
	// Calculate final throughput for each resource type
	for _, rtStats := range sc.resourceTypeStats {
		if rtStats.ProcessingTime.Seconds() > 0 {
			rtStats.Throughput = float64(rtStats.DiscoveryCount) / rtStats.ProcessingTime.Seconds()
		}
	}
	
	// Create execution summary
	executionSummary := ExecutionSummary{
		StartTime:             sc.startTime,
		EndTime:               endTime,
		TotalDuration:         totalExecutionTime,
		TotalResources:        atomic.LoadInt64(&sc.globalResourceCount),
		TotalAPICallss:        atomic.LoadInt64(&sc.globalAPICallCount),
		TotalErrors:           atomic.LoadInt64(&sc.globalErrorCount),
		TotalRetries:          atomic.LoadInt64(&sc.globalRetryCount),
		OverallThroughput:     averageThroughput,
		AvgAPILatency:         0, // Will calculate if needed
	}
	
	// Generate performance analysis
	performanceAnalysis := sc.generatePerformanceAnalysis()
	
	return &StatisticsReport{
		ExecutionSummary:     executionSummary,
		CompartmentStats:     sc.compartmentStats,
		ResourceTypeStats:    sc.resourceTypeStats,
		PerformanceAnalysis:  performanceAnalysis,
	}
}

// generatePerformanceAnalysis generates performance analysis insights
func (sc *StatisticsCollector) generatePerformanceAnalysis() PerformanceAnalysis {
	analysis := PerformanceAnalysis{
		Recommendations:    []string{},
		Bottlenecks:        []string{},
	}
	
	// Find slowest and fastest compartments
	var slowestComp, fastestComp, mostResourcesComp, mostAPICallsComp *CompartmentStats
	for _, compStats := range sc.compartmentStats {
		if slowestComp == nil || compStats.ProcessingTime > slowestComp.ProcessingTime {
			slowestComp = compStats
		}
		if fastestComp == nil || compStats.ProcessingTime < fastestComp.ProcessingTime {
			fastestComp = compStats
		}
		if mostResourcesComp == nil || compStats.ResourceCount > mostResourcesComp.ResourceCount {
			mostResourcesComp = compStats
		}
		if mostAPICallsComp == nil || compStats.APICallCount > mostAPICallsComp.APICallCount {
			mostAPICallsComp = compStats
		}
	}
	
	// Find slowest and fastest resource types
	var slowestRT, fastestRT, mostErrorRT *ResourceTypeStats
	for _, rtStats := range sc.resourceTypeStats {
		if slowestRT == nil || rtStats.ProcessingTime > slowestRT.ProcessingTime {
			slowestRT = rtStats
		}
		if fastestRT == nil || rtStats.ProcessingTime < fastestRT.ProcessingTime {
			fastestRT = rtStats
		}
		if mostErrorRT == nil || rtStats.Errors > mostErrorRT.Errors {
			mostErrorRT = rtStats
		}
	}
	
	// Set analysis fields based on found data
	if slowestRT != nil {
		analysis.SlowestResourceType = slowestRT.ResourceType
	}
	if fastestRT != nil {
		analysis.FastestResourceType = fastestRT.ResourceType
	}
	if mostErrorRT != nil {
		analysis.HighestErrorRate = mostErrorRT.ResourceType
	}
	if mostResourcesComp != nil {
		analysis.MostProductiveComp = mostResourcesComp.CompartmentName
	}
	
	// Generate bottleneck analysis
	if slowestRT != nil && slowestRT.ProcessingTime > 0 {
		analysis.Bottlenecks = append(analysis.Bottlenecks,
			fmt.Sprintf("Resource type '%s' is the slowest with %v processing time", slowestRT.ResourceType, slowestRT.ProcessingTime))
	}
	
	if mostErrorRT != nil && mostErrorRT.Errors > 0 {
		analysis.Bottlenecks = append(analysis.Bottlenecks,
			fmt.Sprintf("Resource type '%s' has the highest error rate with %d errors", mostErrorRT.ResourceType, mostErrorRT.Errors))
	}
	
	// Generate recommendations
	totalErrors := atomic.LoadInt64(&sc.globalErrorCount)
	totalRetries := atomic.LoadInt64(&sc.globalRetryCount)
	totalAPICalls := atomic.LoadInt64(&sc.globalAPICallCount)
	
	if totalErrors > 0 {
		errorRate := float64(totalErrors) / float64(totalAPICalls) * 100
		if errorRate > 5 {
			analysis.Recommendations = append(analysis.Recommendations,
				fmt.Sprintf("High error rate detected (%.2f%%). Consider reviewing API permissions and network connectivity", errorRate))
		}
	}
	
	if totalRetries > 0 {
		retryRate := float64(totalRetries) / float64(totalAPICalls) * 100
		if retryRate > 10 {
			analysis.Recommendations = append(analysis.Recommendations,
				fmt.Sprintf("High retry rate detected (%.2f%%). Consider increasing timeout values or reducing concurrency", retryRate))
		}
	}
	
	if len(sc.compartmentStats) > 10 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Large number of compartments detected. Consider implementing compartment filtering for faster execution")
	}
	
	return analysis
}

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


// Start marks the beginning of statistics collection
func (sc *StatisticsCollector) Start() {
	if !sc.enabled {
		return
	}
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.startTime = time.Now()
}

// Stop marks the end of statistics collection
func (sc *StatisticsCollector) Stop() {
	if !sc.enabled {
		return
	}
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.endTime = time.Now()
}

// RecordResourceTypeStart records the start of resource type processing
func (sc *StatisticsCollector) RecordResourceTypeStart(resourceType string) time.Time {
	if !sc.enabled {
		return time.Now()
	}
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	if sc.resourceTypeStats[resourceType] == nil {
		sc.resourceTypeStats[resourceType] = &ResourceTypeStats{
			FirstSeen: time.Now(),
			MinTime:   time.Hour, // Initialize with large value
		}
	}
	
	return time.Now()
}

// RecordResourceTypeEnd records the completion of resource type processing
func (sc *StatisticsCollector) RecordResourceTypeEnd(resourceType string, startTime time.Time, count int64, errors int64) {
	if !sc.enabled {
		return
	}
	
	duration := time.Since(startTime)
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	stats := sc.resourceTypeStats[resourceType]
	if stats == nil {
		stats = &ResourceTypeStats{
			FirstSeen: startTime,
			MinTime:   duration,
		}
		sc.resourceTypeStats[resourceType] = stats
	}
	
	stats.Count += count
	stats.TotalTime += duration
	stats.Errors += errors
	stats.LastSeen = time.Now()
	
	if duration < stats.MinTime {
		stats.MinTime = duration
	}
	if duration > stats.MaxTime {
		stats.MaxTime = duration
	}
	
	atomic.AddInt64(&sc.totalResources, count)
	atomic.AddInt64(&sc.totalErrors, errors)
	atomic.AddInt64(&sc.totalAPICall, 1)
}

// RecordCompartmentStart records the start of compartment processing
func (sc *StatisticsCollector) RecordCompartmentStart(compartmentName string) {
	if !sc.enabled {
		return
	}
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	sc.compartmentStats[compartmentName] = &CompartmentStats{
		Name:      compartmentName,
		StartTime: time.Now(),
	}
}

// RecordCompartmentEnd records the completion of compartment processing
func (sc *StatisticsCollector) RecordCompartmentEnd(compartmentName string, resourceCount int64, errors int64) {
	if !sc.enabled {
		return
	}
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	stats := sc.compartmentStats[compartmentName]
	if stats != nil {
		stats.EndTime = time.Now()
		stats.ProcessingTime = stats.EndTime.Sub(stats.StartTime)
		stats.ResourceCount = resourceCount
		stats.Errors = errors
	}
}

// RecordRetry records a retry attempt
func (sc *StatisticsCollector) RecordRetry() {
	if !sc.enabled {
		return
	}
	atomic.AddInt64(&sc.totalRetries, 1)
}

// RecordError records an error
func (sc *StatisticsCollector) RecordError() {
	if !sc.enabled {
		return
	}
	atomic.AddInt64(&sc.totalErrors, 1)
}

// GenerateReport generates a comprehensive statistics report
func (sc *StatisticsCollector) GenerateReport() StatisticsReport {
	if !sc.enabled {
		return StatisticsReport{}
	}
	
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	// Calculate execution summary
	totalDuration := sc.endTime.Sub(sc.startTime)
	overallThroughput := float64(atomic.LoadInt64(&sc.totalResources)) / totalDuration.Seconds()
	avgAPILatency := time.Duration(0)
	if sc.totalAPICall > 0 {
		avgAPILatency = totalDuration / time.Duration(sc.totalAPICall)
	}
	
	executionSummary := ExecutionSummary{
		StartTime:         sc.startTime,
		EndTime:           sc.endTime,
		TotalDuration:     totalDuration,
		TotalResources:    atomic.LoadInt64(&sc.totalResources),
		TotalAPICallss:    atomic.LoadInt64(&sc.totalAPICall),
		TotalErrors:       atomic.LoadInt64(&sc.totalErrors),
		TotalRetries:      atomic.LoadInt64(&sc.totalRetries),
		OverallThroughput: overallThroughput,
		AvgAPILatency:     avgAPILatency,
	}
	
	// Copy maps for thread safety
	resourceStats := make(map[string]ResourceTypeStats)
	for k, v := range sc.resourceTypeStats {
		resourceStats[k] = *v
	}
	
	compartmentStats := make(map[string]CompartmentStats)
	for k, v := range sc.compartmentStats {
		compartmentStats[k] = *v
	}
	
	// Generate performance analysis
	performanceAnalysis := sc.generatePerformanceAnalysis()
	
	return StatisticsReport{
		ExecutionSummary:    executionSummary,
		ResourceTypeStats:   resourceStats,
		CompartmentStats:    compartmentStats,
		PerformanceAnalysis: performanceAnalysis,
	}
}

// generatePerformanceAnalysis generates automated performance insights
func (sc *StatisticsCollector) generatePerformanceAnalysis() PerformanceAnalysis {
	var slowestType, fastestType, highestErrorType, mostProductiveComp string
	var maxTime, minTime time.Duration = 0, time.Hour
	var maxErrorRate float64
	var maxResources int64
	
	// Analyze resource types
	for typeName, stats := range sc.resourceTypeStats {
		avgTime := stats.TotalTime / time.Duration(stats.Count)
		if avgTime > maxTime {
			maxTime = avgTime
			slowestType = typeName
		}
		if avgTime < minTime && stats.Count > 0 {
			minTime = avgTime
			fastestType = typeName
		}
		
		if stats.Count > 0 {
			errorRate := float64(stats.Errors) / float64(stats.Count)
			if errorRate > maxErrorRate {
				maxErrorRate = errorRate
				highestErrorType = typeName
			}
		}
	}
	
	// Analyze compartments
	for _, stats := range sc.compartmentStats {
		if stats.ResourceCount > maxResources {
			maxResources = stats.ResourceCount
			mostProductiveComp = stats.Name
		}
	}
	
	// Generate recommendations
	recommendations := []string{}
	bottlenecks := []string{}
	
	if maxErrorRate > 0.1 {
		recommendations = append(recommendations, fmt.Sprintf("High error rate detected in %s (%.1f%%). Consider implementing additional retry logic.", highestErrorType, maxErrorRate*100))
		bottlenecks = append(bottlenecks, fmt.Sprintf("Error rate: %s", highestErrorType))
	}
	
	if maxTime > 30*time.Second {
		recommendations = append(recommendations, fmt.Sprintf("Slow processing detected for %s (avg: %v). Consider optimizing API calls or implementing caching.", slowestType, maxTime))
		bottlenecks = append(bottlenecks, fmt.Sprintf("Processing time: %s", slowestType))
	}
	
	totalErrors := atomic.LoadInt64(&sc.totalErrors)
	totalResources := atomic.LoadInt64(&sc.totalResources)
	if totalErrors > 0 && totalResources > 0 {
		overallErrorRate := float64(totalErrors) / float64(totalResources)
		if overallErrorRate > 0.05 {
			recommendations = append(recommendations, fmt.Sprintf("Overall error rate is %.1f%%. Consider reviewing OCI permissions and network connectivity.", overallErrorRate*100))
		}
	}
	
	return PerformanceAnalysis{
		SlowestResourceType: slowestType,
		FastestResourceType: fastestType,
		HighestErrorRate:    highestErrorType,
		MostProductiveComp:  mostProductiveComp,
		Recommendations:     recommendations,
		Bottlenecks:         bottlenecks,
	}
}

// OutputStatisticsReport outputs the statistics report in the specified format
func OutputStatisticsReport(report StatisticsReport, format StatisticsFormat) error {
	switch format {
	case StatsFormatJSON:
		return outputStatisticsJSON(report)
	case StatsFormatCSV:
		return outputStatisticsCSV(report)
	case StatsFormatText:
		return outputStatisticsText(report)
	default:
		return outputStatisticsText(report)
	}
}

// outputStatisticsJSON outputs statistics in JSON format
func outputStatisticsJSON(report StatisticsReport) error {
	encoder := json.NewEncoder(os.Stderr)
	encoder.SetIndent("", "  ")
	fmt.Fprint(os.Stderr, "\n=== STATISTICS REPORT ===\n")
	return encoder.Encode(report)
}

// outputStatisticsCSV outputs statistics in CSV format
func outputStatisticsCSV(report StatisticsReport) error {
	writer := csv.NewWriter(os.Stderr)
	defer writer.Flush()
	
	fmt.Fprint(os.Stderr, "\n=== STATISTICS REPORT (CSV) ===\n")
	
	// Execution Summary
	fmt.Fprint(os.Stderr, "\nExecution Summary:\n")
	summaryHeader := []string{"Metric", "Value"}
	writer.Write(summaryHeader)
	
	writer.Write([]string{"Total Duration", report.ExecutionSummary.TotalDuration.String()})
	writer.Write([]string{"Total Resources", fmt.Sprintf("%d", report.ExecutionSummary.TotalResources)})
	writer.Write([]string{"Total API Calls", fmt.Sprintf("%d", report.ExecutionSummary.TotalAPICallss)})
	writer.Write([]string{"Total Errors", fmt.Sprintf("%d", report.ExecutionSummary.TotalErrors)})
	writer.Write([]string{"Total Retries", fmt.Sprintf("%d", report.ExecutionSummary.TotalRetries)})
	writer.Write([]string{"Overall Throughput", fmt.Sprintf("%.2f res/sec", report.ExecutionSummary.OverallThroughput)})
	writer.Flush()
	
	// Resource Type Statistics
	fmt.Fprint(os.Stderr, "\nResource Type Statistics:\n")
	resourceHeader := []string{"Resource Type", "Count", "Total Time", "Avg Time", "Min Time", "Max Time", "Errors"}
	writer.Write(resourceHeader)
	
	for resourceType, stats := range report.ResourceTypeStats {
		avgTime := time.Duration(0)
		if stats.Count > 0 {
			avgTime = stats.TotalTime / time.Duration(stats.Count)
		}
		
		writer.Write([]string{
			resourceType,
			fmt.Sprintf("%d", stats.Count),
			stats.TotalTime.String(),
			avgTime.String(),
			stats.MinTime.String(),
			stats.MaxTime.String(),
			fmt.Sprintf("%d", stats.Errors),
		})
	}
	writer.Flush()
	
	return nil
}

// outputStatisticsText outputs statistics in human-readable text format
func outputStatisticsText(report StatisticsReport) error {
	fmt.Fprint(os.Stderr, "\n")
	fmt.Fprint(os.Stderr, "╔══════════════════════════════════════════════════════════════════════════════╗\n")
	fmt.Fprint(os.Stderr, "║                             STATISTICS REPORT                              ║\n")
	fmt.Fprint(os.Stderr, "╚══════════════════════════════════════════════════════════════════════════════╝\n")
	fmt.Fprint(os.Stderr, "\n")
	
	// Execution Summary
	fmt.Fprint(os.Stderr, "📊 EXECUTION SUMMARY\n")
	fmt.Fprint(os.Stderr, "═══════════════════════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(os.Stderr, "• Start Time:       %s\n", report.ExecutionSummary.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(os.Stderr, "• End Time:         %s\n", report.ExecutionSummary.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(os.Stderr, "• Total Duration:   %v\n", report.ExecutionSummary.TotalDuration)
	fmt.Fprintf(os.Stderr, "• Total Resources:  %d\n", report.ExecutionSummary.TotalResources)
	fmt.Fprintf(os.Stderr, "• Total API Calls:  %d\n", report.ExecutionSummary.TotalAPICallss)
	fmt.Fprintf(os.Stderr, "• Total Errors:     %d\n", report.ExecutionSummary.TotalErrors)
	fmt.Fprintf(os.Stderr, "• Total Retries:    %d\n", report.ExecutionSummary.TotalRetries)
	fmt.Fprintf(os.Stderr, "• Throughput:       %.2f resources/second\n", report.ExecutionSummary.OverallThroughput)
	fmt.Fprintf(os.Stderr, "• Avg API Latency:  %v\n", report.ExecutionSummary.AvgAPILatency)
	fmt.Fprint(os.Stderr, "\n")
	
	// Resource Type Statistics
	if len(report.ResourceTypeStats) > 0 {
		fmt.Fprint(os.Stderr, "📈 RESOURCE TYPE STATISTICS\n")
		fmt.Fprint(os.Stderr, "═══════════════════════════════════════════════════════════════════════════════\n")
		fmt.Fprintf(os.Stderr, "%-20s %8s %12s %12s %12s %12s %8s\n", 
			"Resource Type", "Count", "Total Time", "Avg Time", "Min Time", "Max Time", "Errors")
		fmt.Fprint(os.Stderr, "───────────────────────────────────────────────────────────────────────────────\n")
		
		for resourceType, stats := range report.ResourceTypeStats {
			avgTime := time.Duration(0)
			if stats.Count > 0 {
				avgTime = stats.TotalTime / time.Duration(stats.Count)
			}
			
			fmt.Fprintf(os.Stderr, "%-20s %8d %12v %12v %12v %12v %8d\n",
				resourceType, stats.Count, 
				stats.TotalTime.Truncate(time.Millisecond),
				avgTime.Truncate(time.Millisecond),
				stats.MinTime.Truncate(time.Millisecond),
				stats.MaxTime.Truncate(time.Millisecond),
				stats.Errors)
		}
		fmt.Fprint(os.Stderr, "\n")
	}
	
	// Compartment Statistics
	if len(report.CompartmentStats) > 0 {
		fmt.Fprint(os.Stderr, "🏢 COMPARTMENT STATISTICS\n")
		fmt.Fprint(os.Stderr, "═══════════════════════════════════════════════════════════════════════════════\n")
		fmt.Fprintf(os.Stderr, "%-25s %12s %15s %8s\n", "Compartment", "Resources", "Processing Time", "Errors")
		fmt.Fprint(os.Stderr, "───────────────────────────────────────────────────────────────────────────────\n")
		
		for _, stats := range report.CompartmentStats {
			fmt.Fprintf(os.Stderr, "%-25s %12d %15v %8d\n",
				stats.Name, stats.ResourceCount,
				stats.ProcessingTime.Truncate(time.Millisecond),
				stats.Errors)
		}
		fmt.Fprint(os.Stderr, "\n")
	}
	
	// Performance Analysis
	fmt.Fprint(os.Stderr, "🔍 PERFORMANCE ANALYSIS\n")
	fmt.Fprint(os.Stderr, "═══════════════════════════════════════════════════════════════════════════════\n")
	if report.PerformanceAnalysis.SlowestResourceType != "" {
		fmt.Fprintf(os.Stderr, "• Slowest Resource Type:    %s\n", report.PerformanceAnalysis.SlowestResourceType)
	}
	if report.PerformanceAnalysis.FastestResourceType != "" {
		fmt.Fprintf(os.Stderr, "• Fastest Resource Type:    %s\n", report.PerformanceAnalysis.FastestResourceType)
	}
	if report.PerformanceAnalysis.HighestErrorRate != "" {
		fmt.Fprintf(os.Stderr, "• Highest Error Rate:       %s\n", report.PerformanceAnalysis.HighestErrorRate)
	}
	if report.PerformanceAnalysis.MostProductiveComp != "" {
		fmt.Fprintf(os.Stderr, "• Most Productive Comp.:    %s\n", report.PerformanceAnalysis.MostProductiveComp)
	}
	
	if len(report.PerformanceAnalysis.Bottlenecks) > 0 {
		fmt.Fprint(os.Stderr, "\n⚠️  IDENTIFIED BOTTLENECKS:\n")
		for _, bottleneck := range report.PerformanceAnalysis.Bottlenecks {
			fmt.Fprintf(os.Stderr, "   • %s\n", bottleneck)
		}
	}
	
	if len(report.PerformanceAnalysis.Recommendations) > 0 {
		fmt.Fprint(os.Stderr, "\n💡 RECOMMENDATIONS:\n")
		for _, rec := range report.PerformanceAnalysis.Recommendations {
			fmt.Fprintf(os.Stderr, "   • %s\n", rec)
		}
	}
	
	fmt.Fprint(os.Stderr, "\n")
	return nil
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

// Statistics output functions

// OutputStatisticsText outputs statistics in human-readable text format
func OutputStatisticsText(stats *StatisticsReport, writer io.Writer) error {
	if stats == nil {
		return fmt.Errorf("no statistics available")
	}
	
	fmt.Fprintf(writer, "\n=== OCI Resource Discovery Statistics Report ===\n")
	fmt.Fprintf(writer, "Generated at: %s\n", stats.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(writer, "CLI Version: %s\n\n", stats.CliVersion)
	
	// Execution Summary
	fmt.Fprintf(writer, "--- Execution Summary ---\n")
	fmt.Fprintf(writer, "Start Time: %s\n", stats.ExecutionSummary.StartTime.Format(time.RFC3339))
	fmt.Fprintf(writer, "End Time: %s\n", stats.ExecutionSummary.EndTime.Format(time.RFC3339))
	fmt.Fprintf(writer, "Total Execution Time: %v\n", stats.ExecutionSummary.TotalExecutionTime)
	fmt.Fprintf(writer, "Total Resources Found: %d\n", stats.ExecutionSummary.TotalResourcesFound)
	fmt.Fprintf(writer, "Total API Calls: %d\n", stats.ExecutionSummary.TotalAPICallsExecuted)
	fmt.Fprintf(writer, "Total Errors: %d\n", stats.ExecutionSummary.TotalErrorsEncountered)
	fmt.Fprintf(writer, "Total Retries: %d\n", stats.ExecutionSummary.TotalRetriesPerformed)
	fmt.Fprintf(writer, "Average Throughput: %.2f resources/sec\n", stats.ExecutionSummary.AverageThroughput)
	fmt.Fprintf(writer, "Peak Throughput: %.2f resources/sec\n", stats.ExecutionSummary.PeakThroughput)
	fmt.Fprintf(writer, "Compartments Processed: %d\n", stats.ExecutionSummary.CompartmentCount)
	fmt.Fprintf(writer, "Resource Types: %d\n", stats.ExecutionSummary.ResourceTypeCount)
	fmt.Fprintf(writer, "Concurrency Level: %d\n", stats.ExecutionSummary.ConcurrencyLevel)
	fmt.Fprintf(writer, "Timeout Configuration: %v\n\n", stats.ExecutionSummary.TimeoutConfiguration)
	
	// Resource Type Statistics
	fmt.Fprintf(writer, "--- Resource Type Statistics ---\n")
	fmt.Fprintf(writer, "%-25s | %8s | %12s | %10s | %8s | %8s | %12s\n",
		"Resource Type", "Found", "Proc Time", "API Calls", "Errors", "Retries", "Throughput")
	fmt.Fprintf(writer, "%s\n", strings.Repeat("-", 100))
	
	// Sort resource types by processing time (descending)
	var sortedResourceTypes []*ResourceTypeStats
	for _, rtStats := range stats.ResourceTypeStats {
		sortedResourceTypes = append(sortedResourceTypes, rtStats)
	}
	sort.Slice(sortedResourceTypes, func(i, j int) bool {
		return sortedResourceTypes[i].ProcessingTime > sortedResourceTypes[j].ProcessingTime
	})
	
	for _, rtStats := range sortedResourceTypes {
		fmt.Fprintf(writer, "%-25s | %8d | %12v | %10d | %8d | %8d | %9.2f/s\n",
			rtStats.ResourceType,
			rtStats.DiscoveryCount,
			rtStats.ProcessingTime,
			rtStats.APICallCount,
			rtStats.ErrorCount,
			rtStats.RetryCount,
			rtStats.Throughput)
	}
	
	// Compartment Statistics
	fmt.Fprintf(writer, "\n--- Compartment Statistics ---\n")
	fmt.Fprintf(writer, "%-30s | %8s | %12s | %10s | %8s | %8s\n",
		"Compartment", "Resources", "Proc Time", "API Calls", "Errors", "Retries")
	fmt.Fprintf(writer, "%s\n", strings.Repeat("-", 90))
	
	// Sort compartments by processing time (descending)
	var sortedCompartments []*CompartmentStats
	for _, compStats := range stats.CompartmentStats {
		sortedCompartments = append(sortedCompartments, compStats)
	}
	sort.Slice(sortedCompartments, func(i, j int) bool {
		return sortedCompartments[i].ProcessingTime > sortedCompartments[j].ProcessingTime
	})
	
	for _, compStats := range sortedCompartments {
		compartmentName := compStats.CompartmentName
		if len(compartmentName) > 30 {
			compartmentName = compartmentName[:27] + "..."
		}
		fmt.Fprintf(writer, "%-30s | %8d | %12v | %10d | %8d | %8d\n",
			compartmentName,
			compStats.ResourceCount,
			compStats.ProcessingTime,
			compStats.APICallCount,
			compStats.ErrorCount,
			compStats.RetryCount)
	}
	
	// Performance Analysis
	fmt.Fprintf(writer, "\n--- Performance Analysis ---\n")
	analysis := stats.PerformanceAnalysis
	
	if analysis.SlowestCompartment != nil {
		fmt.Fprintf(writer, "Slowest Compartment: %s (%v)\n",
			analysis.SlowestCompartment.CompartmentName,
			analysis.SlowestCompartment.ProcessingTime)
	}
	
	if analysis.FastestCompartment != nil {
		fmt.Fprintf(writer, "Fastest Compartment: %s (%v)\n",
			analysis.FastestCompartment.CompartmentName,
			analysis.FastestCompartment.ProcessingTime)
	}
	
	if analysis.MostResourcesFound != nil {
		fmt.Fprintf(writer, "Most Resources Found: %s (%d resources)\n",
			analysis.MostResourcesFound.CompartmentName,
			analysis.MostResourcesFound.ResourceCount)
	}
	
	if analysis.SlowestResourceType != nil {
		fmt.Fprintf(writer, "Slowest Resource Type: %s (%v)\n",
			analysis.SlowestResourceType.ResourceType,
			analysis.SlowestResourceType.ProcessingTime)
	}
	
	if analysis.MostErrorProneType != nil && analysis.MostErrorProneType.ErrorCount > 0 {
		fmt.Fprintf(writer, "Most Error-Prone Type: %s (%d errors)\n",
			analysis.MostErrorProneType.ResourceType,
			analysis.MostErrorProneType.ErrorCount)
	}
	
	// Bottleneck Analysis
	if len(analysis.BottleneckAnalysis) > 0 {
		fmt.Fprintf(writer, "\nBottleneck Analysis:\n")
		for i, bottleneck := range analysis.BottleneckAnalysis {
			fmt.Fprintf(writer, "  %d. %s\n", i+1, bottleneck)
		}
	}
	
	// Recommendations
	if len(analysis.Recommendations) > 0 {
		fmt.Fprintf(writer, "\nRecommendations:\n")
		for i, recommendation := range analysis.Recommendations {
			fmt.Fprintf(writer, "  %d. %s\n", i+1, recommendation)
		}
	}
	
	fmt.Fprintf(writer, "\n=== End of Statistics Report ===\n")
	return nil
}

// OutputStatisticsJSON outputs statistics in JSON format
func OutputStatisticsJSON(stats *StatisticsReport, writer io.Writer) error {
	if stats == nil {
		return fmt.Errorf("no statistics available")
	}
	
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(stats)
}

// OutputStatisticsCSV outputs statistics in CSV format
func OutputStatisticsCSV(stats *StatisticsReport, writer io.Writer) error {
	if stats == nil {
		return fmt.Errorf("no statistics available")
	}
	
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()
	
	// Write execution summary
	if err := csvWriter.Write([]string{"Section", "Metric", "Value"}); err != nil {
		return err
	}
	
	execSummary := stats.ExecutionSummary
	records := [][]string{
		{"Execution Summary", "Start Time", execSummary.StartTime.Format(time.RFC3339)},
		{"Execution Summary", "End Time", execSummary.EndTime.Format(time.RFC3339)},
		{"Execution Summary", "Total Execution Time (ms)", strconv.FormatInt(execSummary.TotalExecutionTime.Milliseconds(), 10)},
		{"Execution Summary", "Total Resources Found", strconv.FormatInt(execSummary.TotalResourcesFound, 10)},
		{"Execution Summary", "Total API Calls", strconv.FormatInt(execSummary.TotalAPICallsExecuted, 10)},
		{"Execution Summary", "Total Errors", strconv.FormatInt(execSummary.TotalErrorsEncountered, 10)},
		{"Execution Summary", "Total Retries", strconv.FormatInt(execSummary.TotalRetriesPerformed, 10)},
		{"Execution Summary", "Average Throughput (resources/sec)", fmt.Sprintf("%.2f", execSummary.AverageThroughput)},
		{"Execution Summary", "Peak Throughput (resources/sec)", fmt.Sprintf("%.2f", execSummary.PeakThroughput)},
		{"Execution Summary", "Compartments Processed", strconv.FormatInt(execSummary.CompartmentCount, 10)},
		{"Execution Summary", "Resource Types", strconv.FormatInt(execSummary.ResourceTypeCount, 10)},
		{"Execution Summary", "Concurrency Level", strconv.Itoa(execSummary.ConcurrencyLevel)},
	}
	
	for _, record := range records {
		if err := csvWriter.Write(record); err != nil {
			return err
		}
	}
	
	// Write resource type statistics
	for _, rtStats := range stats.ResourceTypeStats {
		records := [][]string{
			{"Resource Type", rtStats.ResourceType + " - Discovery Count", strconv.FormatInt(rtStats.DiscoveryCount, 10)},
			{"Resource Type", rtStats.ResourceType + " - Processing Time (ms)", strconv.FormatInt(rtStats.ProcessingTime.Milliseconds(), 10)},
			{"Resource Type", rtStats.ResourceType + " - API Call Count", strconv.FormatInt(rtStats.APICallCount, 10)},
			{"Resource Type", rtStats.ResourceType + " - Error Count", strconv.FormatInt(rtStats.ErrorCount, 10)},
			{"Resource Type", rtStats.ResourceType + " - Retry Count", strconv.FormatInt(rtStats.RetryCount, 10)},
			{"Resource Type", rtStats.ResourceType + " - Throughput (resources/sec)", fmt.Sprintf("%.2f", rtStats.Throughput)},
		}
		
		for _, record := range records {
			if err := csvWriter.Write(record); err != nil {
				return err
			}
		}
	}
	
	// Write compartment statistics
	for _, compStats := range stats.CompartmentStats {
		records := [][]string{
			{"Compartment", compStats.CompartmentName + " - Resource Count", strconv.FormatInt(compStats.ResourceCount, 10)},
			{"Compartment", compStats.CompartmentName + " - Processing Time (ms)", strconv.FormatInt(compStats.ProcessingTime.Milliseconds(), 10)},
			{"Compartment", compStats.CompartmentName + " - API Call Count", strconv.FormatInt(compStats.APICallCount, 10)},
			{"Compartment", compStats.CompartmentName + " - Error Count", strconv.FormatInt(compStats.ErrorCount, 10)},
			{"Compartment", compStats.CompartmentName + " - Retry Count", strconv.FormatInt(compStats.RetryCount, 10)},
		}
		
		for _, record := range records {
			if err := csvWriter.Write(record); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// OutputStatistics outputs statistics in the specified format
func OutputStatistics(stats *StatisticsReport, format StatisticsFormat, writer io.Writer) error {
	switch format {
	case StatsFormatText:
		return OutputStatisticsText(stats, writer)
	case StatsFormatJSON:
		return OutputStatisticsJSON(stats, writer)
	case StatsFormatCSV:
		return OutputStatisticsCSV(stats, writer)
	default:
		return fmt.Errorf("unsupported statistics format: %s", format)
	}
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

func discoverComputeInstancesWithStats(ctx context.Context, clients *OCIClients, compartmentID, compartmentName string, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allInstances []core.Instance
	startTime := time.Now()
	apiCallCount := int64(0)

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

		apiStartTime := time.Now()
		resp, err := clients.ComputeClient.ListInstances(ctx, req)
		apiCallCount++
		apiLatency := time.Since(apiStartTime)
		
		if err != nil {
			if statsCollector != nil {
				statsCollector.RecordStatistics(StatisticsUpdate{
					CompartmentID:   compartmentID,
					CompartmentName: compartmentName,
					ResourceType:    "compute_instance",
					APICallCount:    1,
					ErrorCount:      1,
					Latency:         apiLatency,
					OperationType:   "error",
				})
			}
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

	processingTime := time.Since(startTime)
	
	// Record statistics
	if statsCollector != nil {
		statsCollector.RecordStatistics(StatisticsUpdate{
			CompartmentID:   compartmentID,
			CompartmentName: compartmentName,
			ResourceType:    "compute_instance",
			ResourceCount:   int64(len(resources)),
			ProcessingTime:  processingTime,
			APICallCount:    apiCallCount,
			OperationType:   "complete",
		})
	}
	
	logger.Debug("Completed compute instances discovery for compartment %s: found %d instances across %d pages", compartmentID, len(resources), pageCount)
	return resources, nil
}

func discoverVCNs(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	return discoverVCNsWithStats(ctx, clients, compartmentID, "", nil)
}

func discoverVCNsWithStats(ctx context.Context, clients *OCIClients, compartmentID, compartmentName string, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allVcns []core.Vcn
	startTime := time.Now()
	apiCallCount := int64(0)

	// Implement pagination to get all VCNs
	var page *string
	for {
		req := core.ListVcnsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		apiStartTime := time.Now()
		resp, err := clients.VirtualNetworkClient.ListVcns(ctx, req)
		apiCallCount++
		apiLatency := time.Since(apiStartTime)
		
		if err != nil {
			if statsCollector != nil {
				statsCollector.RecordStatistics(StatisticsUpdate{
					CompartmentID:   compartmentID,
					CompartmentName: compartmentName,
					ResourceType:    "vcn",
					APICallCount:    1,
					ErrorCount:      1,
					Latency:         apiLatency,
					OperationType:   "error",
				})
			}
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

	processingTime := time.Since(startTime)
	
	// Record statistics
	if statsCollector != nil {
		statsCollector.RecordStatistics(StatisticsUpdate{
			CompartmentID:   compartmentID,
			CompartmentName: compartmentName,
			ResourceType:    "vcn",
			ResourceCount:   int64(len(resources)),
			ProcessingTime:  processingTime,
			APICallCount:    apiCallCount,
			OperationType:   "complete",
		})
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
	return discoverAutonomousDatabasesWithStats(ctx, clients, compartmentID, "", nil)
}

func discoverAutonomousDatabasesWithStats(ctx context.Context, clients *OCIClients, compartmentID, compartmentName string, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allAutonomousDatabases []database.AutonomousDatabaseSummary
	startTime := time.Now()
	apiCallCount := int64(0)

	// Implement pagination to get all Autonomous Databases
	var page *string
	for {
		req := database.ListAutonomousDatabasesRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		apiStartTime := time.Now()
		resp, err := clients.DatabaseClient.ListAutonomousDatabases(ctx, req)
		apiCallCount++
		apiLatency := time.Since(apiStartTime)
		
		if err != nil {
			if statsCollector != nil {
				statsCollector.RecordStatistics(StatisticsUpdate{
					CompartmentID:   compartmentID,
					CompartmentName: compartmentName,
					ResourceType:    "autonomous_database",
					APICallCount:    1,
					ErrorCount:      1,
					Latency:         apiLatency,
					OperationType:   "error",
				})
			}
			return nil, err
		}

		allAutonomousDatabases = append(allAutonomousDatabases, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, adb := range allAutonomousDatabases {
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
			if adb.DbWorkload != "" {
				additionalInfo["workload_type"] = string(adb.DbWorkload)
			}
			
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

	processingTime := time.Since(startTime)
	
	// Record statistics
	if statsCollector != nil {
		statsCollector.RecordStatistics(StatisticsUpdate{
			CompartmentID:   compartmentID,
			CompartmentName: compartmentName,
			ResourceType:    "autonomous_database",
			ResourceCount:   int64(len(resources)),
			ProcessingTime:  processingTime,
			APICallCount:    apiCallCount,
			OperationType:   "complete",
		})
	}

	return resources, nil
}

func discoverFunctions(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	return discoverFunctionsWithStats(ctx, clients, compartmentID, "", nil)
}

func discoverFunctionsWithStats(ctx context.Context, clients *OCIClients, compartmentID, compartmentName string, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allApplications []functions.ApplicationSummary
	startTime := time.Now()
	apiCallCount := int64(0)

	// First get all applications
	var page *string
	for {
		req := functions.ListApplicationsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		apiStartTime := time.Now()
		resp, err := clients.FunctionsClient.ListApplications(ctx, req)
		apiCallCount++
		apiLatency := time.Since(apiStartTime)
		
		if err != nil {
			if statsCollector != nil {
				statsCollector.RecordStatistics(StatisticsUpdate{
					CompartmentID:   compartmentID,
					CompartmentName: compartmentName,
					ResourceType:    "function_application",
					APICallCount:    1,
					ErrorCount:      1,
					Latency:         apiLatency,
					OperationType:   "error",
				})
			}
			return nil, err
		}

		allApplications = append(allApplications, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	// For each application, get its functions
	for _, app := range allApplications {
		if app.LifecycleState != functions.ApplicationSummaryLifecycleStateDeleted {
			// Add application as a resource
			appName := ""
			if app.DisplayName != nil {
				appName = *app.DisplayName
			}
			appOcid := ""
			if app.Id != nil {
				appOcid = *app.Id
			}

			appAdditionalInfo := make(map[string]interface{})
			if app.SubnetIds != nil && len(app.SubnetIds) > 0 {
				appAdditionalInfo["subnet_ids"] = app.SubnetIds
			}

			resources = append(resources, ResourceInfo{
				ResourceType:   "function_application",
				ResourceName:   appName,
				OCID:          appOcid,
				CompartmentID: compartmentID,
				AdditionalInfo: appAdditionalInfo,
			})

			// Get functions in this application
			if app.Id != nil {
				var funcPage *string
				for {
					funcReq := functions.ListFunctionsRequest{
						ApplicationId: app.Id,
						Page:         funcPage,
					}

					apiStartTime := time.Now()
					funcResp, err := clients.FunctionsClient.ListFunctions(ctx, funcReq)
					apiCallCount++
					
					if err != nil {
						continue // Skip functions if we can't list them
					}

					for _, fn := range funcResp.Items {
						if fn.LifecycleState != functions.FunctionSummaryLifecycleStateDeleted {
							funcName := ""
							if fn.DisplayName != nil {
								funcName = *fn.DisplayName
							}
							funcOcid := ""
							if fn.Id != nil {
								funcOcid = *fn.Id
							}

							funcAdditionalInfo := make(map[string]interface{})
							
							// Add runtime
							if fn.Image != nil {
								funcAdditionalInfo["image"] = *fn.Image
							}
							
							// Add memory in MBs
							if fn.MemoryInMBs != nil {
								funcAdditionalInfo["memory_mb"] = *fn.MemoryInMBs
							}
							
							// Add timeout
							if fn.TimeoutInSeconds != nil {
								funcAdditionalInfo["timeout_seconds"] = *fn.TimeoutInSeconds
							}

							resources = append(resources, ResourceInfo{
								ResourceType:   "function",
								ResourceName:   funcName,
								OCID:          funcOcid,
								CompartmentID: compartmentID,
								AdditionalInfo: funcAdditionalInfo,
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

	processingTime := time.Since(startTime)
	
	// Record statistics
	if statsCollector != nil {
		statsCollector.RecordStatistics(StatisticsUpdate{
			CompartmentID:   compartmentID,
			CompartmentName: compartmentName,
			ResourceType:    "function",
			ResourceCount:   int64(len(resources)),
			ProcessingTime:  processingTime,
			APICallCount:    apiCallCount,
			OperationType:   "complete",
		})
	}

	return resources, nil
}

func discoverAPIGateways(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	return discoverAPIGatewaysWithStats(ctx, clients, compartmentID, "", nil)
}

func discoverAPIGatewaysWithStats(ctx context.Context, clients *OCIClients, compartmentID, compartmentName string, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allGateways []apigateway.GatewaySummary
	startTime := time.Now()
	apiCallCount := int64(0)

	// Implement pagination to get all API Gateways
	var page *string
	for {
		req := apigateway.ListGatewaysRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		apiStartTime := time.Now()
		resp, err := clients.APIGatewayClient.ListGateways(ctx, req)
		apiCallCount++
		apiLatency := time.Since(apiStartTime)
		
		if err != nil {
			if statsCollector != nil {
				statsCollector.RecordStatistics(StatisticsUpdate{
					CompartmentID:   compartmentID,
					CompartmentName: compartmentName,
					ResourceType:    "api_gateway",
					APICallCount:    1,
					ErrorCount:      1,
					Latency:         apiLatency,
					OperationType:   "error",
				})
			}
			return nil, err
		}

		allGateways = append(allGateways, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, gateway := range allGateways {
		if gateway.LifecycleState != apigateway.GatewaySummaryLifecycleStateDeleted {
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
			if gateway.EndpointType != "" {
				additionalInfo["endpoint_type"] = string(gateway.EndpointType)
			}
			
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

	processingTime := time.Since(startTime)
	
	// Record statistics
	if statsCollector != nil {
		statsCollector.RecordStatistics(StatisticsUpdate{
			CompartmentID:   compartmentID,
			CompartmentName: compartmentName,
			ResourceType:    "api_gateway",
			ResourceCount:   int64(len(resources)),
			ProcessingTime:  processingTime,
			APICallCount:    apiCallCount,
			OperationType:   "complete",
		})
	}

	return resources, nil
}

func discoverFileStorageSystems(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	return discoverFileStorageSystemsWithStats(ctx, clients, compartmentID, "", nil)
}

func discoverFileStorageSystemsWithStats(ctx context.Context, clients *OCIClients, compartmentID, compartmentName string, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allFileSystems []filestorage.FileSystemSummary
	startTime := time.Now()
	apiCallCount := int64(0)

	// Implement pagination to get all File Systems
	var page *string
	for {
		req := filestorage.ListFileSystemsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		apiStartTime := time.Now()
		resp, err := clients.FileStorageClient.ListFileSystems(ctx, req)
		apiCallCount++
		apiLatency := time.Since(apiStartTime)
		
		if err != nil {
			if statsCollector != nil {
				statsCollector.RecordStatistics(StatisticsUpdate{
					CompartmentID:   compartmentID,
					CompartmentName: compartmentName,
					ResourceType:    "file_storage_system",
					APICallCount:    1,
					ErrorCount:      1,
					Latency:         apiLatency,
					OperationType:   "error",
				})
			}
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

	processingTime := time.Since(startTime)
	
	// Record statistics
	if statsCollector != nil {
		statsCollector.RecordStatistics(StatisticsUpdate{
			CompartmentID:   compartmentID,
			CompartmentName: compartmentName,
			ResourceType:    "file_storage_system",
			ResourceCount:   int64(len(resources)),
			ProcessingTime:  processingTime,
			APICallCount:    apiCallCount,
			OperationType:   "complete",
		})
	}

	return resources, nil
}

func discoverNetworkLoadBalancers(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	return discoverNetworkLoadBalancersWithStats(ctx, clients, compartmentID, "", nil)
}

func discoverNetworkLoadBalancersWithStats(ctx context.Context, clients *OCIClients, compartmentID, compartmentName string, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allNLBs []networkloadbalancer.NetworkLoadBalancerSummary
	startTime := time.Now()
	apiCallCount := int64(0)

	// Implement pagination to get all Network Load Balancers
	var page *string
	for {
		req := networkloadbalancer.ListNetworkLoadBalancersRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		apiStartTime := time.Now()
		resp, err := clients.NetworkLoadBalancerClient.ListNetworkLoadBalancers(ctx, req)
		apiCallCount++
		apiLatency := time.Since(apiStartTime)
		
		if err != nil {
			if statsCollector != nil {
				statsCollector.RecordStatistics(StatisticsUpdate{
					CompartmentID:   compartmentID,
					CompartmentName: compartmentName,
					ResourceType:    "network_load_balancer",
					APICallCount:    1,
					ErrorCount:      1,
					Latency:         apiLatency,
					OperationType:   "error",
				})
			}
			return nil, err
		}

		allNLBs = append(allNLBs, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}

	for _, nlb := range allNLBs {
		if nlb.LifecycleState != networkloadbalancer.NetworkLoadBalancerSummaryLifecycleStateDeleted {
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
				for _, ipAddr := range nlb.IpAddresses {
					if ipAddr.IpAddress != nil {
						ipAddresses = append(ipAddresses, *ipAddr.IpAddress)
					}
				}
				if len(ipAddresses) > 0 {
					additionalInfo["ip_addresses"] = ipAddresses
				}
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

	processingTime := time.Since(startTime)
	
	// Record statistics
	if statsCollector != nil {
		statsCollector.RecordStatistics(StatisticsUpdate{
			CompartmentID:   compartmentID,
			CompartmentName: compartmentName,
			ResourceType:    "network_load_balancer",
			ResourceCount:   int64(len(resources)),
			ProcessingTime:  processingTime,
			APICallCount:    apiCallCount,
			OperationType:   "complete",
		})
	}

	return resources, nil
}

func discoverStreams(ctx context.Context, clients *OCIClients, compartmentID string) ([]ResourceInfo, error) {
	return discoverStreamsWithStats(ctx, clients, compartmentID, "", nil)
}

func discoverStreamsWithStats(ctx context.Context, clients *OCIClients, compartmentID, compartmentName string, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var resources []ResourceInfo
	var allStreams []streaming.StreamSummary
	startTime := time.Now()
	apiCallCount := int64(0)

	// Implement pagination to get all Streams
	var page *string
	for {
		req := streaming.ListStreamsRequest{
			CompartmentId: common.String(compartmentID),
			Page:         page,
		}

		apiStartTime := time.Now()
		resp, err := clients.StreamingClient.ListStreams(ctx, req)
		apiCallCount++
		apiLatency := time.Since(apiStartTime)
		
		if err != nil {
			if statsCollector != nil {
				statsCollector.RecordStatistics(StatisticsUpdate{
					CompartmentID:   compartmentID,
					CompartmentName: compartmentName,
					ResourceType:    "stream",
					APICallCount:    1,
					ErrorCount:      1,
					Latency:         apiLatency,
					OperationType:   "error",
				})
			}
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
			
			// Add retention in hours
			if stream.RetentionInHours != nil {
				additionalInfo["retention_hours"] = *stream.RetentionInHours
			}
			
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

	processingTime := time.Since(startTime)
	
	// Record statistics
	if statsCollector != nil {
		statsCollector.RecordStatistics(StatisticsUpdate{
			CompartmentID:   compartmentID,
			CompartmentName: compartmentName,
			ResourceType:    "stream",
			ResourceCount:   int64(len(resources)),
			ProcessingTime:  processingTime,
			APICallCount:    apiCallCount,
			OperationType:   "complete",
		})
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
	return discoverAllResourcesWithProgressAndStats(ctx, clients, progressTracker, nil)
}

func discoverAllResourcesWithProgressAndStats(ctx context.Context, clients *OCIClients, progressTracker *ProgressTracker, statsCollector *StatisticsCollector) ([]ResourceInfo, error) {
	var allResources []ResourceInfo
	var resourcesMutex sync.Mutex

	// Start progress tracking
	if progressTracker != nil {
		progressTracker.Start()
		defer progressTracker.Stop()
	}

	// Get all compartments
	if progressTracker != nil {
		progressTracker.Update(ProgressUpdate{
			Operation: "Discovering compartments",
		})
	}
	logger.Info("Getting compartments...")
	logger.Debug("Starting compartment discovery with context timeout: %v", ctx)
	compartments, err := getCompartments(ctx, clients)
	if err != nil {
		return nil, fmt.Errorf("failed to get compartments: %w", err)
	}
	logger.Info("Found %d compartments", len(compartments))
	logger.Verbose("Compartment discovery completed successfully")
	
	// Set progress tracker totals (disabled)
	// // progressTracker.SetTotalCompartments(len(compartments))

	// Create a worker pool for parallel compartment processing
	maxWorkers := 5  // Reasonable limit to avoid API rate limiting
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	// Discover resources in each compartment concurrently
	for i, compartment := range compartments {
		wg.Add(1)
		go func(idx int, comp identity.Compartment) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			compartmentID := *comp.Id
			compartmentName := "root"
			if comp.Name != nil {
				compartmentName = *comp.Name
			}
			
			logger.Info("Processing compartment %d/%d: %s", idx+1, len(compartments), compartmentName)
			logger.Debug("Processing compartment with ID: %s", compartmentID)
			
			// Update progress tracker with current compartment
			if progressTracker != nil {
				progressTracker.Update(ProgressUpdate{
					CompartmentName: compartmentName,
					Operation:      "Processing compartment",
				})
			}

			var compartmentResources []ResourceInfo
			
			// Discover compute instances with retry
			// Progress update for compute instances discovery
			if progressTracker != nil {
				progressTracker.Update(ProgressUpdate{
					CompartmentName: compartmentName,
					Operation:      "Discovering compute instances",
				})
			}
			logger.Verbose("  Discovering compute instances in compartment: %s", compartmentName)
			logger.Debug("  Starting compute instance discovery with retry mechanism")
			var instances []ResourceInfo
			err := withRetryAndProgress(ctx, func() error {
				var retryErr error
				instances, retryErr = discoverComputeInstancesWithStats(ctx, clients, compartmentID, compartmentName, statsCollector)
				return retryErr
			}, 3, "compute instances discovery", progressTracker)
			
			if err == nil {
				compartmentResources = append(compartmentResources, instances...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "Compute instances discovered",
						ResourceCount:  int64(len(instances)),
					})
				}
				logger.Info("  Found %d compute instances", len(instances))
				logger.Debug("  Compute instances discovery completed successfully")
			} else if !isRetriableError(err) {
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{IsError: true})
				}
				logger.Error("Failed to discover compute instances in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping compute instances in compartment %s due to retriable error: %v", compartmentName, err)
			}

			// Discover VCNs
			if progressTracker != nil {
				progressTracker.Update(ProgressUpdate{
					CompartmentName: compartmentName,
					Operation:      "Discovering VCNs",
				})
			}
			logger.Verbose("  Discovering VCNs in compartment: %s", compartmentName)
			if vcns, err := discoverVCNs(ctx, clients, compartmentID); err == nil {
				compartmentResources = append(compartmentResources, vcns...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "VCNs discovered",
						ResourceCount:  int64(len(vcns)),
					})
				}
				logger.Info("  Found %d VCNs", len(vcns))
				logger.Debug("  VCNs discovery completed successfully")
			} else if !isRetriableError(err) {
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{IsError: true})
				}
				logger.Error("Failed to discover VCNs in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping VCNs in compartment %s due to retriable error: %v", compartmentName, err)
			}

			// Discover subnets
			if progressTracker != nil {
				progressTracker.Update(ProgressUpdate{
					CompartmentName: compartmentName,
					Operation:      "Discovering subnets",
				})
			}
			logger.Verbose("  Discovering subnets in compartment: %s", compartmentName)
			if subnets, err := discoverSubnets(ctx, clients, compartmentID); err == nil {
				compartmentResources = append(compartmentResources, subnets...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "Subnets discovered",
						ResourceCount:  int64(len(subnets)),
					})
				}
				logger.Info("  Found %d subnets", len(subnets))
				logger.Debug("  Subnets discovery completed successfully")
			} else if !isRetriableError(err) {
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{IsError: true})
				}
				logger.Error("Failed to discover subnets in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping subnets in compartment %s due to retriable error: %v", compartmentName, err)
			}

			// Discover block volumes
			// progressTracker.SetCurrentOperation("Discovering block volumes", compartmentName)
			logger.Verbose("  Discovering block volumes in compartment: %s", compartmentName)
			if volumes, err := discoverBlockVolumes(ctx, clients, compartmentID); err == nil {
				compartmentResources = append(compartmentResources, volumes...)
				// progressTracker.AddDiscoveredResources("block_volume", len(volumes))
				logger.Info("  Found %d block volumes", len(volumes))
				logger.Debug("  Block volumes discovery completed successfully")
			} else if !isRetriableError(err) {
				// progressTracker.IncrementError()
				logger.Error("Failed to discover block volumes in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping block volumes in compartment %s due to retriable error: %v", compartmentName, err)
			}
			// progressTracker.IncrementResourceType()

			// Discover Object Storage buckets
			// progressTracker.SetCurrentOperation("Discovering Object Storage buckets", compartmentName)
			logger.Verbose("  Discovering Object Storage buckets in compartment: %s", compartmentName)
			if buckets, err := discoverObjectStorageBuckets(ctx, clients, compartmentID); err == nil {
				compartmentResources = append(compartmentResources, buckets...)
				// progressTracker.AddDiscoveredResources("object_storage_bucket", len(buckets))
				logger.Info("  Found %d Object Storage buckets", len(buckets))
				logger.Debug("  Object Storage buckets discovery completed successfully")
			} else if !isRetriableError(err) {
				// progressTracker.IncrementError()
				logger.Error("Failed to discover Object Storage buckets in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping Object Storage buckets in compartment %s due to retriable error: %v", compartmentName, err)
			}
			// progressTracker.IncrementResourceType()

			// Discover OKE clusters
			// progressTracker.SetCurrentOperation("Discovering OKE clusters", compartmentName)
			logger.Verbose("  Discovering OKE clusters in compartment: %s", compartmentName)
			if clusters, err := discoverOKEClusters(ctx, clients, compartmentID); err == nil {
				compartmentResources = append(compartmentResources, clusters...)
				// progressTracker.AddDiscoveredResources("oke_cluster", len(clusters))
				logger.Info("  Found %d OKE clusters", len(clusters))
				logger.Debug("  OKE clusters discovery completed successfully")
			} else if !isRetriableError(err) {
				// progressTracker.IncrementError()
				logger.Error("Failed to discover OKE clusters in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping OKE clusters in compartment %s due to retriable error: %v", compartmentName, err)
			}
			// progressTracker.IncrementResourceType()

			// Discover Load Balancers
			// progressTracker.SetCurrentOperation("Discovering Load Balancers", compartmentName)
			logger.Verbose("  Discovering Load Balancers in compartment: %s", compartmentName)
			if lbs, err := discoverLoadBalancers(ctx, clients, compartmentID); err == nil {
				compartmentResources = append(compartmentResources, lbs...)
				// progressTracker.AddDiscoveredResources("load_balancer", len(lbs))
				logger.Info("  Found %d Load Balancers", len(lbs))
				logger.Debug("  Load Balancers discovery completed successfully")
			} else if !isRetriableError(err) {
				// progressTracker.IncrementError()
				logger.Error("Failed to discover Load Balancers in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping Load Balancers in compartment %s due to retriable error: %v", compartmentName, err)
			}
			// progressTracker.IncrementResourceType()

			// Discover Database Systems
			// progressTracker.SetCurrentOperation("Discovering Database Systems", compartmentName)
			logger.Verbose("  Discovering Database Systems in compartment: %s", compartmentName)
			if dbs, err := discoverDatabases(ctx, clients, compartmentID); err == nil {
				compartmentResources = append(compartmentResources, dbs...)
				// progressTracker.AddDiscoveredResources("database_system", len(dbs))
				logger.Info("  Found %d Database Systems", len(dbs))
				logger.Debug("  Database Systems discovery completed successfully")
			} else if !isRetriableError(err) {
				// progressTracker.IncrementError()
				logger.Error("Failed to discover Database Systems in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping Database Systems in compartment %s due to retriable error: %v", compartmentName, err)
			}
			// progressTracker.IncrementResourceType()

			// Discover DRGs
			// progressTracker.SetCurrentOperation("Discovering DRGs", compartmentName)
			logger.Verbose("  Discovering DRGs in compartment: %s", compartmentName)
			if drgs, err := discoverDRGs(ctx, clients, compartmentID); err == nil {
				compartmentResources = append(compartmentResources, drgs...)
				// progressTracker.AddDiscoveredResources("drg", len(drgs))
				logger.Info("  Found %d DRGs", len(drgs))
				logger.Debug("  DRGs discovery completed successfully")
			} else if !isRetriableError(err) {
				// progressTracker.IncrementError()
				logger.Error("Failed to discover DRGs in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping DRGs in compartment %s due to retriable error: %v", compartmentName, err)
			}
			// progressTracker.IncrementResourceType()

			// Discover Autonomous Databases
			logger.Verbose("  Discovering Autonomous Databases in compartment: %s", compartmentName)
			var autonomousDBs []ResourceInfo
			err = withRetryAndProgress(ctx, func() error {
				var retryErr error
				autonomousDBs, retryErr = discoverAutonomousDatabasesWithStats(ctx, clients, compartmentID, compartmentName, statsCollector)
				return retryErr
			}, 3, "autonomous databases discovery", progressTracker)
			
			if err == nil {
				compartmentResources = append(compartmentResources, autonomousDBs...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "Autonomous Databases discovered",
						ResourceCount:  int64(len(autonomousDBs)),
					})
				}
				logger.Info("  Found %d Autonomous Databases", len(autonomousDBs))
				logger.Debug("  Autonomous Databases discovery completed successfully")
			} else if !isRetriableError(err) {
				logger.Error("Failed to discover Autonomous Databases in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping Autonomous Databases in compartment %s due to retriable error: %v", compartmentName, err)
			}

			// Discover Functions
			logger.Verbose("  Discovering Functions in compartment: %s", compartmentName)
			var functions []ResourceInfo
			err = withRetryAndProgress(ctx, func() error {
				var retryErr error
				functions, retryErr = discoverFunctionsWithStats(ctx, clients, compartmentID, compartmentName, statsCollector)
				return retryErr
			}, 3, "functions discovery", progressTracker)
			
			if err == nil {
				compartmentResources = append(compartmentResources, functions...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "Functions discovered",
						ResourceCount:  int64(len(functions)),
					})
				}
				logger.Info("  Found %d Functions", len(functions))
				logger.Debug("  Functions discovery completed successfully")
			} else if !isRetriableError(err) {
				logger.Error("Failed to discover Functions in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping Functions in compartment %s due to retriable error: %v", compartmentName, err)
			}

			// Discover API Gateways
			logger.Verbose("  Discovering API Gateways in compartment: %s", compartmentName)
			var apiGateways []ResourceInfo
			err = withRetryAndProgress(ctx, func() error {
				var retryErr error
				apiGateways, retryErr = discoverAPIGatewaysWithStats(ctx, clients, compartmentID, compartmentName, statsCollector)
				return retryErr
			}, 3, "api gateways discovery", progressTracker)
			
			if err == nil {
				compartmentResources = append(compartmentResources, apiGateways...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "API Gateways discovered",
						ResourceCount:  int64(len(apiGateways)),
					})
				}
				logger.Info("  Found %d API Gateways", len(apiGateways))
				logger.Debug("  API Gateways discovery completed successfully")
			} else if !isRetriableError(err) {
				logger.Error("Failed to discover API Gateways in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping API Gateways in compartment %s due to retriable error: %v", compartmentName, err)
			}

			// Discover File Storage Systems
			logger.Verbose("  Discovering File Storage Systems in compartment: %s", compartmentName)
			var fileSystems []ResourceInfo
			err = withRetryAndProgress(ctx, func() error {
				var retryErr error
				fileSystems, retryErr = discoverFileStorageSystemsWithStats(ctx, clients, compartmentID, compartmentName, statsCollector)
				return retryErr
			}, 3, "file storage systems discovery", progressTracker)
			
			if err == nil {
				compartmentResources = append(compartmentResources, fileSystems...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "File Storage Systems discovered",
						ResourceCount:  int64(len(fileSystems)),
					})
				}
				logger.Info("  Found %d File Storage Systems", len(fileSystems))
				logger.Debug("  File Storage Systems discovery completed successfully")
			} else if !isRetriableError(err) {
				logger.Error("Failed to discover File Storage Systems in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping File Storage Systems in compartment %s due to retriable error: %v", compartmentName, err)
			}

			// Discover Network Load Balancers
			logger.Verbose("  Discovering Network Load Balancers in compartment: %s", compartmentName)
			var networkLBs []ResourceInfo
			err = withRetryAndProgress(ctx, func() error {
				var retryErr error
				networkLBs, retryErr = discoverNetworkLoadBalancersWithStats(ctx, clients, compartmentID, compartmentName, statsCollector)
				return retryErr
			}, 3, "network load balancers discovery", progressTracker)
			
			if err == nil {
				compartmentResources = append(compartmentResources, networkLBs...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "Network Load Balancers discovered",
						ResourceCount:  int64(len(networkLBs)),
					})
				}
				logger.Info("  Found %d Network Load Balancers", len(networkLBs))
				logger.Debug("  Network Load Balancers discovery completed successfully")
			} else if !isRetriableError(err) {
				logger.Error("Failed to discover Network Load Balancers in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping Network Load Balancers in compartment %s due to retriable error: %v", compartmentName, err)
			}

			// Discover Streams
			logger.Verbose("  Discovering Streams in compartment: %s", compartmentName)
			var streams []ResourceInfo
			err = withRetryAndProgress(ctx, func() error {
				var retryErr error
				streams, retryErr = discoverStreamsWithStats(ctx, clients, compartmentID, compartmentName, statsCollector)
				return retryErr
			}, 3, "streams discovery", progressTracker)
			
			if err == nil {
				compartmentResources = append(compartmentResources, streams...)
				if progressTracker != nil {
					progressTracker.Update(ProgressUpdate{
						CompartmentName: compartmentName,
						Operation:      "Streams discovered",
						ResourceCount:  int64(len(streams)),
					})
				}
				logger.Info("  Found %d Streams", len(streams))
				logger.Debug("  Streams discovery completed successfully")
			} else if !isRetriableError(err) {
				logger.Error("Failed to discover Streams in compartment %s: %v", compartmentName, err)
			} else {
				logger.Verbose("Skipping Streams in compartment %s due to retriable error: %v", compartmentName, err)
			}
			
			// Mark compartment as completed
			if progressTracker != nil {
				progressTracker.Update(ProgressUpdate{
					CompartmentName: compartmentName,
					Operation:      "Compartment completed",
					IsCompartmentComplete: true,
				})
			}
			
			// Thread-safe append to allResources
			resourcesMutex.Lock()
			allResources = append(allResources, compartmentResources...)
			resourcesMutex.Unlock()
		}(i, compartment)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()

	logger.Info("Discovery completed. Total resources found: %d", len(allResources))
	logger.Verbose("Resource discovery operation completed successfully")
	logger.Debug("Final resource count breakdown: %d total resources across %d compartments", len(allResources), len(compartments))
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