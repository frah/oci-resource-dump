package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// LogLevel represents the logging verbosity level
type LogLevel int

const (
	LogLevelSilent  LogLevel = iota // Only errors
	LogLevelNormal                  // Basic progress info (default)
	LogLevelVerbose                 // Detailed operational info
	LogLevelDebug                   // Full diagnostic info
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
