package agent

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moduforge/backend/internal/agent/mcp"
	"github.com/moduforge/backend/internal/agent/registry"
	_ "github.com/mattn/go-sqlite3"
)

// TestMCPWritePolicyEnforcement verifies the Claude Code-style permission
// gate: write tools are blocked unless an allow/ask policy exists.
func TestMCPWritePolicyEnforcement(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE mcp_tool_policies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server TEXT NOT NULL,
		tool TEXT NOT NULL,
		allow_auto INTEGER NOT NULL DEFAULT 0,
		mode TEXT NOT NULL DEFAULT 'deny',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(server, tool)
	)`); err != nil {
		t.Fatal(err)
	}

	reg := registry.NewSkillRegistry(&registry.Deps{DB: db})
	// Register a fake write tool and a fake read tool (stand-ins for MCP ToolSkill)
	reg.Register(writeFakeSkill{})
	reg.Register(readFakeSkill{})

	r := NewAgentRunner(reg, "k", "http://x", "m", db)

	// 1. Write tool without policy → blocked (deny)
	decision, msg, err := r.mcpPermissionDecision("mcp__github__push_files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision != "deny" {
		t.Fatalf("write tool without policy must be deny, got %q", decision)
	}
	if !strings.Contains(msg, "写操作") {
		t.Fatalf("block message should mention write op, got: %s", msg)
	}

	// 2. executeSkill blocks the write tool end-to-end (db still clean)
	_, err = r.executeSkill(context.Background(), "mcp__github__push_files", map[string]interface{}{"owner": "x"}, nil)
	if err == nil {
		t.Fatal("executeSkill must return error for blocked write tool")
	}
	if !strings.Contains(err.Error(), "写操作") {
		t.Fatalf("executeSkill error should mention write op, got: %v", err)
	}

	// 3. Write tool with mode='allow' → allowed
	if _, err := db.Exec(`INSERT INTO mcp_tool_policies (server, tool, allow_auto, mode) VALUES ('github','push_files',1,'allow')`); err != nil {
		t.Fatal(err)
	}
	decision, _, err = r.mcpPermissionDecision("mcp__github__push_files")
	if err != nil || decision != "allow" {
		t.Fatalf("write tool with mode=allow should pass, decision=%q err=%v", decision, err)
	}

	// 4. Read tool → always allowed (no policy needed)
	decision, _, err = r.mcpPermissionDecision("mcp__github__get_me")
	if err != nil || decision != "allow" {
		t.Fatalf("read tool must always pass, decision=%q err=%v", decision, err)
	}

	// 5. Read tool executes fine through executeSkill
	out, err := r.executeSkill(context.Background(), "mcp__github__get_me", nil, nil)
	if err != nil || out != "read ok" {
		t.Fatalf("read tool should execute, out=%q err=%v", out, err)
	}
}

// TestMCPAskModeApproval verifies ask-mode: the tool call suspends, a
// permission_request event is emitted, and ResolveApproval(true) lets it run.
func TestMCPAskModeApproval(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE mcp_tool_policies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server TEXT NOT NULL,
		tool TEXT NOT NULL,
		allow_auto INTEGER NOT NULL DEFAULT 0,
		mode TEXT NOT NULL DEFAULT 'deny',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(server, tool)
	)`); err != nil {
		t.Fatal(err)
	}
	// ask-mode policy for the write tool
	if _, err := db.Exec(`INSERT INTO mcp_tool_policies (server, tool, allow_auto, mode) VALUES ('github','push_files',0,'ask')`); err != nil {
		t.Fatal(err)
	}

	reg := registry.NewSkillRegistry(&registry.Deps{DB: db})
	reg.Register(writeFakeSkill{})

	r := NewAgentRunner(reg, "k", "http://x", "m", db)

	decision, _, err := r.mcpPermissionDecision("mcp__github__push_files")
	if err != nil || decision != "ask" {
		t.Fatalf("ask-mode policy should return ask, decision=%q err=%v", decision, err)
	}

	// Execute in a goroutine — it should block waiting for approval.
	writer := &recordingSSEWriter{}
	type result struct {
		out string
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		out, err := r.executeSkill(context.Background(), "mcp__github__push_files", map[string]interface{}{"owner": "x"}, writer)
		resCh <- result{out, err}
	}()

	// Wait for the permission_request event.
	deadline := time.Now().Add(3 * time.Second)
	var reqID string
	for time.Now().Before(deadline) {
		reqID = writer.permissionRequestID()
		if reqID != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if reqID == "" {
		t.Fatal("permission_request event was never emitted")
	}
	if writer.permissionTool() != "push_files" {
		t.Fatalf("permission tool = %q, want push_files", writer.permissionTool())
	}

	// Resolve with allow=true → the tool should run.
	if !r.ResolveApproval(reqID, true) {
		t.Fatalf("ResolveApproval(%s, true) returned false", reqID)
	}

	select {
	case res := <-resCh:
		if res.err != nil {
			t.Fatalf("executeSkill after approval: %v", res.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("executeSkill did not resume after approval")
	}
}

// TestMCPAskModeDenied verifies ask-mode rejection blocks the call.
func TestMCPAskModeDenied(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE mcp_tool_policies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server TEXT NOT NULL,
		tool TEXT NOT NULL,
		allow_auto INTEGER NOT NULL DEFAULT 0,
		mode TEXT NOT NULL DEFAULT 'deny',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(server, tool)
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mcp_tool_policies (server, tool, allow_auto, mode) VALUES ('github','push_files',0,'ask')`); err != nil {
		t.Fatal(err)
	}

	reg := registry.NewSkillRegistry(&registry.Deps{DB: db})
	reg.Register(writeFakeSkill{})
	r := NewAgentRunner(reg, "k", "http://x", "m", db)

	writer := &recordingSSEWriter{}
	type result struct {
		out string
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		out, err := r.executeSkill(context.Background(), "mcp__github__push_files", map[string]interface{}{"owner": "x"}, writer)
		resCh <- result{out, err}
	}()

	deadline := time.Now().Add(3 * time.Second)
	var reqID string
	for time.Now().Before(deadline) {
		reqID = writer.permissionRequestID()
		if reqID != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if reqID == "" {
		t.Fatal("permission_request event was never emitted")
	}

	if !r.ResolveApproval(reqID, false) {
		t.Fatalf("ResolveApproval(%s, false) returned false", reqID)
	}
	select {
	case res := <-resCh:
		if res.err == nil {
			t.Fatal("executeSkill should fail after rejection")
		}
		if !strings.Contains(res.err.Error(), "拒绝") {
			t.Fatalf("error should mention rejection, got: %v", res.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("executeSkill did not resume after rejection")
	}
}

// recordingSSEWriter records SSE events, focusing on permission_request.
type recordingSSEWriter struct {
	mu        sync.Mutex
	events    []map[string]interface{}
	reqID     string
	reqServer string
	reqTool   string
}

func (w *recordingSSEWriter) WriteSSE(data map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.events = append(w.events, data)
	if t, _ := data["type"].(string); t == "permission_request" {
		w.reqID, _ = data["request_id"].(string)
		w.reqServer, _ = data["server"].(string)
		w.reqTool, _ = data["tool"].(string)
	}
	return nil
}
func (w *recordingSSEWriter) WriteSSEPlain(data string) error            { return nil }
func (w *recordingSSEWriter) WriteSSEComment(comment string) error       { return nil }
func (w *recordingSSEWriter) Flush() error                               { return nil }
func (w *recordingSSEWriter) IsDisconnected() bool                       { return false }

func (w *recordingSSEWriter) permissionRequestID() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reqID
}
func (w *recordingSSEWriter) permissionTool() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.reqTool
}

type writeFakeSkill struct{}

func (writeFakeSkill) Name() string                  { return "mcp__github__push_files" }
func (writeFakeSkill) Description() string           { return "fake write tool" }
func (writeFakeSkill) Metadata() registry.SkillMeta { return registry.SkillMeta{} }
func (writeFakeSkill) Parameters() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}
func (writeFakeSkill) Writes() bool { return mcp.IsWriteTool(mcp.Tool{Name: "push_files"}) }
func (writeFakeSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	return "should not reach", nil
}

type readFakeSkill struct{}

func (readFakeSkill) Name() string                  { return "mcp__github__get_me" }
func (readFakeSkill) Description() string           { return "fake read tool" }
func (readFakeSkill) Metadata() registry.SkillMeta { return registry.SkillMeta{} }
func (readFakeSkill) Parameters() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}
func (readFakeSkill) Writes() bool { return mcp.IsWriteTool(mcp.Tool{Name: "get_me"}) }
func (readFakeSkill) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	return "read ok", nil
}
