package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/agent"
	"github.com/moduforge/backend/internal/agent/mcp"
	"github.com/moduforge/backend/internal/agent/registry"
	"github.com/moduforge/backend/internal/agent/skills"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/service"
	"github.com/moduforge/backend/internal/storage"
)

type AgentHandler struct {
	runner *agent.AgentRunner
	db     *database.DB
	cfg    *config.Config
	mcpMgr *mcp.Manager

	// agentSem limits concurrent Agent runs (server-level resource guard).
	// Multiple users share the global LLM key when they have no custom
	// provider, so unbounded concurrency can exhaust RPM/TPM quotas and
	// server CPU/memory. See also per-key rate limiting in agent/llm.go.
	agentSem chan struct{}
}

// maxConcurrentAgents is the default server-wide cap for concurrent
// Agent runs. Overridable via AGENT_MAX_CONCURRENT env (see config).
const maxConcurrentAgents = 4

func NewAgentHandler(cfg *config.Config, db *database.DB) *AgentHandler {
	// Auto-register all skills via init() factories — no manual registration needed
	memStore := service.NewMemoryStore(db.Conn)

	// Create file hash cache shared between skills and runner
	fileHashCache := agent.NewFileHashCache()

	deps := &registry.Deps{
		DB:            db.Conn,
		StoragePath:   cfg.StoragePath,
		LLMApiKey:     cfg.EffectiveLLMKey(),
		LLMEndpoint:   cfg.LLMEndpoint,
		LLMModel:      cfg.LLMModel,
		HTTPClient:    agent.LLMHTTPClient(),
		MemoryStore:   memStore,
		FileHashCache: fileHashCache,
	}

	// Initialize S3-compatible storage if endpoint is configured
	if cfg.S3Endpoint != "" {
		// Retry up to 30s for SeaweedFS to start
		var s3adapter *storage.S3Adapter
		var s3err error
		for i := 0; i < 30; i++ {
			s3adapter, s3err = storage.NewS3Adapter(storage.S3Config{
				Endpoint:  cfg.S3Endpoint,
				AccessKey: cfg.S3AccessKey,
				SecretKey: cfg.S3SecretKey,
				Bucket:    cfg.S3Bucket,
				Prefix:    "projects",
				Secure:    false,
			})
			if s3err == nil {
				deps.Storage = s3adapter
				slog.Info("S3 storage enabled", "endpoint", cfg.S3Endpoint, "bucket", cfg.S3Bucket)
				break
			}
			slog.Warn("S3 storage init failed, retrying...", "attempt", i+1, "error", s3err)
			time.Sleep(1 * time.Second)
		}
		if s3err != nil {
			slog.Warn("S3 storage init failed after 30 retries, falling back to legacy storage", "error", s3err)
		}
	}
	registry := agent.NewSkillRegistry(deps)

	// MCP (Model Context Protocol) integration — connect to external tool
	// servers (filesystem, GitHub, database, etc.) and expose their tools as
	// dynamic agent skills. Config sources: DB (UI-managed, takes precedence)
	// then MCP_SERVERS env / MCP_SERVERS_FILE.
	ctx := context.Background()
	mcpMgr := mcp.NewManager()
	mcpMgr.SetOnServerReady(func(client *mcp.Client) {
		for _, tool := range client.Tools() {
			registry.Register(mcp.NewToolSkill(client, tool))
			slog.Info("MCP tool registered", "server", client.Name, "tool", tool.Name)
		}
	})
	// 1. DB-managed servers (persisted via MCP page / API)
	loadMCPServersFromDB(ctx, db.Conn, mcpMgr)
	// 2. Static env/file servers (kept as fallback; UI-managed names win)
	if err := mcpMgr.LoadFromEnv(ctx); err != nil {
		slog.Warn("MCP manager init failed", "error", err)
	}
	// 3. Register tools from clients that were ready before the callback was set
	for _, client := range mcpMgr.Clients() {
		if client.IsReady() {
			for _, tool := range client.Tools() {
				registry.Register(mcp.NewToolSkill(client, tool))
				slog.Info("MCP tool registered", "server", client.Name, "tool", tool.Name)
			}
		}
	}
	// skills_doc — dynamic skill catalog (SKILLS.md equivalent). Registered
	// after MCP tools so it reflects the complete live skill set.
	registry.Register(skills.NewSkillsDocSkill(registry))

	runner := agent.NewAgentRunner(
		registry,
		cfg.EffectiveLLMKey(),
		cfg.LLMEndpoint,
		cfg.LLMModel,
		db.Conn,
	)
	runner.SetMemoryStore(memStore)
	runner.SetFileHashCache(fileHashCache)
	runner.SetMonthlyCostLimit(cfg.MonthlyCostLimit)

	return &AgentHandler{
		runner: runner,
		db:     db,
		cfg:    cfg,
		mcpMgr: mcpMgr,
		agentSem: make(chan struct{}, maxConcurrentAgents),
	}
}

// GetCacheStats returns cache statistics for the agent
func (h *AgentHandler) GetCacheStats(c fiber.Ctx) error {
	if h.runner == nil {
		return c.JSON(fiber.Map{
			"status":  "unavailable",
			"message": "Agent runner not configured",
		})
	}

	stats := h.runner.GetCacheStats()
	return c.JSON(fiber.Map{
		"status": "ok",
		"caches": stats,
	})
}

// GetRunner exposes the underlying AgentRunner so other handlers can share
// its services (e.g. monthly cost cap for AI chat/generate).
func (h *AgentHandler) GetRunner() *agent.AgentRunner {
	return h.runner
}

// loadAgentSettings 从数据库读取 Agent 配置并应用到 RunConfig（不修改共享 runner 状态）
func (h *AgentHandler) loadAgentSettings(cfg *agent.RunConfig) {
	if h.db == nil {
		return
	}
	// 确保表存在
	h.db.Conn.Exec(`CREATE TABLE IF NOT EXISTS agent_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	var val string
	if err := h.db.Conn.QueryRow("SELECT value FROM agent_settings WHERE key='max_iterations'").Scan(&val); err == nil {
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 && n <= 200 {
			cfg.MaxIterations = n
		}
	}
	if err := h.db.Conn.QueryRow("SELECT value FROM agent_settings WHERE key='max_result_len'").Scan(&val); err == nil {
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil && n > 0 && n <= 100000 {
			cfg.MaxResultLen = n
		}
	}
}

// bufioSSEWriter wraps bufio.Writer and implements agent.SSEWriter.
// Unlike the old fiberSSEWriter (which had a no-op Flush), this writer
// flushes after every write so SSE events reach the client in real time.
// It tracks connection state — once a write fails, all subsequent writes
// return immediately without attempting I/O (preventing busy-loop on
// disconnected clients).
type bufioSSEWriter struct {
	bw           *bufio.Writer
	disconnected bool
	mu           sync.Mutex // serializes concurrent writes (parallel tools, keepalives)
}

func (w *bufioSSEWriter) WriteSSE(data map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.disconnected {
		return io.ErrClosedPipe
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = w.bw.Write([]byte("data: " + string(jsonBytes) + "\n\n"))
	if err != nil {
		w.disconnected = true
		return err
	}
	return w.bw.Flush()
}

func (w *bufioSSEWriter) WriteSSEPlain(data string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.disconnected {
		return io.ErrClosedPipe
	}
	_, err := w.bw.Write([]byte("data: " + data + "\n\n"))
	if err != nil {
		w.disconnected = true
		return err
	}
	return w.bw.Flush()
}

func (w *bufioSSEWriter) WriteSSEComment(comment string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.disconnected {
		return io.ErrClosedPipe
	}
	_, err := w.bw.Write([]byte(": " + comment + "\n\n"))
	if err != nil {
		w.disconnected = true
		return err
	}
	return w.bw.Flush()
}

// IsDisconnected returns true if the underlying connection has been lost.
func (w *bufioSSEWriter) IsDisconnected() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.disconnected
}

// FlushWithTimeout flushes with a timeout to avoid blocking on disconnected clients.
func (w *bufioSSEWriter) FlushWithTimeout(d time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- w.Flush()
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		return fmt.Errorf("flush timeout after %v", d)
	}
}

func (w *bufioSSEWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.bw.Flush()
}

// answerCaptureWriter wraps an SSEWriter and intercepts agent step events
// to capture the final answer and intermediate steps for persistence.
type answerCaptureWriter struct {
	agent.SSEWriter
	onAnswer func(string)
	onStep   func(stepType, content string) // called for think/skill_call/skill_result/answer/reasoning
	// Accumulate reasoning chunks (streaming) into a single step
	reasoningBuf strings.Builder
	flushed      bool
	// Track tool calls for persistence
	toolCallsJSON string
	toolCallID    string
	// Last token usage reported by the LLM (persisted with the assistant message)
	tokenUsageJSON string
	mu             sync.Mutex // serializes concurrent WriteSSE from parallel tool goroutines
}

func (w *answerCaptureWriter) WriteSSE(data map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	// Capture token usage for persistence (frontend also receives this event directly)
	if dataType, _ := data["type"].(string); dataType == "usage" {
		if usage, ok := data["usage"]; ok {
			if b, err := json.Marshal(usage); err == nil {
				w.tokenUsageJSON = string(b)
			}
		}
		return w.SSEWriter.WriteSSE(data)
	}
	// Accumulate reasoning chunks (streaming LLM extended thinking)
	if dataType, _ := data["type"].(string); dataType == "reasoning" {
		if content, _ := data["content"].(string); content != "" {
			w.reasoningBuf.WriteString(content)
		}
		return w.SSEWriter.WriteSSE(data)
	}
	// Flush accumulated reasoning when a non-reasoning event arrives
	if w.reasoningBuf.Len() > 0 && !w.flushed {
		w.flushed = true
		if w.onStep != nil {
			w.onStep("reasoning", w.reasoningBuf.String())
		}
	}
	if step, ok := data["step"].(string); ok {
		content, _ := data["content"].(string)
		skill, _ := data["skill"].(string)
		switch step {
		case "answer":
			if content != "" {
				w.onAnswer(content)
			}
			if w.onStep != nil {
				w.onStep("answer", content)
			}
		case "think":
			if w.onStep != nil && content != "" {
				w.onStep("think", content)
			}
		case "skill_call":
			if w.onStep != nil && skill != "" {
				input, _ := json.Marshal(data["input"])
				w.onStep("skill_call", fmt.Sprintf("%s: %s", skill, string(input)))
				// Capture tool_call_id if present
				if tcID, ok := data["tool_call_id"].(string); ok {
					w.toolCallID = tcID
				}
				// Capture tool_calls JSON if present
				if tcJSON, ok := data["tool_calls"].(string); ok {
					w.toolCallsJSON = tcJSON
				}
			}
		case "skill_result":
			if w.onStep != nil && content != "" {
				// Save truncated content for persistence — Agent needs tool results in multi-round history
				summary := content
				if len(summary) > 2000 {
					summary = summary[:2000] + "... [truncated]"
				}
				if blocked, _ := data["blocked"].(bool); blocked {
					summary = content
				}
				w.onStep("skill_result", summary)
			}
		}
	}
	return w.SSEWriter.WriteSSE(data)
}
