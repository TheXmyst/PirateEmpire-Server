package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	currentLevel LogLevel = INFO
	levelNames            = map[LogLevel]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
	}
)

// Init initializes the logger with the level from environment variable
func Init() {
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	switch levelStr {
	case "DEBUG":
		currentLevel = DEBUG
	case "INFO":
		currentLevel = INFO
	case "WARN":
		currentLevel = WARN
	case "ERROR":
		currentLevel = ERROR
	default:
		currentLevel = INFO // Default to INFO
	}

	log.SetFlags(0) // Remove default timestamp, we'll add our own
	Info("Logger initialized", "level", levelNames[currentLevel])
}

// logf is the internal logging function
func logf(level LogLevel, msg string, args ...interface{}) {
	if level < currentLevel {
		return
	}

	timestamp := time.Now().Format("15:04:05")
	levelName := levelNames[level]

	// Build message with key-value pairs
	fullMsg := fmt.Sprintf("[%s] [%s] %s", timestamp, levelName, msg)

	// Add key-value pairs if provided
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			fullMsg += fmt.Sprintf(" %v=%v", args[i], args[i+1])
		}
	}

	log.Println(fullMsg)
}

// Debug logs a debug message (only visible with LOG_LEVEL=debug)
func Debug(msg string, args ...interface{}) {
	logf(DEBUG, msg, args...)
}

// Info logs an info message (visible by default)
func Info(msg string, args ...interface{}) {
	logf(INFO, msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...interface{}) {
	logf(WARN, msg, args...)
}

// Error logs an error message
func Error(msg string, args ...interface{}) {
	logf(ERROR, msg, args...)
}

// Fatal logs an error and exits
func Fatal(msg string, args ...interface{}) {
	logf(ERROR, msg, args...)
	os.Exit(1)
}
