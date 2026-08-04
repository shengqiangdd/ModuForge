package handler

import (
	"database/sql"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

type FileCommentHandler struct {
	db *sql.DB
}

func NewFileCommentHandler(db *sql.DB) *FileCommentHandler {
	return &FileCommentHandler{db: db}
}

type FileComment struct {
	ID         int64  `json:"id"`
	ProjectID  string `json:"project_id"`
	FilePath   string `json:"file_path"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	LineNumber int    `json:"line_number"`
	Content    string `json:"content"`
	ParentID   int64  `json:"parent_id"`
	Resolved   bool   `json:"resolved"`
	Replies    []FileComment `json:"replies,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// POST /projects/:id/files/*/comments
func (h *FileCommentHandler) AddComment(c fiber.Ctx) error {
	projectID := c.Params("id")
	filePath := c.Params("*")

	var req struct {
		LineNumber int    `json:"line_number"`
		Content    string `json:"content"`
		ParentID   int64  `json:"parent_id"`
		Username   string `json:"username"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Content == "" {
		return c.Status(400).JSON(fiber.Map{"error": "content is required"})
	}

	userID := safeUserID(c)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}

	username := req.Username
	if username == "" {
		username = userID
	}

	result, err := h.db.Exec(
		`INSERT INTO file_comments (project_id, file_path, user_id, username, line_number, content, parent_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID, filePath, userID, username, req.LineNumber, req.Content, req.ParentID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	id, _ := result.LastInsertId()
	return c.Status(201).JSON(fiber.Map{
		"id":          id,
		"project_id":  projectID,
		"file_path":   filePath,
		"user_id":     userID,
		"username":    username,
		"line_number": req.LineNumber,
		"content":     req.Content,
		"parent_id":   req.ParentID,
	})
}

// GET /projects/:id/files/*/comments
func (h *FileCommentHandler) GetComments(c fiber.Ctx) error {
	projectID := c.Params("id")
	filePath := c.Params("*")

	rows, err := h.db.Query(
		`SELECT id, project_id, file_path, user_id, username, line_number, content, parent_id, resolved, created_at
		 FROM file_comments WHERE project_id = ? AND file_path = ?
		 ORDER BY line_number ASC, created_at ASC`,
		projectID, filePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var allComments []FileComment
	commentMap := make(map[int64]*FileComment)

	for rows.Next() {
		var fc FileComment
		if err := rows.Scan(&fc.ID, &fc.ProjectID, &fc.FilePath, &fc.UserID, &fc.Username,
			&fc.LineNumber, &fc.Content, &fc.ParentID, &fc.Resolved, &fc.CreatedAt); err != nil {
			continue
		}
		fc.Replies = []FileComment{}
		commentMap[fc.ID] = &fc
		if fc.ParentID == 0 {
			allComments = append(allComments, fc)
		}
	}

	// Build reply tree
	for i := range allComments {
		if replies, ok := commentMap[allComments[i].ID]; ok {
			_ = replies // placeholder
		}
	}

	// Attach replies to parent comments
	for _, fc := range allComments {
		if fc.ParentID != 0 {
			if parent, ok := commentMap[fc.ParentID]; ok {
				parent.Replies = append(parent.Replies, fc)
			}
		}
	}

	if allComments == nil {
		allComments = []FileComment{}
	}

	return c.JSON(fiber.Map{"comments": allComments})
}

// DELETE /projects/:id/comments/:comment_id
func (h *FileCommentHandler) DeleteComment(c fiber.Ctx) error {
	commentID := c.Params("comment_id")
	userID := safeUserID(c)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}

	id, err := strconv.ParseInt(commentID, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid comment id"})
	}

	// Only allow deleting own comments (or admin check could be added)
	result, err := h.db.Exec(
		`DELETE FROM file_comments WHERE id = ? AND user_id = ?`,
		id, userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "comment not found or not owned by you"})
	}

	return c.JSON(fiber.Map{"ok": true})
}

// POST /projects/:id/comments/:comment_id/reply
func (h *FileCommentHandler) ReplyToComment(c fiber.Ctx) error {
	commentID := c.Params("comment_id")
	projectID := c.Params("id")

	var req struct {
		Content  string `json:"content"`
		Username string `json:"username"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Content == "" {
		return c.Status(400).JSON(fiber.Map{"error": "content is required"})
	}

	userID := safeUserID(c)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
	}

	parentID, err := strconv.ParseInt(commentID, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid comment id"})
	}

	// Get parent's file_path and line_number
	var filePath string
	var lineNumber int
	err = h.db.QueryRow(
		`SELECT file_path, line_number FROM file_comments WHERE id = ?`, parentID).Scan(&filePath, &lineNumber)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "parent comment not found"})
	}

	username := req.Username
	if username == "" {
		username = userID
	}

	result, err := h.db.Exec(
		`INSERT INTO file_comments (project_id, file_path, user_id, username, line_number, content, parent_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID, filePath, userID, username, lineNumber, req.Content, parentID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	id, _ := result.LastInsertId()
	return c.Status(201).JSON(fiber.Map{
		"id":         id,
		"parent_id":  parentID,
		"content":    req.Content,
		"user_id":    userID,
		"username":   username,
	})
}

// GET /projects/:id/comments — list all comments for a project (used by project overview)
func (h *FileCommentHandler) ListProjectComments(c fiber.Ctx) error {
	projectID := c.Params("id")

	rows, err := h.db.Query(
		`SELECT id, project_id, file_path, user_id, username, line_number, content, parent_id, resolved, created_at
		 FROM file_comments WHERE project_id = ?
		 ORDER BY created_at DESC LIMIT 50`,
		projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var comments []FileComment
	for rows.Next() {
		var fc FileComment
		if err := rows.Scan(&fc.ID, &fc.ProjectID, &fc.FilePath, &fc.UserID, &fc.Username,
			&fc.LineNumber, &fc.Content, &fc.ParentID, &fc.Resolved, &fc.CreatedAt); err != nil {
			continue
		}
		comments = append(comments, fc)
	}

	if comments == nil {
		comments = []FileComment{}
	}

	return c.JSON(fiber.Map{"comments": comments})
}
