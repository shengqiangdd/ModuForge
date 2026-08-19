package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
)

// dBSlogHandler wraps a slog.Handler and additionally persists WARN/ERROR
// records into the app_logs table so the admin/troubleshooting page has data.
// INFO/DEBUG still goes only to the underlying handler (stdout).
type dBSlogHandler struct {
	slog.Handler
	db *sql.DB
}

// Handle writes the record to the underlying handler and, for WARN+, asynchronously
// persists it to app_logs. DB persistence is best-effort: a failure to write the log
// must never break normal logging, so it is swallowed.
func (h *dBSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := h.Handler.Handle(ctx, r); err != nil {
		return err
	}
	if r.Level >= slog.LevelWarn {
		msg := r.Message
		module := moduleFromRecord(r)
		var details string
		if r.NumAttrs() > 0 {
			var sb strings.Builder
			r.Attrs(func(a slog.Attr) bool {
				if sb.Len() > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(a.Key)
				sb.WriteString("=")
				sb.WriteString(a.Value.String())
				return true
			})
			details = sb.String()
		}
		level := r.Level.String()
		ts := r.Time.Format("2006-01-02T15:04:05Z07:00")
		// Non-blocking best-effort write; ignore errors (logging must never fail the app).
		go func(db *sql.DB, lvl, mod, msg, det, created string) {
			_, _ = db.Exec(
				`INSERT INTO app_logs (level, module, message, details, created_at) VALUES (?, ?, ?, ?, ?)`,
				lvl, mod, msg, det, created,
			)
		}(h.db, level, module, msg, details, ts)
	}
	return nil
}

// moduleFromRecord extracts a module-ish grouping key from the record's attrs
// (preferring "module" then "server", else falling back to the enclosing group / "").
func moduleFromRecord(r slog.Record) string {
	mod := ""
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "module", "server", "tool":
			if mod == "" {
				mod = a.Value.String()
			}
		}
		return true
	})
	return mod
}

// EnableDBLogSink enables persistence of user-facing WARN/ERROR slog records into
// app_logs. Call once after the DB is open, wrapping the current default logger's
// handler so stdout output is preserved.
func EnableDBLogSink(db *sql.DB) {
	base := slog.Default().Handler()
	// Only wrap if not already wrapped (idempotent).
	if _, ok := base.(*dBSlogHandler); ok {
		return
	}
	slog.SetDefault(slog.New(&dBSlogHandler{Handler: base, db: db}))
}
