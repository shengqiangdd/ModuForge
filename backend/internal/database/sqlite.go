package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	Conn *sql.DB
}

func NewSQLiteDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=30000&_foreign_keys=ON&_loc=ON&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite only allows ONE writer at a time. MaxOpenConns(1) serializes
	// all access to prevent "database is locked" errors. Reads within the
	// same connection still work fine in WAL mode.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	db := &DB{Conn: conn}
	db.Conn.Exec("PRAGMA mmap_size = 268435456") // 256MB

	// Run database migrations: prefer golang-migrate if migration files exist,
	// otherwise fall back to the inline schemaMigrations() approach.
	migrationsDir := resolveMigrationsDir(dbPath)
	if ok, err := RunMigrations(dbPath, migrationsDir); err != nil {
		return nil, fmt.Errorf("golang-migrate: %w", err)
	} else if ok {
		log.Println("[DB] Golang-migrate applied successfully")
	} else {
		// Fallback: no migration files found, use inline schema
		if err := db.migrate(); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
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

// resolveMigrationsDir determines the path to the migrations directory.
// Priority: MIGRATIONS_DIR env var > migrations/ relative to dbPath's parent > ./migrations
func resolveMigrationsDir(dbPath string) string {
	if envDir := os.Getenv("MIGRATIONS_DIR"); envDir != "" {
		return envDir
	}
	// Try migrations/ relative to the database file's directory
	dbDir := filepath.Dir(dbPath)
	candidate := filepath.Join(dbDir, "migrations")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	// Try migrations/ relative to working directory
	if info, err := os.Stat("migrations"); err == nil && info.IsDir() {
		return "migrations"
	}
	return candidate
}
