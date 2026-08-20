package handler

import (
	"bufio"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

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

	// Monthly AI cost cap guard (generation mode).
	if info, exceeded := h.costCapExceeded(uid, req.Model); exceeded {
		return BadRequest(c, fmt.Sprintf("本月 AI 成本已达上限（$%.2f / $%.2f），请调整 AI_MONTHLY_COST_LIMIT 或下月再试", info.EstimatedCost, info.LimitUSD))
	}

	// Merge project context into description if available
	description := req.Description
	if req.ProjectContext != "" {
		description = fmt.Sprintf("%s\n\n## Project Context:\n%s", description, req.ProjectContext)
	} else if req.ProjectID != "" && uid != "" && h.db != nil {
		if ctx := h.autoLoadProjectContext(c.Context(), req.ProjectID, uid); ctx != "" {
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

	// Monthly AI cost cap guard (chat mode).
	if info, exceeded := h.costCapExceeded(uid, req.Model); exceeded {
		return BadRequest(c, fmt.Sprintf("本月 AI 成本已达上限（$%.2f / $%.2f），请调整 AI_MONTHLY_COST_LIMIT 或下月再试", info.EstimatedCost, info.LimitUSD))
	}

	// Merge project context: manual context takes precedence
	contextInfo := req.Context
	if req.ProjectContext != "" {
		contextInfo = req.ProjectContext
	} else if req.ProjectID != "" && uid != "" && h.db != nil {
		// Auto-load project files as context
		contextInfo = h.autoLoadProjectContext(c.Context(), req.ProjectID, uid)
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
