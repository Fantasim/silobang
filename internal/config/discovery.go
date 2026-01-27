package config

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"meshbank/internal/constants"
	"meshbank/internal/database"
)

type TopicInfo struct {
	Name    string
	Path    string
	Healthy bool
	Error   string
}

func DiscoverTopics(workingDir string) ([]TopicInfo, error) {
	entries, err := os.ReadDir(workingDir)
	if err != nil {
		return nil, err
	}

	var topics []TopicInfo
	topicNamePattern := regexp.MustCompile(constants.TopicNameRegex)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip .internal directory
		if name == constants.InternalDir {
			continue
		}

		// Check if name matches topic naming rules
		if !topicNamePattern.MatchString(name) {
			continue
		}

		topicPath := filepath.Join(workingDir, name)
		internalPath := filepath.Join(topicPath, constants.InternalDir)
		dbPath := filepath.Join(internalPath, name+".db")

		// Check if .internal directory exists
		internalInfo, internalErr := os.Stat(internalPath)
		if os.IsNotExist(internalErr) {
			// No .internal directory, skip (not a topic folder)
			continue
		}

		if internalErr != nil {
			topics = append(topics, TopicInfo{
				Name:    name,
				Path:    topicPath,
				Healthy: false,
				Error:   fmt.Sprintf("cannot access .internal: %v", internalErr),
			})
			continue
		}

		if !internalInfo.IsDir() {
			topics = append(topics, TopicInfo{
				Name:    name,
				Path:    topicPath,
				Healthy: false,
				Error:   ".internal is not a directory",
			})
			continue
		}

		// Check if database file exists
		_, dbErr := os.Stat(dbPath)
		if os.IsNotExist(dbErr) {
			topics = append(topics, TopicInfo{
				Name:    name,
				Path:    topicPath,
				Healthy: false,
				Error:   fmt.Sprintf("missing database file: %s.db", name),
			})
			continue
		}

		if dbErr != nil {
			topics = append(topics, TopicInfo{
				Name:    name,
				Path:    topicPath,
				Healthy: false,
				Error:   fmt.Sprintf("cannot access database: %v", dbErr),
			})
			continue
		}

		// Verify dat hashes
		topicDB, err := database.OpenDatabase(dbPath)
		if err != nil {
			topics = append(topics, TopicInfo{
				Name:    name,
				Path:    topicPath,
				Healthy: false,
				Error:   fmt.Sprintf("failed to open database: %v", err),
			})
			continue
		}

		mismatched, err := database.VerifyAllDatHashes(topicDB, topicPath)
		topicDB.Close()

		if err != nil {
			topics = append(topics, TopicInfo{
				Name:    name,
				Path:    topicPath,
				Healthy: false,
				Error:   fmt.Sprintf("failed to verify dat hashes: %v", err),
			})
			continue
		}

		if len(mismatched) > 0 {
			topics = append(topics, TopicInfo{
				Name:    name,
				Path:    topicPath,
				Healthy: false,
				Error:   fmt.Sprintf("dat hash mismatch: %v", mismatched),
			})
			continue
		}

		// Topic is healthy
		topics = append(topics, TopicInfo{
			Name:    name,
			Path:    topicPath,
			Healthy: true,
			Error:   "",
		})
	}

	return topics, nil
}

// IndexTopicToOrchestrator indexes all assets from a topic into the orchestrator database
func IndexTopicToOrchestrator(topicPath string, topicName string, orchestratorDB *sql.DB) error {
	// Open topic database
	topicDBPath := filepath.Join(topicPath, constants.InternalDir, topicName+".db")
	topicDB, err := database.OpenDatabase(topicDBPath)
	if err != nil {
		return err
	}
	defer topicDB.Close()

	// Query all assets
	rows, err := topicDB.Query("SELECT asset_id, blob_name FROM assets")
	if err != nil {
		return err
	}
	defer rows.Close()

	// Index each asset (INSERT OR IGNORE for duplicates)
	for rows.Next() {
		var hash, datFile string
		if err := rows.Scan(&hash, &datFile); err != nil {
			return err
		}

		// INSERT OR IGNORE - first topic wins
		if err := database.InsertAssetIndexIgnore(orchestratorDB, hash, topicName, datFile); err != nil {
			return err
		}
	}

	return rows.Err()
}
