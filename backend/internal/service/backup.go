package service

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BackupService struct {
	db         *sql.DB
	storageDir string
}

func NewBackupService(db *sql.DB, storageDir string) *BackupService {
	os.MkdirAll(storageDir, 0755)
	return &BackupService{db: db, storageDir: storageDir}
}

func (s *BackupService) GetDB() *sql.DB { return s.db }

func (s *BackupService) ExportDatabase(ctx context.Context) (string, error) {
	tables := []string{
		"users", "projects", "build_tasks", "plugins", "plugin_hooks",
		"market_modules", "market_versions", "market_reviews", "market_stars",
		"collaborators", "collab_comments", "collab_edit_sessions",
		"team_members", "audit_logs", "ai_history", "benchmark_history",
		"provider_configs", "custom_providers", "llm_prompts",
	}

	timestamp := time.Now().UnixMilli()
	output := filepath.Join(s.storageDir, fmt.Sprintf("moduforge_backup_%d.sql", timestamp))

	f, err := os.Create(output)
	if err != nil {
		return "", fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "-- ModuForge Database Backup\n")
	fmt.Fprintf(f, "-- Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(f, "PRAGMA foreign_keys = OFF;\n\n")

	for _, table := range tables {
		// Validate table name to prevent SQL injection
		valid := true
		for _, c := range table {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			continue
		}

		cols, _ := rows.Columns()
		if len(cols) == 0 {
			rows.Close()
			continue
		}

		placeholders := make([]string, len(cols))
		colList := strings.Join(cols, ", ")
		for i := range placeholders {
			placeholders[i] = "?"
		}

		values := make([]interface{}, len(cols))
		scanTargets := make([]interface{}, len(cols))
		for i := range values {
			scanTargets[i] = &values[i]
		}

		for rows.Next() {
			if err := rows.Scan(scanTargets...); err != nil {
				continue
			}

			var valStrs []string
			for _, v := range values {
				if v == nil {
					valStrs = append(valStrs, "NULL")
				} else {
					switch val := v.(type) {
					case []byte:
						valStrs = append(valStrs, fmt.Sprintf("'%s'", escapeSQL(string(val))))
					default:
						valStrs = append(valStrs, fmt.Sprintf("'%s'", escapeSQL(fmt.Sprintf("%v", val))))
					}
				}
			}

			fmt.Fprintf(f, "INSERT INTO %s (%s) VALUES (%s);\n", table, colList, strings.Join(valStrs, ", "))
		}
		rows.Close()

		fmt.Fprintf(f, "\n")
	}

	fmt.Fprintf(f, "PRAGMA foreign_keys = ON;\n")

	return output, nil
}

func (s *BackupService) ImportDatabase(ctx context.Context, sqlPath string) error {
	data, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("read sql file: %w", err)
	}

	statements := strings.Split(string(data), ";\n")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("execute statement: %w", err)
		}
	}

	return nil
}

func (s *BackupService) ExportProject(ctx context.Context, projectID string, files map[string]string) (string, error) {
	timestamp := time.Now().UnixMilli()
	output := filepath.Join(s.storageDir, fmt.Sprintf("project_%s_%d.zip", projectID, timestamp))

	zipFile, err := os.Create(output)
	if err != nil {
		return "", fmt.Errorf("create zip: %w", err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	for path, content := range files {
		header := &zip.FileHeader{
			Name:     path,
			Method:   zip.Deflate,
			Modified: time.Now(),
		}
		if strings.HasSuffix(path, ".sh") {
			header.SetMode(0755)
		} else {
			header.SetMode(0644)
		}

		w, err := zw.CreateHeader(header)
		if err != nil {
			return "", fmt.Errorf("create zip entry %s: %w", path, err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			return "", fmt.Errorf("write %s: %w", path, err)
		}
	}

	return output, nil
}

func (s *BackupService) ImportProject(ctx context.Context, zipPath string) (map[string]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	files := make(map[string]string)

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, err)
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Name, err)
		}

		files[f.Name] = string(data)
	}

	return files, nil
}

func escapeSQL(s string) string {
	r := strings.NewReplacer(
		"'", "''",
		"\\", "\\\\",
	)
	return r.Replace(s)
}

// ===== Backup Schedules =====

type BackupSchedule struct {
	ID          int64     `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Frequency   string    `json:"frequency"`
	KeepCount   int       `json:"keep_count"`
	LastBackup  *string   `json:"last_backup_at"`
	NextBackup  *string   `json:"next_backup_at"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   string    `json:"created_at"`
}

func (s *BackupService) CreateSchedule(userID string, name, frequency string, keepCount int) (*BackupSchedule, error) {
	if keepCount < 1 {
		keepCount = 7
	}
	next := time.Now()
	switch frequency {
	case "weekly":
		next = next.AddDate(0, 0, 7)
	case "monthly":
		next = next.AddDate(0, 1, 0)
	default:
		next = next.AddDate(0, 0, 1)
	}
	res, err := s.db.Exec("INSERT INTO backup_schedules (user_id, name, frequency, keep_count, next_backup_at) VALUES (?, ?, ?, ?, ?)",
		userID, name, frequency, keepCount, next)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.getSchedule(id)
}

func (s *BackupService) getSchedule(id int64) (*BackupSchedule, error) {
	var sc BackupSchedule
	var active int
	var last, next sql.NullString
	err := s.db.QueryRow("SELECT id, user_id, name, frequency, keep_count, last_backup_at, next_backup_at, is_active, created_at FROM backup_schedules WHERE id = ?", id).
		Scan(&sc.ID, &sc.UserID, &sc.Name, &sc.Frequency, &sc.KeepCount, &last, &next, &active, &sc.CreatedAt)
	if err != nil {
		return nil, err
	}
	if last.Valid {
		sc.LastBackup = &last.String
	}
	if next.Valid {
		sc.NextBackup = &next.String
	}
	sc.IsActive = active == 1
	return &sc, nil
}

func (s *BackupService) ListSchedules(userID string) ([]BackupSchedule, error) {
	rows, err := s.db.Query("SELECT id, user_id, name, frequency, keep_count, last_backup_at, next_backup_at, is_active, created_at FROM backup_schedules WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var schedules []BackupSchedule
	for rows.Next() {
		var sc BackupSchedule
		var active int
		var last, next sql.NullString
		if err := rows.Scan(&sc.ID, &sc.UserID, &sc.Name, &sc.Frequency, &sc.KeepCount, &last, &next, &active, &sc.CreatedAt); err != nil {
			continue
		}
		if last.Valid {
			sc.LastBackup = &last.String
		}
		if next.Valid {
			sc.NextBackup = &next.String
		}
		sc.IsActive = active == 1
		schedules = append(schedules, sc)
	}
	if schedules == nil {
		schedules = []BackupSchedule{}
	}
	return schedules, nil
}

func (s *BackupService) DeleteSchedule(scheduleID int64) error {
	_, err := s.db.Exec("DELETE FROM backup_schedules WHERE id = ?", scheduleID)
	return err
}

func (s *BackupService) DeleteScheduleByUser(scheduleID int64, userID string) error {
	if userID != "" {
		_, err := s.db.Exec("DELETE FROM backup_schedules WHERE id = ? AND user_id = ?", scheduleID, userID)
		return err
	}
	return s.DeleteSchedule(scheduleID)
}

func (s *BackupService) ToggleSchedule(scheduleID int64, active bool) error {
	v := 0
	if active {
		v = 1
	}
	_, err := s.db.Exec("UPDATE backup_schedules SET is_active = ? WHERE id = ?", v, scheduleID)
	return err
}

func (s *BackupService) ToggleScheduleByUser(scheduleID int64, userID string, active bool) error {
	v := 0
	if active {
		v = 1
	}
	if userID != "" {
		_, err := s.db.Exec("UPDATE backup_schedules SET is_active = ? WHERE id = ? AND user_id = ?", v, scheduleID, userID)
		return err
	}
	return s.ToggleSchedule(scheduleID, active)
}

func (s *BackupService) RunScheduledBackup(scheduleID int64) error {
	sc, err := s.getSchedule(scheduleID)
	if err != nil {
		return err
	}
	path, err := s.ExportDatabase(context.Background())
	if err != nil {
		return err
	}
	_ = path

	now := time.Now()
	var next time.Time
	switch sc.Frequency {
	case "weekly":
		next = now.AddDate(0, 0, 7)
	case "monthly":
		next = now.AddDate(0, 1, 0)
	default:
		next = now.AddDate(0, 0, 1)
	}
	s.db.Exec("UPDATE backup_schedules SET last_backup_at = ?, next_backup_at = ? WHERE id = ?", now, next, scheduleID)
	return s.CleanupOldBackups(scheduleID)
}

func (s *BackupService) RunScheduledBackups() {
	rows, err := s.db.Query("SELECT id FROM backup_schedules WHERE is_active = 1 AND next_backup_at <= datetime('now')")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			continue
		}
		s.RunScheduledBackup(id)
	}
}

func (s *BackupService) CleanupOldBackups(scheduleID int64) error {
	var keepCount int
	s.db.QueryRow("SELECT keep_count FROM backup_schedules WHERE id = ?", scheduleID).Scan(&keepCount)
	if keepCount < 1 {
		keepCount = 7
	}

	// List backup files from the storage directory, sorted by modification time (newest first)
	entries, err := os.ReadDir(s.storageDir)
	if err != nil {
		return err
	}

	// Filter only .sql backup files and sort by time
	type backupEntry struct {
		name    string
		modTime time.Time
	}
	var backups []backupEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupEntry{name: entry.Name(), modTime: info.ModTime()})
	}

	// Sort by modification time, newest first
	for i := 0; i < len(backups); i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[j].modTime.After(backups[i].modTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	// Delete old backups beyond keepCount
	if len(backups) > keepCount {
		for _, b := range backups[keepCount:] {
			os.Remove(filepath.Join(s.storageDir, b.name))
		}
	}
	return nil
}
