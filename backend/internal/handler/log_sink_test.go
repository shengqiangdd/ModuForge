package handler

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func TestLogSink_PersistsErrorLines(t *testing.T) {
	db := openSlogTestDB(t)
	defer db.Close()

	var buf bytes.Buffer
	lw := &logSinkWriter{db: db, out: &buf}
	log.New(lw, "", 0)

	lw.Write([]byte("2026/08/19 12:00:00 [Agent] self-reflection triggered: create_repository failed 3 times\n"))
	lw.Write([]byte("2026/08/19 12:00:00 [Security] DENIED session=a cmd=rm rules=xxx\n"))

	time.Sleep(120 * time.Millisecond)
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM app_logs`).Scan(&n)
	if n != 2 {
		t.Fatalf("expected 2 diagnostic lines persisted, got %d", n)
	}
	var mod, msg string
	db.QueryRow(`SELECT module, message FROM app_logs WHERE module='Agent' LIMIT 1`).Scan(&mod, &msg)
	if mod != "Agent" || !strings.Contains(msg, "create_repository failed") {
		t.Fatalf("row = module=%q msg=%q", mod, msg)
	}
}

func TestLogSink_SkipsNoiseAndPreservesOutput(t *testing.T) {
	db := openSlogTestDB(t)
	defer db.Close()

	var orig bytes.Buffer
	lw := &logSinkWriter{db: db, out: &orig}
	lw.Write([]byte("2026/08/19 12:00:00 [Agent] heartbeat ok\n")) // info, no error token
	lw.Write([]byte("2026/08/19 12:00:00 [Agent] planning tool call\n"))

	time.Sleep(120 * time.Millisecond)
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM app_logs`).Scan(&n)
	if n != 0 {
		t.Fatalf("expected 0 rows for non-diagnostic lines, got %d", n)
	}
	// Original output must have been mirrored exactly (2 lines).
	if !strings.Contains(orig.String(), "heartbeat ok") || !strings.Contains(orig.String(), "planning tool call") {
		t.Fatalf("original output not preserved: %q", orig.String())
	}
}
