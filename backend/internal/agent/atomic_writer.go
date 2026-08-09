package agent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// ═══════════════════════════════════════════════════════════════════
// Atomic Writer — Multi-file transactional writes with rollback
// Inspired by: Database transactions, Nix store atomicity
//
// Provides two levels of atomicity:
// 1. Batch writes: Write multiple files, rollback all on failure
// 2. Single-file: Write with backup, restore on verification failure
//
// Usage:
//   aw := NewAtomicWriter(projectPath)
//   tx := aw.BeginTransaction("refactor-handler")
//   tx.Stage(path1, content1)
//   tx.Stage(path2, content2)
//   err := tx.Commit()  // writes all files atomically
//   if err != nil { tx.Rollback() }  // restore all backups
// ═══════════════════════════════════════════════════════════════════

// AtomicWriter manages transactional file writes for a project.
type AtomicWriter struct {
	projectPath   string
	backupDir     string
	mu            sync.Mutex
	transactions  map[string]*Transaction
}

// NewAtomicWriter creates a new atomic writer for a project.
func NewAtomicWriter(projectPath string) *AtomicWriter {
	backupDir := filepath.Join(projectPath, ".mf_backups")
	return &AtomicWriter{
		projectPath:  projectPath,
		backupDir:    backupDir,
		transactions: make(map[string]*Transaction),
	}
}

// Transaction represents a batch of file operations that can be committed or rolled back.
type Transaction struct {
	ID           string
	ProjectPath  string
	StagedFiles  []StagedFile
	Backups      []FileBackup
	Status       string // "pending", "committed", "rolled_back"
	CommitOrder  []int  // order of file writes for rollback (reverse order)
	mu           sync.Mutex
}

// StagedFile represents a file to be written.
type StagedFile struct {
	Path       string // absolute or project-relative path
	Content    string
	IsNew      bool   // true if file didn't exist before
	Operation  string // "create", "update", "delete"
}

// FileBackup stores the backup of a file before modification.
type FileBackup struct {
	Path    string
	Content string
	Existed bool // true if file existed before transaction
	ModTime int64
}

// BeginTransaction starts a new transaction.
func (aw *AtomicWriter) BeginTransaction(id string) *Transaction {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	tx := &Transaction{
		ID:          id,
		ProjectPath: aw.projectPath,
		StagedFiles: make([]StagedFile, 0),
		Backups:     make([]FileBackup, 0),
		Status:      "pending",
		CommitOrder: make([]int, 0),
	}
	aw.transactions[id] = tx
	return tx
}

// Stage adds a file to the transaction.
func (tx *Transaction) Stage(path, content string) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	tx.StagedFiles = append(tx.StagedFiles, StagedFile{
		Path:    path,
		Content: content,
		IsNew:   false, // will be determined during commit
		Operation: "update",
	})
}

// StageNew adds a new file to the transaction.
func (tx *Transaction) StageNew(path, content string) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	tx.StagedFiles = append(tx.StagedFiles, StagedFile{
		Path:      path,
		Content:   content,
		IsNew:     true,
		Operation: "create",
	})
}

// StageDelete marks a file for deletion.
func (tx *Transaction) StageDelete(path string) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	tx.StagedFiles = append(tx.StagedFiles, StagedFile{
		Path:      path,
		Operation: "delete",
	})
}

// Commit writes all staged files and creates backups for rollback.
// Returns nil on success, error on failure (triggers automatic rollback).
func (tx *Transaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.Status != "pending" {
		return fmt.Errorf("transaction %s is not pending (status: %s)", tx.ID, tx.Status)
	}

	// Phase 1: Create backups of all files that will be modified
	for _, sf := range tx.StagedFiles {
		absPath := tx.resolvePath(sf.Path)

		backup := FileBackup{
			Path: absPath,
		}

		// Check if file exists
		if info, err := os.Stat(absPath); err == nil {
			// File exists — read and backup
			content, readErr := os.ReadFile(absPath)
			if readErr != nil {
				tx.Status = "rolled_back"
				return fmt.Errorf("failed to backup %s: %v", absPath, readErr)
			}
			backup.Content = string(content)
			backup.Existed = true
			backup.ModTime = info.ModTime().UnixNano()
		} else {
			backup.Existed = false
		}

		tx.Backups = append(tx.Backups, backup)
	}

	// Phase 2: Write all files (track order for rollback)
	tx.CommitOrder = make([]int, len(tx.StagedFiles))
	for i := range tx.CommitOrder {
		tx.CommitOrder[i] = i
	}

	for i, sf := range tx.StagedFiles {
		absPath := tx.resolvePath(sf.Path)

		// Ensure parent directory exists
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("[AtomicWriter] mkdir failed: %v, rolling back", err)
			tx.rollbackWritten(i)
			tx.Status = "rolled_back"
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}

		switch sf.Operation {
		case "create", "update":
			if err := os.WriteFile(absPath, []byte(sf.Content), 0644); err != nil {
				log.Printf("[AtomicWriter] write failed for %s: %v, rolling back", absPath, err)
				tx.rollbackWritten(i)
				tx.Status = "rolled_back"
				return fmt.Errorf("failed to write %s: %v", absPath, err)
			}
			log.Printf("[AtomicWriter] committed: %s (%d bytes)", sf.Path, len(sf.Content))

		case "delete":
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				log.Printf("[AtomicWriter] delete failed for %s: %v, rolling back", absPath, err)
				tx.rollbackWritten(i)
				tx.Status = "rolled_back"
				return fmt.Errorf("failed to delete %s: %v", absPath, err)
			}
			log.Printf("[AtomicWriter] committed delete: %s", sf.Path)
		}
	}

	tx.Status = "committed"
	log.Printf("[AtomicWriter] transaction %s committed: %d files", tx.ID, len(tx.StagedFiles))
	return nil
}

// Rollback restores all files to their pre-transaction state.
func (tx *Transaction) Rollback() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.Status == "rolled_back" {
		return nil // already rolled back
	}

	if tx.Status == "committed" {
		// Reverse the writes: iterate in reverse order
		for i := len(tx.CommitOrder) - 1; i >= 0; i-- {
			idx := tx.CommitOrder[i]
			if idx >= len(tx.Backups) {
				continue
			}
			backup := tx.Backups[idx]
			sf := tx.StagedFiles[idx]

			switch sf.Operation {
			case "create":
				// File was new — remove it
				if !backup.Existed {
					os.Remove(backup.Path)
				}
			case "update":
				// File existed — restore original content
				if backup.Existed {
					os.WriteFile(backup.Path, []byte(backup.Content), 0644)
				}
			case "delete":
				// File was deleted — restore from backup
				if backup.Existed {
					os.WriteFile(backup.Path, []byte(backup.Content), 0644)
				}
			}
		}
	}

	tx.Status = "rolled_back"
	log.Printf("[AtomicWriter] transaction %s rolled back: %d files restored", tx.ID, len(tx.Backups))
	return nil
}

// rollbackWritten rolls back files that were already written (partial failure).
func (tx *Transaction) rollbackWritten(upToIdx int) {
	for i := upToIdx - 1; i >= 0; i-- {
		if i >= len(tx.Backups) {
			continue
		}
		backup := tx.Backups[i]
		sf := tx.StagedFiles[i]

		switch {
		case sf.Operation == "create" && !backup.Existed:
			// File was new and we wrote it — remove it
			os.Remove(tx.resolvePath(sf.Path))
		case sf.Operation == "update" && backup.Existed:
			// File existed and we overwrote it — restore
			os.WriteFile(backup.Path, []byte(backup.Content), 0644)
		}
	}
}

// resolvePath resolves a path to absolute, handling project-relative paths.
func (tx *Transaction) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(tx.ProjectPath, path)
}

// ═══════════════════════════════════════════════════════════════════
// Convenience Methods — Common atomic write patterns
// ═══════════════════════════════════════════════════════════════════

// AtomicWriteFiles writes multiple files atomically with rollback support.
// This is the high-level API for batch file operations.
// Returns nil on success, error on failure (files are rolled back).
func AtomicWriteFiles(projectPath string, files map[string]string, txID string) error {
	if len(files) == 0 {
		return nil
	}

	aw := NewAtomicWriter(projectPath)
	tx := aw.BeginTransaction(txID)

	// Sort paths for deterministic ordering
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	// Stage all files
	for _, p := range paths {
		tx.Stage(p, files[p])
	}

	// Commit with automatic rollback on failure
	return tx.Commit()
}

// AtomicWriteFilesWithBackup writes files and returns a rollback function.
// The caller can call the rollback function if needed later.
func AtomicWriteFilesWithBackup(projectPath string, files map[string]string, txID string) (rollback func() error, err error) {
	if len(files) == 0 {
		return func() error { return nil }, nil
	}

	aw := NewAtomicWriter(projectPath)
	tx := aw.BeginTransaction(txID)

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		tx.Stage(p, files[p])
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return tx.Rollback, nil
}

// ═══════════════════════════════════════════════════════════════════
// Verification — Post-commit integrity check
// ═══════════════════════════════════════════════════════════════════

// VerifyCommit checks that all committed files have the expected content.
// Returns nil if all files match, or an error describing mismatches.
func (tx *Transaction) VerifyCommit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.Status != "committed" {
		return fmt.Errorf("transaction not committed (status: %s)", tx.Status)
	}

	for _, sf := range tx.StagedFiles {
		if sf.Operation == "delete" {
			continue // nothing to verify for deletes
		}

		absPath := tx.resolvePath(sf.Path)
		content, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("verification failed: cannot read %s: %v", sf.Path, err)
		}

		if string(content) != sf.Content {
			return fmt.Errorf("verification failed: %s content mismatch (expected %d bytes, got %d bytes)",
				sf.Path, len(sf.Content), len(content))
		}
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════
// Cleanup — Remove old backups
// ═══════════════════════════════════════════════════════════════════

// CleanupBackups removes the backup directory for a project.
func (aw *AtomicWriter) CleanupBackups() error {
	return os.RemoveAll(aw.backupDir)
}
