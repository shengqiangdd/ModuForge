package agent

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/moduforge/backend/internal/agent/mcp"
	"github.com/moduforge/backend/internal/agent/registry"
	_ "github.com/mattn/go-sqlite3"
)

// TestMCPWritePolicyEnforcement verifies the Claude Code-style permission
// gate: write tools are blocked unless allow_auto policy exists.
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

	// 1. Write tool without policy → blocked
	allowed, msg, err := r.mcpWriteAllowed("mcp__github__push_files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("write tool without policy must be blocked")
	}
	if !strings.Contains(msg, "自动允许") {
		t.Fatalf("block message should hint allow_auto, got: %s", msg)
	}

	// 2. executeSkill blocks the write tool end-to-end (db still clean)
	_, err = r.executeSkill(context.Background(), "mcp__github__push_files", map[string]interface{}{"owner": "x"})
	if err == nil {
		t.Fatal("executeSkill must return error for blocked write tool")
	}
	if !strings.Contains(err.Error(), "自动允许") {
		t.Fatalf("executeSkill error should hint allow_auto, got: %v", err)
	}

	// 3. Write tool with allow_auto=1 → allowed
	if _, err := db.Exec(`INSERT INTO mcp_tool_policies (server, tool, allow_auto) VALUES ('github','push_files',1)`); err != nil {
		t.Fatal(err)
	}
	allowed, _, err = r.mcpWriteAllowed("mcp__github__push_files")
	if err != nil || !allowed {
		t.Fatalf("write tool with allow_auto should pass, allowed=%v err=%v", allowed, err)
	}

	// 4. Read tool → always allowed (no policy needed)
	allowed, _, err = r.mcpWriteAllowed("mcp__github__get_me")
	if err != nil || !allowed {
		t.Fatalf("read tool must always pass, allowed=%v err=%v", allowed, err)
	}

	// 5. Read tool executes fine through executeSkill
	out, err := r.executeSkill(context.Background(), "mcp__github__get_me", nil)
	if err != nil || out != "read ok" {
		t.Fatalf("read tool should execute, out=%q err=%v", out, err)
	}
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
