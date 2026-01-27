package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"meshbank/internal/constants"
)

const (
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

// Logger provides logging with optional file output and daily rotation.
type Logger struct {
	level         string
	prefix        string
	mu            sync.Mutex
	workDir       string              // Empty = stdout only
	fileHandles   map[string]*os.File // Open handles by level
	currentDay    int                 // Day tracker for rotation (year*1000 + yday)
	writeToStdout bool                // Also write to stdout (default: true)
}

// LoggerOptions configures the logger behavior.
type LoggerOptions struct {
	Level         string
	WorkDir       string // If set, enables file logging
	WriteToStdout bool   // If true (default), also writes to stdout
}

var levelOrder = map[string]int{
	LevelDebug: 0,
	LevelInfo:  1,
	LevelWarn:  2,
	LevelError: 3,
}

// NewLogger creates a logger with stdout output only.
// This maintains backward compatibility with existing code.
func NewLogger(level string) *Logger {
	return NewLoggerWithOptions(LoggerOptions{
		Level:         level,
		WriteToStdout: true,
	})
}

// NewLoggerWithOptions creates a logger with full configuration.
func NewLoggerWithOptions(opts LoggerOptions) *Logger {
	level := opts.Level
	if _, ok := levelOrder[level]; !ok {
		level = LevelDebug
	}

	l := &Logger{
		level:         level,
		writeToStdout: opts.WriteToStdout,
		fileHandles:   make(map[string]*os.File),
		workDir:       opts.WorkDir,
	}

	// Initialize day tracker if file logging is enabled
	if opts.WorkDir != "" {
		l.currentDay = getDayKey(time.Now())
	}

	return l
}

// SetWorkDir enables or changes file logging to the specified working directory.
// Pass empty string to disable file logging.
func (l *Logger) SetWorkDir(workDir string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Close existing handles if any
	l.closeFileHandlesUnsafe()

	l.workDir = workDir
	l.currentDay = 0 // Force rotation check on next write

	if workDir != "" {
		l.currentDay = getDayKey(time.Now())
	}

	return nil
}

// Close closes all file handles gracefully.
// Should be called when shutting down the application.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closeFileHandlesUnsafe()
}

// closeFileHandlesUnsafe closes all file handles without locking.
// Caller must hold the mutex.
func (l *Logger) closeFileHandlesUnsafe() error {
	var lastErr error
	for level, handle := range l.fileHandles {
		if err := handle.Close(); err != nil {
			lastErr = err
		}
		delete(l.fileHandles, level)
	}
	return lastErr
}

func (l *Logger) shouldLog(level string) bool {
	return levelOrder[level] >= levelOrder[l.level]
}

// getDayKey returns a unique key for the current day (year*1000 + day of year).
func getDayKey(t time.Time) int {
	return t.Year()*1000 + t.YearDay()
}

// getLogFilename generates a log filename from the given time.
// Uses Unix timestamp of the start of the day (midnight UTC).
func getLogFilename(t time.Time) string {
	year, month, day := t.UTC().Date()
	startOfDay := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return fmt.Sprintf("%d%s", startOfDay.Unix(), constants.LogFileExtension)
}

// levelToDir maps a log level to its directory name.
func levelToDir(level string) string {
	switch level {
	case LevelDebug:
		return constants.LogsDirDebug
	case LevelInfo:
		return constants.LogsDirInfo
	case LevelWarn:
		return constants.LogsDirWarn
	case LevelError:
		return constants.LogsDirError
	default:
		return constants.LogsDirDebug
	}
}

// checkRotation checks if the day has changed and rotates log files if needed.
// Must be called with mutex NOT held.
func (l *Logger) checkRotation() {
	if l.workDir == "" {
		return
	}

	now := time.Now()
	dayKey := getDayKey(now)

	if dayKey != l.currentDay {
		l.mu.Lock()
		// Double-check after acquiring lock
		if dayKey != l.currentDay {
			l.closeFileHandlesUnsafe()
			l.currentDay = dayKey
		}
		l.mu.Unlock()
	}
}

// getFileHandleUnsafe returns the file handle for the given level.
// Creates the file if it doesn't exist. Caller must hold the mutex.
func (l *Logger) getFileHandleUnsafe(level string) (*os.File, error) {
	if handle, exists := l.fileHandles[level]; exists {
		return handle, nil
	}

	// Build path: workDir/.internal/logs/level/timestamp.log
	logDir := filepath.Join(l.workDir, constants.InternalDir, constants.LogsDir, levelToDir(level))

	// Ensure directory exists
	if err := os.MkdirAll(logDir, constants.DirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	filename := getLogFilename(time.Now())
	filePath := filepath.Join(logDir, filename)

	// Open for append, create if not exists
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.FilePermissions)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
	}

	l.fileHandles[level] = file
	return file, nil
}

func (l *Logger) log(level, format string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}

	// Check for day rotation if file logging enabled
	if l.workDir != "" {
		l.checkRotation()
	}

	timestamp := time.Now().Format(constants.LogTimestampFormat)
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s | %s\n", level, timestamp, message)

	l.mu.Lock()
	defer l.mu.Unlock()

	// Write to stdout if enabled
	if l.writeToStdout {
		fmt.Print(logLine)
	}

	// Write to file if enabled
	if l.workDir != "" {
		l.writeToFileUnsafe(level, logLine)
	}
}

// writeToFileUnsafe writes the log line to the appropriate file.
// Caller must hold the mutex.
func (l *Logger) writeToFileUnsafe(level, logLine string) {
	handle, err := l.getFileHandleUnsafe(level)
	if err != nil {
		// Fallback: write error to stdout if file write fails
		if l.writeToStdout {
			fmt.Printf("[LOGGER_ERROR] Failed to open log file: %v\n", err)
		}
		return
	}

	if _, err := handle.WriteString(logLine); err != nil {
		if l.writeToStdout {
			fmt.Printf("[LOGGER_ERROR] Failed to write to log file: %v\n", err)
		}
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

func (l *Logger) SetLevel(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := levelOrder[level]; ok {
		l.level = level
	}
}

// GetWorkDir returns the current working directory for file logging.
// Returns empty string if file logging is disabled.
func (l *Logger) GetWorkDir() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.workDir
}
