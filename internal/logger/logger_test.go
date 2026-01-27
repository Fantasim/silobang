package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"silobang/internal/constants"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		expectedLevel string
	}{
		{"valid debug level", LevelDebug, LevelDebug},
		{"valid info level", LevelInfo, LevelInfo},
		{"valid warn level", LevelWarn, LevelWarn},
		{"valid error level", LevelError, LevelError},
		{"invalid level defaults to debug", "invalid", LevelDebug},
		{"empty level defaults to debug", "", LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := NewLogger(tt.level)
			if log == nil {
				t.Fatal("Expected non-nil logger")
			}
			// Logger should have stdout enabled by default
			if !log.writeToStdout {
				t.Error("Expected writeToStdout to be true by default")
			}
			// Logger should have empty workDir (no file logging)
			if log.workDir != "" {
				t.Error("Expected empty workDir for stdout-only logger")
			}
		})
	}
}

func TestNewLoggerWithOptions(t *testing.T) {
	t.Run("file logging disabled when no workdir", func(t *testing.T) {
		log := NewLoggerWithOptions(LoggerOptions{
			Level:         "info",
			WriteToStdout: true,
		})

		if log.workDir != "" {
			t.Error("Expected empty workDir")
		}
		if !log.writeToStdout {
			t.Error("Expected writeToStdout to be true")
		}
	})

	t.Run("file logging enabled with workdir", func(t *testing.T) {
		tmpDir := t.TempDir()

		log := NewLoggerWithOptions(LoggerOptions{
			Level:         "debug",
			WorkDir:       tmpDir,
			WriteToStdout: false,
		})
		defer log.Close()

		if log.workDir != tmpDir {
			t.Errorf("Expected workDir %s, got %s", tmpDir, log.workDir)
		}
		if log.writeToStdout {
			t.Error("Expected writeToStdout to be false")
		}
		if log.currentDay == 0 {
			t.Error("Expected currentDay to be initialized")
		}
	})
}

func TestGetLogFilename(t *testing.T) {
	t.Run("same day produces same filename", func(t *testing.T) {
		t1 := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		t2 := time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC)

		f1 := getLogFilename(t1)
		f2 := getLogFilename(t2)

		if f1 != f2 {
			t.Errorf("Same day should produce same filename: %s != %s", f1, f2)
		}
	})

	t.Run("different days produce different filenames", func(t *testing.T) {
		t1 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
		t2 := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)

		f1 := getLogFilename(t1)
		f2 := getLogFilename(t2)

		if f1 == f2 {
			t.Error("Different days should produce different filenames")
		}
	})

	t.Run("filename has correct extension", func(t *testing.T) {
		filename := getLogFilename(time.Now())
		if !strings.HasSuffix(filename, constants.LogFileExtension) {
			t.Errorf("Filename %s should end with %s", filename, constants.LogFileExtension)
		}
	})

	t.Run("filename is unix timestamp", func(t *testing.T) {
		t1 := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		filename := getLogFilename(t1)

		// Expected: start of day 2024-01-15 00:00:00 UTC = 1705276800
		expected := "1705276800.log"
		if filename != expected {
			t.Errorf("Expected filename %s, got %s", expected, filename)
		}
	})
}

func TestLevelToDir(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{LevelDebug, constants.LogsDirDebug},
		{LevelInfo, constants.LogsDirInfo},
		{LevelWarn, constants.LogsDirWarn},
		{LevelError, constants.LogsDirError},
		{"unknown", constants.LogsDirDebug}, // Unknown defaults to debug
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			got := levelToDir(tt.level)
			if got != tt.expected {
				t.Errorf("levelToDir(%s) = %s, want %s", tt.level, got, tt.expected)
			}
		})
	}
}

func TestGetDayKey(t *testing.T) {
	t.Run("same day same key", func(t *testing.T) {
		t1 := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		t2 := time.Date(2024, 6, 15, 23, 59, 59, 0, time.UTC)

		if getDayKey(t1) != getDayKey(t2) {
			t.Error("Same day should produce same key")
		}
	})

	t.Run("different days different keys", func(t *testing.T) {
		t1 := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		t2 := time.Date(2024, 6, 16, 0, 0, 0, 0, time.UTC)

		if getDayKey(t1) == getDayKey(t2) {
			t.Error("Different days should produce different keys")
		}
	})

	t.Run("different years different keys", func(t *testing.T) {
		t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		t2 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

		if getDayKey(t1) == getDayKey(t2) {
			t.Error("Different years should produce different keys")
		}
	})
}

func TestLoggerWithWorkDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create required directories
	logsDir := filepath.Join(tmpDir, constants.InternalDir, constants.LogsDir)
	for _, level := range []string{constants.LogsDirDebug, constants.LogsDirInfo, constants.LogsDirWarn, constants.LogsDirError} {
		if err := os.MkdirAll(filepath.Join(logsDir, level), constants.DirPermissions); err != nil {
			t.Fatalf("Failed to create log directory: %v", err)
		}
	}

	log := NewLoggerWithOptions(LoggerOptions{
		Level:         "debug",
		WorkDir:       tmpDir,
		WriteToStdout: false,
	})
	defer log.Close()

	// Write a log message
	log.Info("Test message")

	// Verify file was created
	infoDir := filepath.Join(logsDir, constants.LogsDirInfo)
	files, err := os.ReadDir(infoDir)
	if err != nil {
		t.Fatalf("Failed to read info log directory: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 log file, got %d", len(files))
	}

	// Verify file content
	if len(files) > 0 {
		content, err := os.ReadFile(filepath.Join(infoDir, files[0].Name()))
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}

		if !strings.Contains(string(content), "[INFO]") {
			t.Error("Log content should contain [INFO]")
		}
		if !strings.Contains(string(content), "Test message") {
			t.Error("Log content should contain the message")
		}
		if !strings.Contains(string(content), "|") {
			t.Error("Log content should contain | separator")
		}
	}
}

func TestLoggerLevelSeparation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create required directories
	logsDir := filepath.Join(tmpDir, constants.InternalDir, constants.LogsDir)
	for _, level := range []string{constants.LogsDirDebug, constants.LogsDirInfo, constants.LogsDirWarn, constants.LogsDirError} {
		if err := os.MkdirAll(filepath.Join(logsDir, level), constants.DirPermissions); err != nil {
			t.Fatalf("Failed to create log directory: %v", err)
		}
	}

	log := NewLoggerWithOptions(LoggerOptions{
		Level:         "debug",
		WorkDir:       tmpDir,
		WriteToStdout: false,
	})
	defer log.Close()

	// Write messages at different levels
	log.Debug("Debug message")
	log.Info("Info message")
	log.Warn("Warn message")
	log.Error("Error message")

	// Verify each level has its own file
	levelDirs := map[string]string{
		constants.LogsDirDebug: "Debug message",
		constants.LogsDirInfo:  "Info message",
		constants.LogsDirWarn:  "Warn message",
		constants.LogsDirError: "Error message",
	}

	for dir, expectedMsg := range levelDirs {
		levelDir := filepath.Join(logsDir, dir)
		files, err := os.ReadDir(levelDir)
		if err != nil {
			t.Errorf("Failed to read %s directory: %v", dir, err)
			continue
		}

		if len(files) != 1 {
			t.Errorf("Expected 1 file in %s, got %d", dir, len(files))
			continue
		}

		content, err := os.ReadFile(filepath.Join(levelDir, files[0].Name()))
		if err != nil {
			t.Errorf("Failed to read file in %s: %v", dir, err)
			continue
		}

		if !strings.Contains(string(content), expectedMsg) {
			t.Errorf("Expected %s to contain '%s'", dir, expectedMsg)
		}
	}
}

func TestLoggerSetWorkDir(t *testing.T) {
	t.Run("enable file logging", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create required directories
		logsDir := filepath.Join(tmpDir, constants.InternalDir, constants.LogsDir)
		for _, level := range []string{constants.LogsDirDebug, constants.LogsDirInfo, constants.LogsDirWarn, constants.LogsDirError} {
			if err := os.MkdirAll(filepath.Join(logsDir, level), constants.DirPermissions); err != nil {
				t.Fatalf("Failed to create log directory: %v", err)
			}
		}

		// Start with stdout only
		log := NewLogger("debug")
		log.writeToStdout = false // Disable stdout for test

		// Enable file logging
		if err := log.SetWorkDir(tmpDir); err != nil {
			t.Fatalf("SetWorkDir failed: %v", err)
		}
		defer log.Close()

		log.Info("After SetWorkDir")

		// Verify file was created
		infoDir := filepath.Join(logsDir, constants.LogsDirInfo)
		files, _ := os.ReadDir(infoDir)
		if len(files) != 1 {
			t.Errorf("Expected 1 log file after SetWorkDir, got %d", len(files))
		}
	})

	t.Run("disable file logging", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create required directories
		logsDir := filepath.Join(tmpDir, constants.InternalDir, constants.LogsDir)
		for _, level := range []string{constants.LogsDirDebug, constants.LogsDirInfo, constants.LogsDirWarn, constants.LogsDirError} {
			os.MkdirAll(filepath.Join(logsDir, level), constants.DirPermissions)
		}

		log := NewLoggerWithOptions(LoggerOptions{
			Level:         "debug",
			WorkDir:       tmpDir,
			WriteToStdout: false,
		})

		// Disable file logging
		if err := log.SetWorkDir(""); err != nil {
			t.Fatalf("SetWorkDir failed: %v", err)
		}

		if log.workDir != "" {
			t.Error("Expected empty workDir after disabling")
		}
	})
}

func TestLoggerClose(t *testing.T) {
	tmpDir := t.TempDir()

	// Create required directories
	logsDir := filepath.Join(tmpDir, constants.InternalDir, constants.LogsDir)
	for _, level := range []string{constants.LogsDirDebug, constants.LogsDirInfo, constants.LogsDirWarn, constants.LogsDirError} {
		os.MkdirAll(filepath.Join(logsDir, level), constants.DirPermissions)
	}

	log := NewLoggerWithOptions(LoggerOptions{
		Level:         "debug",
		WorkDir:       tmpDir,
		WriteToStdout: false,
	})

	log.Info("Before close")

	err := log.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Verify handles are closed
	if len(log.fileHandles) != 0 {
		t.Error("File handles not cleaned up after Close()")
	}
}

func TestLoggerShouldLog(t *testing.T) {
	tests := []struct {
		loggerLevel  string
		messageLevel string
		shouldLog    bool
	}{
		{LevelDebug, LevelDebug, true},
		{LevelDebug, LevelInfo, true},
		{LevelDebug, LevelWarn, true},
		{LevelDebug, LevelError, true},
		{LevelInfo, LevelDebug, false},
		{LevelInfo, LevelInfo, true},
		{LevelInfo, LevelWarn, true},
		{LevelInfo, LevelError, true},
		{LevelWarn, LevelDebug, false},
		{LevelWarn, LevelInfo, false},
		{LevelWarn, LevelWarn, true},
		{LevelWarn, LevelError, true},
		{LevelError, LevelDebug, false},
		{LevelError, LevelInfo, false},
		{LevelError, LevelWarn, false},
		{LevelError, LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.loggerLevel+"_"+tt.messageLevel, func(t *testing.T) {
			log := NewLogger(tt.loggerLevel)
			got := log.shouldLog(tt.messageLevel)
			if got != tt.shouldLog {
				t.Errorf("Logger(%s).shouldLog(%s) = %v, want %v",
					tt.loggerLevel, tt.messageLevel, got, tt.shouldLog)
			}
		})
	}
}

func TestLoggerGetWorkDir(t *testing.T) {
	t.Run("empty when not set", func(t *testing.T) {
		log := NewLogger("debug")
		if log.GetWorkDir() != "" {
			t.Error("Expected empty workDir")
		}
	})

	t.Run("returns workdir when set", func(t *testing.T) {
		tmpDir := t.TempDir()
		log := NewLoggerWithOptions(LoggerOptions{
			Level:         "debug",
			WorkDir:       tmpDir,
			WriteToStdout: false,
		})
		defer log.Close()

		if log.GetWorkDir() != tmpDir {
			t.Errorf("Expected %s, got %s", tmpDir, log.GetWorkDir())
		}
	})
}

func TestLoggerCreatesDirectoriesOnDemand(t *testing.T) {
	tmpDir := t.TempDir()

	// Only create base .internal dir, not the log subdirs
	internalDir := filepath.Join(tmpDir, constants.InternalDir)
	if err := os.MkdirAll(internalDir, constants.DirPermissions); err != nil {
		t.Fatalf("Failed to create internal dir: %v", err)
	}

	log := NewLoggerWithOptions(LoggerOptions{
		Level:         "debug",
		WorkDir:       tmpDir,
		WriteToStdout: false,
	})
	defer log.Close()

	// This should create the info directory on demand
	log.Info("Test message")

	// Verify directory was created
	infoDir := filepath.Join(tmpDir, constants.InternalDir, constants.LogsDir, constants.LogsDirInfo)
	if _, err := os.Stat(infoDir); os.IsNotExist(err) {
		t.Error("Expected info log directory to be created on demand")
	}
}
