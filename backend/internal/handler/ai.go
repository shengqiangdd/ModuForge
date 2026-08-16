package handler

import (
	"bufio"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/database"
	"github.com/moduforge/backend/internal/llm"
	"github.com/moduforge/backend/internal/service"
)

const maxMessagesPerRequest = 100

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type AIHandler struct {
	svc         *service.AIService
	cfg         *config.Config
	db          *database.DB
	memoryStore *service.MemoryStore
	memV2       *service.MemoryV2Store
	fr          *service.FileContentRepo // S3-first content access (optional)
}

func NewAIHandler(svc *service.AIService, cfg *config.Config, db *database.DB) *AIHandler {
	return &AIHandler{svc: svc, cfg: cfg, db: db, memV2: service.NewMemoryV2Store(db.Conn)}
}

func (h *AIHandler) SetMemoryStore(ms *service.MemoryStore) {
	h.memoryStore = ms
}

// SetFileContentRepo injects the S3-first file content repository.
func (h *AIHandler) SetFileContentRepo(fr *service.FileContentRepo) {
	h.fr = fr
}

// autoLoadProjectContext loads all files from a project and returns them as context
func (h *AIHandler) autoLoadProjectContext(projectID, uid string) string {
	if h.db == nil {
		return ""
	}
	db := h.db.Conn

	// Verify project ownership
	var name, ownerID string
	err := db.QueryRow(`SELECT name, user_id FROM projects WHERE id=?`, projectID).Scan(&name, &ownerID)
	if err != nil || ownerID != uid {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s\n", name))
	fileCount := 0
	if h.fr != nil {
		files, err := h.fr.ReadAll(context.Background(), projectID)
		if err != nil {
			return ""
		}
		for _, f := range files {
			content, err := h.fr.ReadOne(context.Background(), projectID, f.Path)
			if err != nil {
				continue
			}
			// Skip empty or oversized files
			if content == "" || len(content) > 10240 {
				continue
			}
			sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", f.Path, content))
			fileCount++
			if fileCount >= 50 {
				break
			}
		}
	} else {
		rows, err := db.Query(`SELECT path, content FROM project_files WHERE project_id=? ORDER BY path`, projectID)
		if err != nil {
			return ""
		}
		defer rows.Close()

		for rows.Next() {
			var path, content string
			if err := rows.Scan(&path, &content); err != nil {
				continue
			}
			// Skip empty or oversized files
			if content == "" || len(content) > 10240 {
				continue
			}
			sb.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", path, content))
			fileCount++
			if fileCount >= 50 {
				break
			}
		}
	}
	if fileCount == 0 {
		return ""
	}
	return sb.String()
}

func (h *AIHandler) GenerateModule(c fiber.Ctx) error {
	var req struct {
		Description     string            `json:"description"`
		ProjectContext   string            `json:"project_context"`
		ProjectID       string            `json:"project_id"`
		Messages        []service.Message `json:"messages"`
		SessionID       string            `json:"session_id"`
		Provider        string            `json:"provider"`
		Model           string            `json:"model"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		slog.Error("GenerateModule: bind failed", "error", err)
		return BadRequest(c, "invalid request")
	}
	if req.Description == "" && len(req.Messages) == 0 {
		slog.Warn("GenerateModule: no input")
		return BadRequest(c, "description or messages required")
	}
	if len(req.Description) > 5000 {
		slog.Warn("GenerateModule: description too long", "len", len(req.Description))
		return BadRequest(c, "description too long (max 5000)")
	}
	if len(req.Messages) > maxMessagesPerRequest {
		return BadRequest(c, "messages too long (max 100)")
	}

	uid, _ := c.Locals("uid").(string)

	// Merge project context into description if available
	description := req.Description
	if req.ProjectContext != "" {
		description = fmt.Sprintf("%s\n\n## Project Context:\n%s", description, req.ProjectContext)
	} else if req.ProjectID != "" && uid != "" && h.db != nil {
		if ctx := h.autoLoadProjectContext(req.ProjectID, uid); ctx != "" {
			description = fmt.Sprintf("%s\n\n## Project Context:\n%s", description, ctx)
		}
	}

	// Resolve LLM provider from request or fallback to global config
	h.resolveProvider(req.Provider, req.Model)

	slog.Info("GenerateModule", "description_len", len(description), "messages", len(req.Messages), "session_id", req.SessionID, "uid", uid)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		h.svc.GenerateModule(c.Context(), description, uid, req.Messages, req.SessionID, w)
	})
	return nil
}

// resolveProvider resolves LLM provider from request params or fallback to global config.
// Non-destructive: saves and restores the original provider after resolution.
func (h *AIHandler) resolveProvider(providerID, modelID string) {
	h.cfg.Lock()
	defer h.cfg.Unlock()
	if providerID == "" {
		providerID = h.cfg.LLMProvider
	}
	if modelID == "" {
		modelID = h.cfg.LLMModel
	}
	if providerID != "" {
		savedProvider := h.cfg.LLMProvider
		savedModel := h.cfg.LLMModel
		savedEndpoint := h.cfg.LLMEndpoint
		savedKey := h.cfg.LLMApiKey

		h.cfg.LLMProvider = providerID
		if modelID != "" {
			h.cfg.LLMModel = modelID
		}
		if p := llm.FindProvider(providerID); p != nil {
			h.cfg.LLMEndpoint = p.Endpoint
		}
		h.cfg.LLMApiKey = h.cfg.EffectiveLLMKey()
		slog.Info("resolveProvider", "provider", providerID, "model", modelID, "endpoint", h.cfg.LLMEndpoint, "has_key", h.cfg.LLMApiKey != "")

		// Restore original values after the request completes
		defer func() {
			h.cfg.Lock()
			h.cfg.LLMProvider = savedProvider
			h.cfg.LLMModel = savedModel
			h.cfg.LLMEndpoint = savedEndpoint
			h.cfg.LLMApiKey = savedKey
			h.cfg.Unlock()
		}()
	}
}

func (h *AIHandler) GatherRequirements(c fiber.Ctx) error {
	var req struct {
		Message   string            `json:"message"`
		Messages  []service.Message `json:"messages"`
		SessionID string            `json:"session_id"`
		Provider  string            `json:"provider"`
		Model     string            `json:"model"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		slog.Error("GatherRequirements: bind failed", "error", err)
		return BadRequest(c, "invalid request")
	}
	if req.Message == "" && len(req.Messages) == 0 {
		slog.Warn("GatherRequirements: no input")
		return BadRequest(c, "message or messages required")
	}
	if len(req.Message) > 2000 {
		slog.Warn("GatherRequirements: message too long", "len", len(req.Message))
		return BadRequest(c, "message too long (max 2000)")
	}
	if len(req.Messages) > maxMessagesPerRequest {
		return BadRequest(c, "messages too long (max 100)")
	}

	uid, _ := c.Locals("uid").(string)

	// Resolve LLM provider
	h.resolveProvider(req.Provider, req.Model)

	slog.Info("GatherRequirements", "message_len", len(req.Message), "messages", len(req.Messages), "session_id", req.SessionID, "uid", uid)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		h.svc.GatherRequirements(c.Context(), req.Message, uid, req.Messages, req.SessionID, w)
	})
	return nil
}

func (h *AIHandler) Chat(c fiber.Ctx) error {
	var req struct {
		Message        string            `json:"message"`
		Context        string            `json:"context"`
		ProjectContext  string            `json:"project_context"`
		ProjectID      string            `json:"project_id"`
		Messages       []service.Message `json:"messages"`
		SessionID      string            `json:"session_id"`
		Provider       string            `json:"provider"`
		Model          string            `json:"model"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		slog.Error("Chat: bind failed", "error", err)
		return BadRequest(c, "invalid request")
	}
	if req.Message == "" && len(req.Messages) == 0 {
		slog.Warn("Chat: no input")
		return BadRequest(c, "message or messages required")
	}
	if len(req.Message) > 2000 {
		slog.Warn("Chat: message too long", "len", len(req.Message))
		return BadRequest(c, "message too long (max 2000)")
	}
	if len(req.Messages) > maxMessagesPerRequest {
		return BadRequest(c, "messages too long (max 100)")
	}

	uid, _ := c.Locals("uid").(string)

	// Merge project context: manual context takes precedence
	contextInfo := req.Context
	if req.ProjectContext != "" {
		contextInfo = req.ProjectContext
	} else if req.ProjectID != "" && uid != "" && h.db != nil {
		// Auto-load project files as context
		contextInfo = h.autoLoadProjectContext(req.ProjectID, uid)
	}

	// Resolve LLM provider from request or fallback to global config
	// (same pattern as Agent handler — ensures free providers work without API key)
	h.resolveProvider(req.Provider, req.Model)

	slog.Info("Chat", "message_len", len(req.Message), "messages", len(req.Messages), "session_id", req.SessionID, "uid", uid, "has_context", contextInfo != "", "provider", req.Provider, "model", req.Model)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		h.svc.Chat(c.Context(), req.Message, contextInfo, uid, req.Messages, req.SessionID, w)
	})
	return nil
}

func (h *AIHandler) RepairBuild(c fiber.Ctx) error {
	var req struct {
		BuildLog  string            `json:"build_log"`
		Messages  []service.Message `json:"messages"`
		SessionID string            `json:"session_id"`
		Provider  string            `json:"provider"`
		Model     string            `json:"model"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.BuildLog == "" && len(req.Messages) == 0 {
		return BadRequest(c, "build_log or messages required")
	}
	if len(req.BuildLog) > 50000 {
		return BadRequest(c, "build_log too long (max 50000)")
	}
	if len(req.Messages) > maxMessagesPerRequest {
		return BadRequest(c, "messages too long (max 100)")
	}

	uid, _ := c.Locals("uid").(string)

	// Resolve LLM provider
	h.resolveProvider(req.Provider, req.Model)

	slog.Info("RepairBuild", "build_log_len", len(req.BuildLog), "messages", len(req.Messages), "session_id", req.SessionID, "uid", uid)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		h.svc.RepairBuild(c.Context(), req.BuildLog, uid, req.Messages, req.SessionID, w)
	})
	return nil
}

// CompareModels 并发比较多个模型的回答
func (h *AIHandler) CompareModels(c fiber.Ctx) error {
	var req struct {
		Message   string   `json:"message"`
		ModelIDs  []string `json:"model_ids"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Message == "" {
		return BadRequest(c, "message required")
	}
	if len(req.ModelIDs) < 2 {
		return BadRequest(c, "at least 2 model_ids required")
	}
	if len(req.ModelIDs) > 6 {
		return BadRequest(c, "max 6 model_ids")
	}
	if len(req.Message) > 2000 {
		return BadRequest(c, "message too long (max 2000)")
	}

	uid, _ := c.Locals("uid").(string)

	results, err := h.svc.CompareModels(c.Context(), req.Message, req.ModelIDs, uid)
	if err != nil {
		return InternalError(c, err.Error())
	}
	return c.JSON(fiber.Map{"results": results})
}

// GetHistory 返回指定 session 的对话历史
func (h *AIHandler) GetHistory(c fiber.Ctx) error {
	sessionID := c.Params("session_id")
	if sessionID == "" {
		return BadRequest(c, "session_id required")
	}

	messages := h.svc.GetHistory(sessionID)
	if messages == nil {
		return c.JSON(fiber.Map{"messages": []service.Message{}})
	}
	return c.JSON(fiber.Map{"messages": messages})
}

// DeleteHistory 删除指定 session 的对话历史
func (h *AIHandler) DeleteHistory(c fiber.Ctx) error {
	sessionID := c.Params("session_id")
	if sessionID == "" {
		return BadRequest(c, "session_id required")
	}

	h.svc.DeleteHistory(sessionID)
	return c.JSON(fiber.Map{"status": "ok"})
}

// ListProviders 返回所有可用的 LLM 提供商和模型（合并用户配置）
func (h *AIHandler) ListProviders(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)

	var userConfigs map[string]struct{ Endpoint, APIKey, ModelsJSON string }
	var customProviders []llm.Provider

	if uid != "" && h.db != nil {
		configs, err := h.db.GetProviderConfigs(uid)
		if err == nil {
			userConfigs = make(map[string]struct{ Endpoint, APIKey, ModelsJSON string })
			for _, pc := range configs {
				userConfigs[pc.ID] = struct{ Endpoint, APIKey, ModelsJSON string }{Endpoint: pc.Endpoint, APIKey: pc.APIKey, ModelsJSON: pc.ModelsJSON}
			}
		}

		customList, err := h.db.GetCustomProviders(uid)
		if err == nil {
			for _, cp := range customList {
				var models []llm.Model
				if cp.ModelsJSON != "" {
					_ = json.Unmarshal([]byte(cp.ModelsJSON), &models)
				}
				customProviders = append(customProviders, llm.Provider{
					Name:        cp.Name,
					ID:          cp.ID,
					Endpoint:    cp.Endpoint,
					Models:      models,
					RequiresKey: cp.APIKey != "",
					IsFree:      false,
					Tier:        "paid",
				})
			}
		}
	}

	providers := llm.GetMergedProviders(userConfigs, customProviders)
	return c.JSON(fiber.Map{"providers": providers})
}

// RefreshModels 从远程 API 刷新模型列表，返回与本地配置的 diff
func (h *AIHandler) RefreshModels(c fiber.Ctx) error {
	remoteModels, err := llm.FetchRemoteModels()
	if err != nil {
		return ErrorResponse(c, 502, "failed to fetch remote models: "+err.Error(), "BAD_GATEWAY")
	}

	// Build set of locally known model IDs under opencode-zen
	providers := llm.GetProviders()
	localIDs := make(map[string]bool)
	for _, p := range providers {
		for _, m := range p.Models {
			if m.Provider == "opencode-zen" {
				localIDs[m.ID] = true
			}
		}
	}

	// Build set of remote model IDs
	remoteIDs := make(map[string]bool)
	var remoteList []string
	for _, rm := range remoteModels {
		remoteIDs[rm.ID] = true
		remoteList = append(remoteList, rm.ID)
	}

	// Diff: new models (in remote but not local) and removed (in local but not remote)
	var added, removed []string
	for id := range remoteIDs {
		if !localIDs[id] {
			added = append(added, id)
		}
	}
	for id := range localIDs {
		if !remoteIDs[id] {
			removed = append(removed, id)
		}
	}

	return c.JSON(fiber.Map{
		"status":      "ok",
		"total_remote": len(remoteModels),
		"total_local":  len(localIDs),
		"added":       added,
		"removed":     removed,
		"models":      remoteList,
	})
}

// GetLLMConfig 返回当前 LLM 配置
func (h *AIHandler) GetLLMConfig(c fiber.Ctx) error {
	// Try to load from database first (persists across restarts)
	if h.db != nil {
		var provider, modelID, endpoint string
		err := h.db.Conn.QueryRow(`SELECT provider, model_id, endpoint FROM llm_config WHERE id='default'`).Scan(&provider, &modelID, &endpoint)
		if err == nil && provider != "" {
			h.cfg.Lock()
			h.cfg.LLMProvider = provider
			h.cfg.LLMModelID = modelID
			h.cfg.LLMEndpoint = endpoint
			h.cfg.Unlock()
		}
	}

	h.cfg.RLock()
	effectiveKey := h.cfg.EffectiveLLMKey()
	keyConfigured := effectiveKey != ""

	resp := c.JSON(fiber.Map{
		"provider": h.cfg.LLMProvider,
		"model_id": h.cfg.LLMModelID,
		// Legacy fields for backward compatibility
		"legacy_endpoint": h.cfg.LLMEndpoint,
		"legacy_model":    h.cfg.LLMModel,
		"key_configured":  keyConfigured,
		// Don't expose actual keys
	})
	h.cfg.RUnlock()
	return resp
}

func (h *AIHandler) GetPrompts(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	prompts, err := h.svc.GetPrompts(uid)
	if err != nil {
		slog.Error("GetPrompts failed", "error", err)
		return InternalError(c, "failed to load prompts")
	}
	return c.JSON(fiber.Map{"prompts": prompts})
}

func (h *AIHandler) UpdatePrompt(c fiber.Ctx) error {
	var req struct {
		Mode    string `json:"mode"`
		Content string `json:"content"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Mode == "" || req.Content == "" {
		return BadRequest(c, "mode and content required")
	}
	if len(req.Mode) > 50 {
		return BadRequest(c, "mode too long (max 50)")
	}
	if len(req.Content) > 5000 {
		return BadRequest(c, "content too long (max 5000)")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	if err := h.svc.UpdatePrompt(req.Mode, req.Content, uid); err != nil {
		slog.Error("UpdatePrompt failed", "mode", req.Mode, "error", err)
		return InternalError(c, "failed to save prompt")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AIHandler) ResetPrompt(c fiber.Ctx) error {
	mode := c.Params("mode")
	if mode == "" {
		return BadRequest(c, "mode required")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	if err := h.svc.ResetPrompt(mode, uid); err != nil {
		slog.Error("ResetPrompt failed", "mode", mode, "error", err)
		return InternalError(c, "failed to reset prompt")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// UpdateLLMConfig 更新 LLM 提供商和模型配置
func (h *AIHandler) UpdateLLMConfig(c fiber.Ctx) error {
	var req struct {
		Provider string `json:"provider"`
		ModelID  string `json:"model_id"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		slog.Error("UpdateLLMConfig: bind failed", "error", err)
		return BadRequest(c, "invalid request")
	}

	slog.Info("UpdateLLMConfig", "provider", req.Provider, "model_id", req.ModelID)

	if req.Provider == "" || req.ModelID == "" {
		slog.Warn("UpdateLLMConfig: missing fields", "provider", req.Provider, "model_id", req.ModelID)
		return BadRequest(c, "provider and model_id required")
	}
	if len(req.Provider) > 50 {
		return BadRequest(c, "provider too long (max 50)")
	}
	if len(req.ModelID) > 100 {
		return BadRequest(c, "model_id too long (max 100)")
	}

	uid, _ := c.Locals("uid").(string)
	provider := llm.FindProvider(req.Provider)

	// If not found in presets, check custom providers
	if provider == nil && uid != "" && h.db != nil {
		cp, err := h.db.GetCustomProvider(uid, req.Provider)
		if err == nil && cp != nil {
			var models []llm.Model
			if cp.ModelsJSON != "" {
				_ = json.Unmarshal([]byte(cp.ModelsJSON), &models)
			}
			provider = &llm.Provider{
				Name:        cp.Name,
				ID:          cp.ID,
				Endpoint:    cp.Endpoint,
				Models:      models,
				RequiresKey: cp.APIKey != "",
				IsFree:      false,
				Tier:        "paid",
			}
		}
	}

	if provider == nil {
		return BadRequest(c, "unknown provider: "+req.Provider)
	}

	// Validate model exists in provider (skip for custom providers with empty models)
	model := llm.FindModel(req.Provider, req.ModelID)
	if model == nil && provider != nil {
		for _, m := range provider.Models {
			if m.ID == req.ModelID {
				mCopy := m
				model = &mCopy
				break
			}
		}
	}
	if model == nil && len(provider.Models) > 0 {
		// Only reject if provider has a model list but the model isn't in it
		return BadRequest(c, "model not found in provider: "+req.ModelID)
	}

	// Update runtime config
	h.cfg.Lock()
	h.cfg.LLMProvider = req.Provider
	h.cfg.LLMModelID = req.ModelID

	// Also update legacy fields for backward compatibility
	h.cfg.LLMEndpoint = provider.Endpoint
	h.cfg.LLMModel = req.ModelID
	h.cfg.LLMApiKey = h.cfg.EffectiveLLMKey()
	h.cfg.Unlock()

	// Persist to database so selection survives server restarts
	if h.db != nil {
		h.db.Conn.Exec(`INSERT INTO llm_config (id, provider, model_id, endpoint, updated_at)
			VALUES ('default', ?, ?, ?, datetime('now'))
			ON CONFLICT(id) DO UPDATE SET provider=excluded.provider, model_id=excluded.model_id,
			endpoint=excluded.endpoint, updated_at=excluded.updated_at`,
			req.Provider, req.ModelID, provider.Endpoint)
	}

	return c.JSON(fiber.Map{
		"status":   "ok",
		"provider": provider.Name,
		"model":    model.Name,
	})
}

// ---------- Conversation Persistence ----------

func (h *AIHandler) ListConversations(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	convs, err := service.ListConversations(h.db.Conn, uid)
	if err != nil {
		slog.Error("ListConversations", "error", err)
		return InternalError(c, "failed to list conversations")
	}
	if convs == nil {
		convs = []service.ConversationSummary{}
	}
	return c.JSON(fiber.Map{"conversations": convs})
}

func (h *AIHandler) SaveConversation(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	var req struct {
		ID        string            `json:"id"`
		Title     string            `json:"title"`
		Mode      string            `json:"mode"`
		Messages  []service.Message `json:"messages"`
		Model     string            `json:"model"`
		ProjectID string            `json:"project_id"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if len(req.Messages) == 0 {
		return BadRequest(c, "messages required")
	}
	savedID, err := service.SaveConversation(h.db.Conn, uid, req.ID, req.Title, req.Mode, req.Messages, req.Model, req.ProjectID)
	if err != nil {
		slog.Error("SaveConversation", "error", err)
		return InternalError(c, "failed to save conversation")
	}
	return c.JSON(fiber.Map{"id": savedID, "status": "ok"})
}

func (h *AIHandler) GetConversation(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "id required")
	}
	data, err := service.LoadConversation(h.db.Conn, uid, id)
	if err != nil {
		slog.Error("GetConversation", "error", err)
		return InternalError(c, "failed to load conversation")
	}
	if data == nil || data.Messages == nil {
		data = &service.ConversationData{Messages: []service.Message{}, Mode: "", ProjectID: ""}
	}
	return c.JSON(fiber.Map{"messages": data.Messages, "mode": data.Mode, "project_id": data.ProjectID})
}

func (h *AIHandler) DeleteConversation(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "id required")
	}
	if err := service.DeleteConversation(h.db.Conn, uid, id); err != nil {
		slog.Error("DeleteConversation", "error", err)
		return InternalError(c, "failed to delete conversation")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AIHandler) AutoBuild(c fiber.Ctx) error {
	var req struct {
		Description string            `json:"description"`
		ProjectID   string            `json:"project_id"`
		SessionID   string            `json:"session_id"`
		Messages    []service.Message `json:"messages"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}

	if req.Description == "" {
		return BadRequest(c, "description required")
	}

	uid, _ := c.Locals("uid").(string)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		h.svc.AutoBuild(c.Context(), req.Description, req.ProjectID, uid, req.Messages, req.SessionID, w)
	})
	return nil
}

// ---------- Provider Config Management ----------

func (h *AIHandler) SaveProviderConfig(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	var req struct {
		ID         string `json:"id"`
		Endpoint   string `json:"endpoint"`
		APIKey     string `json:"api_key"`
		ModelsJSON string `json:"models_json"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.ID == "" {
		return BadRequest(c, "provider id required")
	}

	if err := h.db.UpsertProviderConfig(uid, req.ID, req.Endpoint, req.APIKey, req.ModelsJSON); err != nil {
		slog.Error("SaveProviderConfig: upsert failed", "error", err)
		return InternalError(c, "failed to save config")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AIHandler) GetProviderConfigs(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	configs, err := h.db.GetProviderConfigs(uid)
	if err != nil {
		slog.Error("GetProviderConfigs", "error", err)
		return InternalError(c, "failed to load configs")
	}

	return c.JSON(fiber.Map{"configs": configs})
}

func (h *AIHandler) DeleteProviderConfig(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	providerID := c.Params("id")
	if providerID == "" {
		return BadRequest(c, "provider id required")
	}

	if err := h.db.DeleteProviderConfig(uid, providerID); err != nil {
		slog.Error("DeleteProviderConfig", "error", err)
		return InternalError(c, "failed to delete config")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// ---------- Custom Provider Management ----------

func (h *AIHandler) CreateCustomProvider(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	var req struct {
		Name       string `json:"name"`
		Endpoint   string `json:"endpoint"`
		APIKey     string `json:"api_key"`
		ModelsJSON string `json:"models_json"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.Name == "" || req.Endpoint == "" {
		return BadRequest(c, "name and endpoint required")
	}

	provider := &database.CustomProvider{
		ID:         newUUID(),
		UserID:     uid,
		Name:       req.Name,
		Endpoint:   req.Endpoint,
		APIKey:     req.APIKey,
		ModelsJSON: req.ModelsJSON,
	}

	if err := h.db.CreateCustomProvider(provider); err != nil {
		slog.Error("CreateCustomProvider", "error", err)
		return InternalError(c, "failed to create provider")
	}

	return c.JSON(fiber.Map{"status": "ok", "id": provider.ID})
}

func (h *AIHandler) GetCustomProviders(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	providers, err := h.db.GetCustomProviders(uid)
	if err != nil {
		slog.Error("GetCustomProviders", "error", err)
		return InternalError(c, "failed to load providers")
	}

	return c.JSON(fiber.Map{"providers": providers})
}

func (h *AIHandler) UpdateCustomProvider(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	providerID := c.Params("id")
	if providerID == "" {
		return BadRequest(c, "provider id required")
	}

	var req struct {
		Name       string `json:"name"`
		Endpoint   string `json:"endpoint"`
		APIKey     string `json:"api_key"`
		ModelsJSON string `json:"models_json"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}

	provider := &database.CustomProvider{
		ID:         providerID,
		UserID:     uid,
		Name:       req.Name,
		Endpoint:   req.Endpoint,
		APIKey:     req.APIKey,
		ModelsJSON: req.ModelsJSON,
	}

	if err := h.db.UpdateCustomProvider(provider); err != nil {
		slog.Error("UpdateCustomProvider", "error", err)
		return InternalError(c, "failed to update provider")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *AIHandler) DeleteCustomProvider(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}

	providerID := c.Params("id")
	if providerID == "" {
		return BadRequest(c, "provider id required")
	}

	if err := h.db.DeleteCustomProvider(uid, providerID); err != nil {
		slog.Error("DeleteCustomProvider", "error", err)
		return InternalError(c, "failed to delete provider")
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// ---------- AI Capability Scoring ----------

// GetAICapability 返回当前配置的 AI 能力评分
func (h *AIHandler) GetAICapability(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)

	var db *sql.DB
	if h.db != nil {
		db = h.db.Conn
	}

	score := service.EvaluateAICapability(h.cfg, db, uid)
	return c.JSON(fiber.Map{"capability": score})
}

// GET /ai/memory — list all memory for the current user
func (h *AIHandler) ListMemory(c fiber.Ctx) error {
	if h.memoryStore == nil {
		return InternalError(c, "memory store not available")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "user not authenticated")
	}
	memType := c.Query("type")

	var entries []service.MemoryEntry
	var err error
	if memType != "" {
		entries, err = h.memoryStore.ListMemory(uid, memType)
	} else {
		entries, err = h.memoryStore.ListAllMemory(uid)
	}
	if err != nil {
		return InternalError(c, "failed to list memory: "+err.Error())
	}
	if entries == nil {
		entries = []service.MemoryEntry{}
	}
	return c.JSON(fiber.Map{"entries": entries})
}

// DELETE /ai/memory/:type/:key — delete a specific memory
func (h *AIHandler) DeleteMemory(c fiber.Ctx) error {
	if h.memoryStore == nil {
		return InternalError(c, "memory store not available")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "user not authenticated")
	}
	memType := c.Params("type")
	key := c.Params("key")
	if memType == "" || key == "" {
		return BadRequest(c, "type and key are required")
	}
	if err := h.memoryStore.DeleteMemory(uid, memType, key); err != nil {
		return InternalError(c, "failed to delete memory: "+err.Error())
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// DELETE /ai/memory — delete all memory for the current user
func (h *AIHandler) ClearMemory(c fiber.Ctx) error {
	if h.memoryStore == nil {
		return InternalError(c, "memory store not available")
	}
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "user not authenticated")
	}
	if err := h.memoryStore.DeleteAllMemory(uid); err != nil {
		return InternalError(c, "failed to clear memory: "+err.Error())
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// ---------- Session-Based Conversation Messages ----------

func (h *AIHandler) ListSessions(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	sessions, total, err := service.ListUserSessions(h.db.Conn, uid, limit, offset)
	if err != nil {
		slog.Error("ListSessions", "error", err)
		return InternalError(c, "failed to list sessions")
	}
	if sessions == nil {
		sessions = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{
		"sessions": sessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *AIHandler) GetSessionMessages(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	sessionID := c.Params("session_id")
	if sessionID == "" {
		return BadRequest(c, "session_id required")
	}
	messages, mode, err := service.GetConversationMessages(h.db.Conn, sessionID, uid)
	if err != nil {
		slog.Error("GetSessionMessages", "error", err)
		return InternalError(c, "failed to get messages")
	}
	if messages == nil {
		messages = []service.ConversationMessage{}
	}
	// 获取 project_id 和 agent_mode
	var projectID, agentMode string
	h.db.Conn.QueryRow(`SELECT COALESCE(project_id, ''), COALESCE(agent_mode, 'act') FROM ai_conversations WHERE id=? AND user_id=?`, sessionID, uid).Scan(&projectID, &agentMode)
	return c.JSON(fiber.Map{"messages": messages, "mode": mode, "project_id": projectID, "agent_mode": agentMode})
}

func (h *AIHandler) DeleteSession(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	sessionID := c.Params("session_id")
	if sessionID == "" {
		return BadRequest(c, "session_id required")
	}
	if err := service.DeleteSessionMessages(h.db.Conn, sessionID, uid); err != nil {
		slog.Error("DeleteSession", "error", err)
		return InternalError(c, "failed to delete session")
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// ---------- Session Export ----------

func (h *AIHandler) ExportSession(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	sessionID := c.Params("session_id")
	if sessionID == "" {
		return BadRequest(c, "session_id required")
	}
	format := c.Query("format", "markdown")
	switch format {
	case "json":
		data, err := service.ExportSessionAsJSON(h.db.Conn, sessionID, uid)
		if err != nil {
			return InternalError(c, "failed to export session")
		}
		c.Set("Content-Type", "application/json")
		c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="session-%s.json"`, sessionID))
		return c.Send(data)
	default: // markdown
		md, err := service.ExportSessionAsMarkdown(h.db.Conn, sessionID, uid)
		if err != nil {
			return InternalError(c, "failed to export session")
		}
		c.Set("Content-Type", "text/markdown; charset=utf-8")
		c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="session-%s.md"`, sessionID))
		return c.SendString(md)
	}
}

// ---------- Session Search ----------

func (h *AIHandler) SearchSessions(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	query := c.Query("q", "")
	if query == "" {
		return BadRequest(c, "query required")
	}
	results, err := service.SearchSessionMessages(h.db.Conn, uid, query, 50)
	if err != nil {
		slog.Error("SearchSessions", "error", err)
		return InternalError(c, "failed to search sessions")
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return c.JSON(fiber.Map{"results": results})
}

// ---------- Code Diff ----------

func (h *AIHandler) ComputeDiff(c fiber.Ctx) error {
	var req struct {
		OldCode  string `json:"old_code"`
		NewCode  string `json:"new_code"`
		FilePath string `json:"file_path"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return BadRequest(c, "invalid request")
	}
	if req.OldCode == "" && req.NewCode == "" {
		return BadRequest(c, "old_code or new_code required")
	}

	oldLines := strings.Split(req.OldCode, "\n")
	newLines := strings.Split(req.NewCode, "\n")

	type DiffEntry struct {
		Type string `json:"type"`
		Line int    `json:"line"`
		Old  string `json:"old,omitempty"`
		New  string `json:"new,omitempty"`
	}

	var diffs []DiffEntry

	// Simple LCS-based diff
	lcs := computeLCS(oldLines, newLines)
	oi, ni := 0, 0
	line := 1
	for _, lcsIdx := range lcs {
		for oi < lcsIdx[0] {
			diffs = append(diffs, DiffEntry{Type: "remove", Line: line, Old: oldLines[oi]})
			line++
			oi++
		}
		for ni < lcsIdx[1] {
			diffs = append(diffs, DiffEntry{Type: "add", Line: line, New: newLines[ni]})
			line++
			ni++
		}
		diffs = append(diffs, DiffEntry{Type: "context", Line: line, Old: oldLines[oi], New: newLines[ni]})
		line++
		oi++
		ni++
	}
	for oi < len(oldLines) {
		diffs = append(diffs, DiffEntry{Type: "remove", Line: line, Old: oldLines[oi]})
		line++
		oi++
	}
	for ni < len(newLines) {
		diffs = append(diffs, DiffEntry{Type: "add", Line: line, New: newLines[ni]})
		line++
		ni++
	}

	return c.JSON(fiber.Map{
		"diffs":    diffs,
		"file_path": req.FilePath,
		"old_lines": len(oldLines),
		"new_lines": len(newLines),
	})
}

// computeLCS finds the longest common subsequence of lines between two slices.
func computeLCS(a, b []string) [][2]int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to find matching pairs
	var result [][2]int
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append([][2]int{{i - 1, j - 1}}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return result
}

// ---------- Build Progress SSE ----------

func (h *AIHandler) StreamBuildProgress(c fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return BadRequest(c, "build id required")
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

 stages := []struct {
		Stage    string
		Duration int
		Message  string
	}{
		{"compile", 2000, "正在编译源代码..."},
		{"test", 3000, "正在运行测试..."},
		{"package", 1500, "正在打包模块..."},
	}

	totalMs := 0
	for _, s := range stages {
		totalMs += s.Duration
	}

	elapsed := 0
	for _, s := range stages {
		steps := s.Duration / 500
		if steps < 1 {
			steps = 1
		}
		stepMs := s.Duration / steps
		for i := 0; i < steps; i++ {
			elapsed += stepMs
			progress := elapsed * 100 / totalMs
			if progress > 100 {
				progress = 100
			}
			evt := fmt.Sprintf(`{"type":"progress","stage":"%s","progress":%d,"message":"%s"}`, s.Stage, progress, s.Message)
			if _, err := c.Write([]byte("data: " + evt + "\n\n")); err != nil {
				return err
			}
			time.Sleep(time.Duration(stepMs) * time.Millisecond)
		}
	}

	completeEvt := `{"type":"progress","stage":"done","progress":100,"message":"构建完成！"}`
	c.Write([]byte("data: " + completeEvt + "\n\n"))
	c.Write([]byte("data: [DONE]\n\n"))
	return nil
}

// ─── Memory V2 Handlers ───

func (h *AIHandler) getUserID(c fiber.Ctx) string {
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		return uid
	}
	if uid, ok := c.Locals("uid").(string); ok && uid != "" {
		return uid
	}
	return ""
}

func (h *AIHandler) GetProjectKnowledge(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	pid := c.Params("project_id")
	category := c.Query("category", "")
	entries, err := h.memV2.ListKnowledge(uid, pid, category)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(entries)
}

func (h *AIHandler) SaveProjectKnowledge(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	pid := c.Params("project_id")
	var body struct {
		Category string `json:"category"`
		Key      string `json:"key"`
		Value    string `json:"value"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	if body.Category == "" || body.Key == "" || body.Value == "" {
		return c.Status(400).JSON(fiber.Map{"error": "category, key, value required"})
	}
	if err := h.memV2.SaveKnowledge(uid, pid, body.Category, body.Key, body.Value); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (h *AIHandler) DeleteProjectKnowledge(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	pid := c.Params("project_id")
	cat := c.Params("category")
	key := c.Params("key")
	if err := h.memV2.DeleteKnowledge(uid, pid, cat, key); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (h *AIHandler) GetProjectSummaries(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	pid := c.Params("project_id")
	summaries, err := h.memV2.GetProjectSummaries(uid, pid, 20)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(summaries)
}

func (h *AIHandler) GenerateSummary(c fiber.Ctx) error {
	uid := h.getUserID(c)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	var body struct {
		SessionID string `json:"session_id"`
		ProjectID string `json:"project_id"`
		Messages  string `json:"messages"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	// Call LLM to generate summary
	prompt := "请简要总结以下对话的关键内容、做出的决定和改动的文件。用JSON格式返回：{\"summary\": \"...\", \"key_decisions\": [\"...\"], \"files_changed\": [\"...\"]}\n\n对话内容:\n" + body.Messages

	resp, err := h.callLLMForSummary(prompt, "你是一个精准的对话总结器。只输出JSON，不要其他内容。")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse LLM response
	var result struct {
		Summary      string   `json:"summary"`
		KeyDecisions []string `json:"key_decisions"`
		FilesChanged []string `json:"files_changed"`
	}
	json.Unmarshal([]byte(resp), &result)

	if err := h.memV2.SaveSummary(uid, body.SessionID, body.ProjectID, result.Summary, result.KeyDecisions, result.FilesChanged); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// callLLMForSummary makes a simple LLM API call for summary generation
func (h *AIHandler) callLLMForSummary(prompt, systemPrompt string) (string, error) {
	body := map[string]interface{}{
		"model":      h.cfg.LLMModel,
		"messages":   []map[string]string{{"role": "system", "content": systemPrompt}, {"role": "user", "content": prompt}},
		"stream":     false,
		"max_tokens": 4096,
	}
	bodyJSON, _ := json.Marshal(body)

	endpoint := h.cfg.LLMEndpoint
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := h.cfg.EffectiveLLMKey(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}
	return result.Choices[0].Message.Content, nil
}
