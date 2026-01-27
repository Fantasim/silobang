package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"meshbank/internal/constants"
)

var datFileRegex = regexp.MustCompile(`^(\d{3,})\.dat$`)

// ListDatFiles returns all .dat files in a topic directory, sorted numerically
func ListDatFiles(topicPath string) ([]string, error) {
	entries, err := os.ReadDir(topicPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read topic directory: %w", err)
	}

	var datFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if datFileRegex.MatchString(entry.Name()) {
			datFiles = append(datFiles, entry.Name())
		}
	}

	// Sort numerically (001.dat, 002.dat, ..., 010.dat, ...)
	sort.Slice(datFiles, func(i, j int) bool {
		numI := extractDatNumber(datFiles[i])
		numJ := extractDatNumber(datFiles[j])
		return numI < numJ
	})

	return datFiles, nil
}

// extractDatNumber extracts the numeric part from a .dat filename
func extractDatNumber(filename string) int {
	matches := datFileRegex.FindStringSubmatch(filename)
	if len(matches) < 2 {
		return 0
	}
	num, _ := strconv.Atoi(matches[1])
	return num
}

// FormatDatFilename formats a number into a .dat filename (e.g., 3 -> "003.dat")
func FormatDatFilename(num int) string {
	return fmt.Sprintf(constants.DatFilePattern, num)
}

// GetNextDatFilename determines the next .dat filename for a topic
// If no .dat files exist, returns "001.dat"
func GetNextDatFilename(topicPath string) (string, error) {
	datFiles, err := ListDatFiles(topicPath)
	if err != nil {
		return "", err
	}

	if len(datFiles) == 0 {
		return constants.FirstDatFilename, nil
	}

	// Get highest number and increment
	lastFile := datFiles[len(datFiles)-1]
	lastNum := extractDatNumber(lastFile)
	return FormatDatFilename(lastNum + 1), nil
}

// GetCurrentDatFile returns the current (latest) .dat file and its size
// If no .dat files exist, returns empty string and 0 size
func GetCurrentDatFile(topicPath string) (filename string, size int64, err error) {
	datFiles, err := ListDatFiles(topicPath)
	if err != nil {
		return "", 0, err
	}

	if len(datFiles) == 0 {
		return "", 0, nil
	}

	currentFile := datFiles[len(datFiles)-1]
	currentPath := filepath.Join(topicPath, currentFile)

	size, err = GetDatFileSize(currentPath)
	if err != nil {
		return "", 0, err
	}

	return currentFile, size, nil
}

// DetermineTargetDatFile decides which .dat file to write to
// Creates a new .dat if current would exceed maxSize after adding entrySize
// Returns the filename (not full path) and whether it's a new file
func DetermineTargetDatFile(topicPath string, entrySize int64, maxDatSize int64) (filename string, isNew bool, err error) {
	currentFile, currentSize, err := GetCurrentDatFile(topicPath)
	if err != nil {
		return "", false, err
	}

	// No existing .dat file
	if currentFile == "" {
		return constants.FirstDatFilename, true, nil
	}

	// Check if current file can accommodate the entry
	if currentSize+entrySize <= maxDatSize {
		return currentFile, false, nil
	}

	// Need a new file
	nextFile, err := GetNextDatFilename(topicPath)
	if err != nil {
		return "", false, err
	}

	return nextFile, true, nil
}

// GetTotalDatSize calculates the total size of all .dat files in a topic
func GetTotalDatSize(topicPath string) (int64, error) {
	datFiles, err := ListDatFiles(topicPath)
	if err != nil {
		return 0, err
	}

	var total int64
	for _, filename := range datFiles {
		path := filepath.Join(topicPath, filename)
		size, err := GetDatFileSize(path)
		if err != nil {
			return 0, err
		}
		total += size
	}

	return total, nil
}

// CountDatFiles returns the number of .dat files in a topic
func CountDatFiles(topicPath string) (int, error) {
	datFiles, err := ListDatFiles(topicPath)
	if err != nil {
		return 0, err
	}
	return len(datFiles), nil
}
