package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	Conn *sql.DB
}

func NewSQLiteDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=10000&_foreign_keys=ON&_loc=auto")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite with WAL mode supports concurrent readers.
	// MaxOpenConns > 1 allows concurrent reads while writes are serialized by SQLite.
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(4)
	conn.SetConnMaxLifetime(0)

	db := &DB{Conn: conn}
	db.Conn.Exec("PRAGMA mmap_size = 268435456") // 256MB
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	// Execute all schema migrations
	for _, m := range schemaMigrations() {
		if _, err := db.Conn.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %s: %w", m[:50], err)
		}
	}

	// Cleanup orphan tables from removed features (plugins, permission audits, webhooks)
	for _, drop := range orphanTables() {
		db.Conn.Exec(drop)
	}
	for _, dropIdx := range orphanIndexes() {
		db.Conn.Exec(dropIdx)
	}

	// Cleanup indexes on empty/scaffold tables (no data, no queries)
	for _, idx := range cleanupIndexes() {
		db.Conn.Exec("DROP INDEX IF EXISTS " + idx)
	}

	// Post-migration: add columns that may not exist in older schemas
	for _, m := range addColumnIfMissing() {
		db.Conn.Exec(m) // ignore errors for ALTER TABLE
	}

	// Run specific migrations
	db.migrateAIUsageDaily()
	db.migrateDashboardWidgets()

	// Migrate user_id columns from INTEGER to TEXT in 8 affected tables.
	// Users.id is UUID (TEXT), so user_id must also be TEXT for foreign keys to work.
	db.migrateUserIDTypes()

	// Start WAL checkpoint scheduler
	db.startWALCheckpoint()

	// Migrate adb_saved_devices: rebuild with UNIQUE(user_id, address) instead of UNIQUE(address)
	db.migrateADBSavedDevices()

	// S3 storage migration: add metadata columns to project_files
	db.migrateProjectFilesS3()

	log.Println("[DB] SQLite migrations complete")
	return nil
}

func (db *DB) Close() error {
	return db.Conn.Close()
}
