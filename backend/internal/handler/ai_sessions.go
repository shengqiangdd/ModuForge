package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

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
	// 智能标题：大部分模式 auto-save 不传 title；用轻量 LLM 从首条消息
	// 生成短标题（失败自动 fallback 到 service 内的默认截断逻辑）。
	useTitle := strings.TrimSpace(req.Title)
	isDefault := useTitle == "" || strings.HasSuffix(useTitle, "...") || useTitle == req.Mode
	if isDefault && h.svc != nil {
		if suggested := h.svc.SuggestTitle(c.Context(), uid, req.Messages); suggested != "" {
			useTitle = suggested
		}
	}
	savedID, err := service.SaveConversation(h.db.Conn, uid, req.ID, useTitle, req.Mode, req.Messages, req.Model, req.ProjectID)
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
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	before := c.Query("before", "")
	beforeID := c.Query("before_id", "")
	var messages []service.ConversationMessage
	var hasMore bool
	var mode string
	var err error
	if limit > 0 {
		messages, hasMore, mode, err = service.GetConversationMessagesPage(h.db.Conn, sessionID, uid, limit, before, beforeID)
	} else {
		messages, mode, err = service.GetConversationMessages(h.db.Conn, sessionID, uid)
	}
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
	return c.JSON(fiber.Map{"messages": messages, "mode": mode, "project_id": projectID, "agent_mode": agentMode, "has_more": hasMore, "limit": limit})
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

func (h *AIHandler) RenameSession(c fiber.Ctx) error {
	uid, _ := c.Locals("uid").(string)
	if uid == "" {
		return Unauthorized(c, "authentication required")
	}
	sessionID := c.Params("session_id")
	var req struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return BadRequest(c, "title is required")
	}
	if len([]rune(req.Title)) > 100 {
		return BadRequest(c, "title too long (max 100 chars)")
	}
	if err := service.RenameSession(h.db.Conn, sessionID, uid, req.Title); err != nil {
		slog.Error("RenameSession", "error", err)
		return InternalError(c, "failed to rename session")
	}
	return c.JSON(fiber.Map{"ok": true})
}

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
