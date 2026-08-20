package database

import (
	"crypto/sha256"
	"database/sql"
	"os"
	"testing"

	"github.com/moduforge/backend/internal/config"
)

// expectedTables lists key tables that should exist after migration
var expectedTables = []string{
	"users",
	"projects",
	"market_modules",
	"conversation_messages",
	"ai_conversations",
}

func TestTableExists(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// sqlite_master 是系统影子表，不在自身中列出；用真实创建的表验证检测逻辑
	if _, err := db.Exec(`CREATE TABLE test_exists_check (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}
	if !tableExists(db, "test_exists_check") {
		t.Error("expected created table to exist")
	}
	if tableExists(db, "nonexistent_table_xyz") {
		t.Error("expected nonexistent table to not exist")
	}
}

func TestColumnExists(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// sqlite_master has columns: type, name, tbl_name, rootpage, sql
	if !columnExists(db, "sqlite_master", "name") {
		t.Error("expected column 'name' to exist in sqlite_master")
	}
	if columnExists(db, "sqlite_master", "nonexistent_col") {
		t.Error("expected nonexistent column to not exist")
	}
}

func TestColumnExists_InvalidTableName(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// columnExists should return false for SQL injection attempts
	if columnExists(db, "sqlite_master; DROP TABLE users", "name") {
		t.Error("expected columnExists to reject malicious table name")
	}
}

func TestColumnExists_InvalidTableNameSpecialChars(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if columnExists(db, "users--", "id") {
		t.Error("expected columnExists to reject table name with SQL comment chars")
	}
}

func TestMigrate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Run the migration
	if err := migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// Verify key tables exist
	for _, table := range expectedTables {
		if !tableExists(db, table) {
			t.Errorf("expected table %s to exist after migration", table)
		}
	}

	// Migration should be idempotent
	if err := migrate(db); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
}

func TestMigrate_CreatesIndexes(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// Check a few indexes
	expectedIndexes := []string{"idx_projects_user", "idx_build_tasks_project", "idx_ai_conversations_user"}
	for _, idx := range expectedIndexes {
		rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx)
		if err != nil {
			t.Fatalf("query for index %s failed: %v", idx, err)
		}
		if !rows.Next() {
			t.Errorf("expected index %s to exist", idx)
		}
		rows.Close()
	}
}

func TestEncodeDecodeKey_Base64(t *testing.T) {
	// Without MODUFORGE_ENCRYPT_KEY, should use base64 encoding
	os.Unsetenv("MODUFORGE_ENCRYPT_KEY")
	encryptionKey = nil

	original := "sk-test-api-key-12345"
	encoded := encodeKey(original)
	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded key")
	}

	decoded := decodeKey(encoded)
	if decoded != original {
		t.Errorf("expected %q, got %q", original, decoded)
	}
}

func TestDecodeKey_Empty(t *testing.T) {
	if decodeKey("") != "" {
		t.Error("expected empty string for empty input")
	}
}

func TestDecodeKey_InvalidBase64(t *testing.T) {
	if decodeKey("not-valid-base64!!!") != "" {
		t.Error("expected empty string for invalid base64")
	}
}

func TestEncodeDecodeKey_AES(t *testing.T) {
	key := sha256.Sum256([]byte("test-encryption-key-32bytes!"))
	encryptionKey = key[:]

	original := "sk-sensitive-key"
	encoded := encodeKey(original)
	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded key")
	}

	// Should have "enc:" prefix
	if len(encoded) < 4 || encoded[:4] != "enc:" {
		t.Errorf("expected 'enc:' prefix, got %q", encoded[:4])
	}

	decoded := decodeKey(encoded)
	if decoded != original {
		t.Errorf("expected %q, got %q", original, decoded)
	}
}

func TestEncodeKey_Empty(t *testing.T) {
	encryptionKey = nil

	if encoded := encodeKey(""); encoded != "" {
		t.Errorf("expected empty string, got %q", encoded)
	}
}

func TestInit(t *testing.T) {
	// Quick check: can we open and execute a simple SQLite statement?
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("sqlite3 not available: %v", err)
	}
	if _, err := db.Exec("SELECT 1"); err != nil {
		t.Skipf("sqlite3 exec failed: %v", err)
	}
	db.Close()

	cfg := &config.Config{
		DatabasePath: ":memory:",
		JWTSecret:    "test-secret",
	}
	database, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer database.Close()

	// Verify tables created
	var count int
	database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if count == 0 {
		t.Error("expected at least one table after Init")
	}
}

// openTestDB opens an in-memory SQLite database, skipping if CGO is unavailable.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("sqlite3 not available: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("sqlite3 ping failed: %v", err)
	}
	return db
}