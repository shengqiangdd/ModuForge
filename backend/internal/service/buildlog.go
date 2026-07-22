package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type BuildLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // INFO, WARN, ERROR, SUCCESS
	Message   string    `json:"message"`
}

type BuildLogService struct {
	mu      sync.RWMutex
	logsDir string
}

func NewBuildLogService(logsDir string) *BuildLogService {
	os.MkdirAll(logsDir, 0755)
	return &BuildLogService{logsDir: logsDir}
}

func (s *BuildLogService) WriteLog(buildID string, level, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := BuildLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	logFile := filepath.Join(s.logsDir, buildID+".log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "[%s] [%s] %s\n", entry.Timestamp.Format(time.RFC3339), level, message)
}

func (s *BuildLogService) StreamLogs(ctx context.Context, buildID string) (<-chan BuildLogEntry, error) {
	logFile := filepath.Join(s.logsDir, buildID+".log")

	ch := make(chan BuildLogEntry, 100)

	// Check if file exists, create if not
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("open log file: %w", err)
	}

	go func() {
		defer close(ch)
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			entry := parseLogLine(line)

			select {
			case ch <- entry:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// WatchLogs 实时监听日志文件变化（用于 SSE 推送）
func (s *BuildLogService) WatchLogs(ctx context.Context, buildID string) (<-chan BuildLogEntry, error) {
	logFile := filepath.Join(s.logsDir, buildID+".log")
	ch := make(chan BuildLogEntry, 100)

	go func() {
		defer close(ch)

		// Wait for file to exist
		for i := 0; i < 30; i++ {
			if _, err := os.Stat(logFile); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
		}

		f, err := os.Open(logFile)
		if err != nil {
			return
		}
		defer f.Close()

		// Seek to end for tailing
		f.Seek(0, io.SeekEnd)
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			entry := parseLogLine(line)
			select {
			case ch <- entry:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func parseLogLine(line string) BuildLogEntry {
	entry := BuildLogEntry{}

	// Format: [2025-01-01T12:00:00Z] [INFO] message
	if len(line) > 2 && line[0] == '[' {
		if idx := strings.Index(line, "] ["); idx > 0 {
			entry.Timestamp, _ = time.Parse(time.RFC3339, strings.TrimPrefix(line[1:idx], " "))
		}
	}

	if strings.Contains(line, "[ERROR]") {
		entry.Level = "ERROR"
	} else if strings.Contains(line, "[WARN]") {
		entry.Level = "WARN"
	} else if strings.Contains(line, "[SUCCESS]") {
		entry.Level = "SUCCESS"
	} else {
		entry.Level = "INFO"
	}

	// Extract message after last bracket
	if idx := strings.LastIndex(line, "] "); idx > 0 {
		entry.Message = strings.TrimSpace(line[idx+2:])
	} else {
		entry.Message = line
	}

	return entry
}
