package handler

import (
	"database/sql"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
)

// logSinkWriter intercepts the standard log package output. It forwards every
// line to the original destination (stderr) unchanged, and additionally persists
// lines that look like error/warning diagnostics into app_logs so the admin
// troubleshooting page covers the stdlib-log-based code paths too.
type logSinkWriter struct {
	db *sql.DB
	mu sync.Mutex
	// out is the pre-existing destination that this writer replaces. We capture
	// stderr at construction so normal log output is preserved.
	out io.Writer
	buf []byte
}

// error-ish patterns that mark a log line as worth persisting for troubleshooting.
var errLineRe = regexp.MustCompile(`(?i)(fail(ed|ing)?|error|panic|denied|timeout|timed out|limit (reached|exceeded)|retry|rollback|recover|abort|reject|fatal|deadlock|conflict|拒绝|失败|超时|熔断|panic)`)

// moduleRe extracts the leading [Module] tag, e.g. "[Agent]" or "[BashSkill]".
var moduleRe = regexp.MustCompile(`^\[?\[?([A-Za-z_][A-Za-z0-9_]*)\]?`)

// EnableLogSink routes the stdlib log package through a writer that mirrors the
// existing stderr output and persists WARN-worthy lines to app_logs.
func EnableLogSink(db *sql.DB) {
	log.SetOutput(&logSinkWriter{db: db, out: os.Stderr})
}

func (w *logSinkWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	// Original output must be preserved exactly, so mirror to the captured fd.
	if w.out != nil {
		_, _ = w.out.Write(p)
	}

	// Process complete lines (only line-oriented for persistence; partial
	// trailing data stays buffered).
	for {
		idx := indexByteFrom(w.buf, '\n', 0)
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		if line == "" {
			continue
		}
		w.persistIfDiagnostic(line)
	}
	return len(p), nil
}

// persistIfDiagnostic persists a log line to app_logs when it looks like an
// error/warning, treating the leading [Module] as the grouping key.
func (w *logSinkWriter) persistIfDiagnostic(line string) {
	if !errLineRe.MatchString(line) {
		return
	}
	mod := ""
	if m := moduleRe.FindStringSubmatch(line); len(m) > 1 {
		mod = m[1]
	}
	msg := strings.TrimSpace(line)
	go func(db *sql.DB, module, message string) {
		if db == nil {
			return
		}
		_, _ = db.Exec(
			`INSERT INTO app_logs (level, module, message, details, created_at) VALUES ('WARN', ?, ?, ?, datetime('now'))`,
			module, "", message, // message holds the full line
		)
	}(w.db, mod, msg)
}

func indexByteFrom(b []byte, c byte, start int) int {
	for i := start; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}
