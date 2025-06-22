package main

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

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