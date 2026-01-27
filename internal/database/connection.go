package database

import (
	"database/sql"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// OpenDatabase opens a SQLite database at the given path and applies pragmas
// Uses _txlock=immediate to ensure transactions acquire write locks immediately,
// preventing race conditions in read-then-write operations like hash chain updates.
func OpenDatabase(path string) (*sql.DB, error) {
	// _txlock=immediate ensures that BEGIN starts with RESERVED lock,
	// which serializes write transactions and prevents the hash chain race condition.
	// This is critical for maintaining dat_hashes consistency during concurrent uploads.
	db, err := sql.Open("sqlite3", path+"?_txlock=immediate")
	if err != nil {
		return nil, err
	}

	// Apply pragmas
	if err := ApplyPragmas(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// InitTopicDB opens or creates a topic database and initializes the schema
func InitTopicDB(path string) (*sql.DB, error) {
	db, err := OpenDatabase(path)
	if err != nil {
		return nil, err
	}

	// Execute schema creation
	if _, err := db.Exec(GetTopicSchema()); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// InitOrchestratorDB opens or creates the orchestrator database and initializes the schema
func InitOrchestratorDB(path string) (*sql.DB, error) {
	db, err := OpenDatabase(path)
	if err != nil {
		return nil, err
	}

	// Execute schema creation
	if _, err := db.Exec(GetOrchestratorSchema()); err != nil {
		db.Close()
		return nil, err
	}

	// Run migrations for existing databases
	if err := migrateOrchestratorDB(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// migrateOrchestratorDB applies forward-compatible migrations to existing databases.
// Each migration is idempotent (safe to run multiple times).
func migrateOrchestratorDB(db *sql.DB) error {
	// Migration: add username column to audit_log (added for user-based audit tracking)
	_, err := db.Exec(`ALTER TABLE audit_log ADD COLUMN username TEXT NOT NULL DEFAULT ''`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	return nil
}
