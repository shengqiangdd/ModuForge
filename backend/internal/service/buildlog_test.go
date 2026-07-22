package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildLogService_WriteAndStream(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewBuildLogService(tmpDir)

	svc.WriteLog("build_001", "INFO", "Build started")
	svc.WriteLog("build_001", "SUCCESS", "Build completed")

	// Verify file exists
	logFile := filepath.Join(tmpDir, "build_001.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("log file not created")
	}

	// Stream should return entries
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := svc.StreamLogs(ctx, "build_001")
	if err != nil {
		t.Fatalf("StreamLogs error: %v", err)
	}

	var count int
	for range ch {
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 log entries, got %d", count)
	}
}

func TestBuildLogService_MultipleBuilds(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewBuildLogService(tmpDir)

	svc.WriteLog("build_A", "INFO", "Starting A")
	svc.WriteLog("build_B", "INFO", "Starting B")
	svc.WriteLog("build_A", "ERROR", "A failed")
	svc.WriteLog("build_B", "SUCCESS", "B completed")

	// Check both files exist
	if _, err := os.Stat(filepath.Join(tmpDir, "build_A.log")); os.IsNotExist(err) {
		t.Error("build_A.log missing")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "build_B.log")); os.IsNotExist(err) {
		t.Error("build_B.log missing")
	}

	// Stream build_A - should have 2 entries
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := svc.StreamLogs(ctx, "build_A")
	if err != nil {
		t.Fatalf("StreamLogs error: %v", err)
	}

	var count int
	for range ch {
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 entries for build_A, got %d", count)
	}
}

func TestBuildLogService_StreamLogs_Nonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewBuildLogService(tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := svc.StreamLogs(ctx, "nonexistent_build")
	if err != nil {
		t.Fatalf("StreamLogs error: %v", err)
	}

	// Should get 0 entries from empty file
	var count int
	for range ch {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 entries, got %d", count)
	}
}

func TestParseLogLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		level    string
		message  string
	}{
		{"info", "[2025-01-01T12:00:00Z] [INFO] Build started", "INFO", "Build started"},
		{"error", "[2025-01-01T12:00:00Z] [ERROR] Something failed", "ERROR", "Something failed"},
		{"warn", "[2025-01-01T12:00:00Z] [WARN] Deprecated usage", "WARN", "Deprecated usage"},
		{"success", "[2025-01-01T12:00:00Z] [SUCCESS] Done", "SUCCESS", "Done"},
		{"plain", "just a message", "INFO", "just a message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := parseLogLine(tt.line)
			if entry.Level != tt.level {
				t.Errorf("level: got %q, want %q", entry.Level, tt.level)
			}
			if entry.Message != tt.message {
				t.Errorf("message: got %q, want %q", entry.Message, tt.message)
			}
		})
	}
}
