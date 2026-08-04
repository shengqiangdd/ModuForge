package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type CollaborationHandler struct {
	collab      *service.CollaborationService
	notifSvc    *service.NotificationService
	activitySvc *service.ActivityService
}

func NewCollaborationHandler(collab *service.CollaborationService) *CollaborationHandler {
	return &CollaborationHandler{collab: collab}
}

func (h *CollaborationHandler) SetNotifSvc(s *service.NotificationService)   { h.notifSvc = s }
func (h *CollaborationHandler) SetActivitySvc(s *service.ActivityService) { h.activitySvc = s }

func (h *CollaborationHandler) AddCollaborator(c fiber.Ctx) error {
	projectID := c.Params("id")
	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Role == "" {
		req.Role = "editor"
	}

	collab, err := h.collab.AddCollaborator(c.Context(), projectID, req.UserID, req.Role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(collab)
}

func (h *CollaborationHandler) ListCollaborators(c fiber.Ctx) error {
	projectID := c.Params("id")
	list, err := h.collab.ListCollaborators(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"collaborators": list})
}

func (h *CollaborationHandler) RemoveCollaborator(c fiber.Ctx) error {
	projectID := c.Params("id")
	userID := c.Params("userId")
	if err := h.collab.RemoveCollaborator(c.Context(), projectID, userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *CollaborationHandler) AddComment(c fiber.Ctx) error {
	projectID := c.Params("id")
	var req struct {
		FilePath   string `json:"file_path"`
		Content    string `json:"content"`
		LineNumber int    `json:"line_number"`
		UserID     string `json:"user_id"`
		Username   string `json:"username"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	comment, err := h.collab.AddComment(c.Context(), projectID, req.UserID, req.Username, req.FilePath, req.Content, req.LineNumber)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	service.NotifyProject(projectID, "comment_added", fiber.Map{
		"comment": comment, "project_id": projectID,
	})

	return c.JSON(comment)
}

func (h *CollaborationHandler) ListComments(c fiber.Ctx) error {
	projectID := c.Params("id")
	filePath := c.Query("file_path")
	list, err := h.collab.ListComments(c.Context(), projectID, filePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"comments": list})
}

func (h *CollaborationHandler) ResolveComment(c fiber.Ctx) error {
	commentID := c.Params("commentId")
	if err := h.collab.ResolveComment(c.Context(), commentID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *CollaborationHandler) UpsertEditSession(c fiber.Ctx) error {
	projectID := c.Params("id")
	var session service.EditSession
	if err := c.Bind().JSON(&session); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	session.ProjectID = projectID

	if err := h.collab.UpsertEditSession(c.Context(), &session); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(session)
}

func (h *CollaborationHandler) ListEditSessions(c fiber.Ctx) error {
	projectID := c.Params("id")
	sessions, err := h.collab.ListEditSessions(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"sessions": sessions})
}

func (h *CollaborationHandler) RemoveEditSession(c fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	if err := h.collab.RemoveEditSession(c.Context(), sessionID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ===== Team Members =====

func (h *CollaborationHandler) AddTeamMember(c fiber.Ctx) error {
	projectID := c.Params("id")
	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Role == "" {
		req.Role = "member"
	}
	// Get current user from JWT context
	actor := safeUserID(c)
	if actor == "" {
		actor = "unknown"
	}
	member, err := h.collab.AddTeamMember(c.Context(), projectID, req.UserID, req.Role, actor)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	h.collab.LogAudit(c.Context(), projectID, actor, "team_member_added", "Added user "+req.UserID+" as "+req.Role)

	if h.notifSvc != nil && req.UserID != "" {
		h.notifSvc.Create(req.UserID, "team_invite", "项目邀请", "你被邀请加入项目 "+projectID, "/projects/"+projectID)
	}
	if h.activitySvc != nil {
		pid, _ := parseInt64(projectID)
		h.activitySvc.Log(actor, pid, "member_added", "添加了成员 "+req.UserID)
	}

	service.NotifyProject(projectID, "member_joined", fiber.Map{
		"project_id": projectID, "user_id": req.UserID, "role": req.Role,
	})

	return c.JSON(member)
}

func (h *CollaborationHandler) ListTeamMembers(c fiber.Ctx) error {
	projectID := c.Params("id")
	list, err := h.collab.GetTeamMembers(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"members": list})
}

func (h *CollaborationHandler) UpdateMemberRole(c fiber.Ctx) error {
	projectID := c.Params("id")
	userID := c.Params("userId")
	var req struct {
		Role string `json:"role"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := h.collab.UpdateMemberRole(c.Context(), projectID, userID, req.Role); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	actor := safeUserID(c)
	h.collab.LogAudit(c.Context(), projectID, actor, "team_member_role_changed", "Changed user "+userID+" role to "+req.Role)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *CollaborationHandler) RemoveTeamMember(c fiber.Ctx) error {
	projectID := c.Params("id")
	userID := c.Params("userId")
	if err := h.collab.RemoveTeamMember(c.Context(), projectID, userID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	actor := safeUserID(c)
	h.collab.LogAudit(c.Context(), projectID, actor, "team_member_removed", "Removed user "+userID)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *CollaborationHandler) GetAuditLogs(c fiber.Ctx) error {
	projectID := c.Params("id")
	userID := c.Query("user_id")
	action := c.Query("action")
	start := c.Query("start")
	end := c.Query("end")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs WHERE project_id = ?"
	queryArgs := []interface{}{projectID}

	if userID != "" {
		countQuery += " AND user_id = ?"
		queryArgs = append(queryArgs, userID)
	}
	if action != "" {
		countQuery += " AND action = ?"
		queryArgs = append(queryArgs, action)
	}
	if start != "" {
		countQuery += " AND created_at >= ?"
		queryArgs = append(queryArgs, start)
	}
	if end != "" {
		countQuery += " AND created_at <= ?"
		queryArgs = append(queryArgs, end)
	}
	allowedActions := map[string]bool{
		"create": true, "update": true, "delete": true,
		"comment": true, "build": true, "deploy": true,
		"invite": true, "remove": true,
	}
	if action != "" && !allowedActions[action] {
		action = ""
	}

	tx := h.collab.GetDB()
	row := tx.QueryRow(countQuery, queryArgs...)
	row.Scan(&total)

	dataQuery := "SELECT id, project_id, user_id, action, details, created_at FROM audit_logs WHERE project_id = ?"
	dataArgs := []interface{}{projectID}
	if userID != "" {
		dataQuery += " AND user_id = ?"
		dataArgs = append(dataArgs, userID)
	}
	if action != "" {
		dataQuery += " AND action = ?"
		dataArgs = append(dataArgs, action)
	}
	if start != "" {
		dataQuery += " AND created_at >= ?"
		dataArgs = append(dataArgs, start)
	}
	if end != "" {
		dataQuery += " AND created_at <= ?"
		dataArgs = append(dataArgs, end)
	}
	dataQuery += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	dataArgs = append(dataArgs, limit, offset)

	rows, err := tx.Query(dataQuery, dataArgs...)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()

	type AuditLog struct {
		ID        int64  `json:"id"`
		ProjectID string `json:"project_id"`
		UserID    string `json:"user_id"`
		Action    string `json:"action"`
		Details   string `json:"details"`
		CreatedAt string `json:"created_at"`
	}
	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.UserID, &l.Action, &l.Details, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []AuditLog{}
	}

	return c.JSON(fiber.Map{"logs": logs, "total": total, "page": page, "limit": limit})
}
