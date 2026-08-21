package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// HasMigrationFiles returns true if the migrations directory contains .sql files.
func HasMigrationFiles(migrationsDir string) bool {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			return true
		}
	}
	return false
}

// RunMigrations runs golang-migrate up on the given database path.
// migrationsDir should point to the directory containing .up.sql/.down.sql files.
// Returns true if migrations were applied, false if no migration files exist.
func RunMigrations(dbPath, migrationsDir string) (bool, error) {
	if !HasMigrationFiles(migrationsDir) {
		log.Println("[DB] No migration files found, falling back to inline schema")
		return false, nil
	}

	// For existing databases that were created with the legacy schemaMigrations()
	// approach, we need to stamp the initial migration so golang-migrate doesn't
	// try to re-apply tables that already exist.
	m, err := newMigrateInstance(dbPath, migrationsDir)
	if err != nil {
		return false, fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	// Check if schema_migrations table exists and has any applied versions.
	applied, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return false, fmt.Errorf("migrate version: %w", err)
	}

	if errors.Is(err, migrate.ErrNilVersion) {
		// No migrations have been applied yet.
		// Check if the database already has tables (legacy schema).
		if databaseHasTables(dbPath) {
			log.Println("[DB] Existing database detected without migration history — stamping initial migration")
			if err := stampInitialMigration(dbPath); err != nil {
				return false, fmt.Errorf("stamp initial migration: %w", err)
			}
		}
	} else if dirty {
		return false, fmt.Errorf("migration dirty at version %d — manual fix required", applied)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return false, fmt.Errorf("migrate up: %w", err)
	}

	return true, nil
}

// RunMigrationsDown rolls back migrations.
func RunMigrationsDown(dbPath, migrationsDir string) error {
	if !HasMigrationFiles(migrationsDir) {
		return fmt.Errorf("no migration files found in %s", migrationsDir)
	}

	m, err := newMigrateInstance(dbPath, migrationsDir)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// RunMigrationsStatus prints the current migration version.
func RunMigrationsStatus(dbPath, migrationsDir string) (uint, bool, error) {
	if !HasMigrationFiles(migrationsDir) {
		return 0, false, fmt.Errorf("no migration files found in %s", migrationsDir)
	}

	m, err := newMigrateInstance(dbPath, migrationsDir)
	if err != nil {
		return 0, false, fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("migrate version: %w", err)
	}
	return version, dirty, nil
}

func newMigrateInstance(dbPath, migrationsDir string) (*migrate.Migrate, error) {
	sourceURL := "file://" + migrationsDir
	dbURL := "sqlite3://" + dbPath
	return migrate.New(sourceURL, dbURL)
}

// databaseHasTables checks if the SQLite database already contains user-created tables.
func databaseHasTables(dbPath string) bool {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return false
	}
	defer db.Close()

	var cnt int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&cnt)
	return cnt > 0
}

// stampInitialMigration manually inserts a row into schema_migrations to mark
// the initial migration as applied for existing databases.
func stampInitialMigration(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create the schema_migrations table if it doesn't exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, dirty BOOLEAN NOT NULL DEFAULT 0)`)
	if err != nil {
		return err
	}

	// Mark version 1 as applied
	_, err = db.Exec(`INSERT OR REPLACE INTO schema_migrations (version, dirty) VALUES (1, 0)`)
	return err
}
