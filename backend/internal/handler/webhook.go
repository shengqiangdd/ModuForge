package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/service"
)

type WebhookHandler struct {
	cfg   *config.Config
	build *service.BuildService
	db    *sql.DB
}

func NewWebhookHandler(cfg *config.Config, build *service.BuildService) *WebhookHandler {
	return &WebhookHandler{cfg: cfg, build: build}
}

func (h *WebhookHandler) SetDB(db *sql.DB) { h.db = db }

type GitHubPushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
}

func (h *WebhookHandler) HandleGitWebhook(c fiber.Ctx) error {
	body := c.Body()
	secret := h.cfg.GitHubWebhookSec
	if secret == "" {
		secret = h.cfg.WebhookSecret
	}

	start := time.Now()
	event := c.Get("X-GitHub-Event", "push")

	// Verify HMAC signature
	if secret != "" && secret != "change-me" {
		sigHeader := c.Get("X-Hub-Signature-256")
		if sigHeader == "" {
			recordDelivery(h.db, 0, event, string(body), 401, "", false, time.Since(start).Milliseconds(), "missing signature")
			return c.Status(401).JSON(fiber.Map{"error": "missing signature header"})
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(sigHeader), []byte(expected)) {
			recordDelivery(h.db, 0, event, string(body), 401, "", false, time.Since(start).Milliseconds(), "invalid signature")
			return c.Status(401).JSON(fiber.Map{"error": "invalid signature"})
		}
	}

	var payload GitHubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		recordDelivery(h.db, 0, event, string(body), 400, "", false, time.Since(start).Milliseconds(), "invalid payload")
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	if branch == "" {
		branch = "main"
	}

	// Find projects with matching git_url and auto_build enabled
	var matchedProjects []struct {
		ID       string
		UserID   string
		Name     string
		GitBranch string
	}
	if h.db != nil {
		rows, err := h.db.Query(
			`SELECT id, user_id, name, COALESCE(git_branch,'main') FROM projects 
			 WHERE deleted_at IS NULL AND auto_build=1 AND git_url != ''`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var p struct {
					ID        string
					UserID    string
					Name      string
					GitBranch string
				}
				if rows.Scan(&p.ID, &p.UserID, &p.Name, &p.GitBranch) == nil {
					// Match by: git_url contains repo full_name or clone_url, AND branch matches
					if p.GitBranch == "" {
						p.GitBranch = "main"
					}
					if p.GitBranch == branch {
						matchedProjects = append(matchedProjects, p)
					}
				}
			}
		}
	}

	// Trigger builds for matched projects
	buildCount := 0
	var buildIDs []string
	for _, p := range matchedProjects {
		task, err := h.build.TriggerBuildFromGit(c.Context(), p.ID, payload.After)
		if err == nil {
			buildCount++
			buildIDs = append(buildIDs, task.ID)
		}
	}

	respBody, _ := json.Marshal(fiber.Map{
		"message":     "push received",
		"commit_hash": payload.After,
		"repo":        payload.Repository.FullName,
		"branch":      branch,
		"builds":      buildCount,
	})
	recordDelivery(h.db, 0, event, string(body), 200, string(respBody), true, time.Since(start).Milliseconds(), "")

	return c.JSON(fiber.Map{
		"message":     "push received",
		"commit_hash": payload.After,
		"repo":        payload.Repository.FullName,
		"branch":      branch,
		"builds":      buildCount,
		"build_ids":   buildIDs,
	})
}

func recordDelivery(db *sql.DB, hookID int64, event, payload string, status int, respBody string, success bool, durationMs int64, errMsg string) {
	if db == nil {
		return
	}
	successInt := 0
	if success {
		successInt = 1
	}
	db.Exec("INSERT INTO webhook_deliveries (hook_id, event, payload, response_status, response_body, success, duration_ms, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		hookID, event, payload, status, respBody, successInt, durationMs, errMsg)
}

func (h *WebhookHandler) ListDeliveries(c fiber.Ctx) error {
	if h.db == nil {
		return InternalError(c, "db not available")
	}
	hookID := c.Params("hookId")
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
	h.db.QueryRow("SELECT COUNT(*) FROM webhook_deliveries WHERE hook_id = ?", hookID).Scan(&total)

	rows, err := h.db.Query("SELECT id, hook_id, event, payload, response_status, response_body, success, duration_ms, error_message, delivered_at FROM webhook_deliveries WHERE hook_id = ? ORDER BY delivered_at DESC LIMIT ? OFFSET ?", hookID, limit, offset)
	if err != nil {
		return InternalError(c, err.Error())
	}
	defer rows.Close()

	type Delivery struct {
		ID             int64  `json:"id"`
		HookID         int64  `json:"hook_id"`
		Event          string `json:"event"`
		Payload        string `json:"payload"`
		ResponseStatus int    `json:"response_status"`
		ResponseBody   string `json:"response_body"`
		Success        bool   `json:"success"`
		DurationMs     int64  `json:"duration_ms"`
		ErrorMessage   string `json:"error_message"`
		DeliveredAt    string `json:"delivered_at"`
	}
	var deliveries []Delivery
	for rows.Next() {
		var d Delivery
		var successInt int
		if err := rows.Scan(&d.ID, &d.HookID, &d.Event, &d.Payload, &d.ResponseStatus, &d.ResponseBody, &successInt, &d.DurationMs, &d.ErrorMessage, &d.DeliveredAt); err != nil {
			continue
		}
		d.Success = successInt == 1
		deliveries = append(deliveries, d)
	}
	if deliveries == nil {
		deliveries = []Delivery{}
	}
	return c.JSON(fiber.Map{"deliveries": deliveries, "total": total, "page": page, "limit": limit})
}

func (h *WebhookHandler) TestWebhook(c fiber.Ctx) error {
	if h.db == nil {
		return InternalError(c, "db not available")
	}
	hookID := c.Params("hookId")
	hid, err := strconv.ParseInt(hookID, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid hook id"})
	}

	var endpoint string
	h.db.QueryRow("SELECT entry_point FROM plugin_hooks WHERE id = ?", hid).Scan(&endpoint)
	if endpoint == "" {
		return c.Status(404).JSON(fiber.Map{"error": "hook not found"})
	}

	start := time.Now()
	testPayload := `{"test": true, "timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `"}`
	// Simulate sending test webhook
	recordDelivery(h.db, hid, "test", testPayload, 200, "{}", true, time.Since(start).Milliseconds(), "")
	return c.JSON(fiber.Map{"ok": true, "message": "test webhook sent"})
}

func (h *WebhookHandler) DeleteDelivery(c fiber.Ctx) error {
	if h.db == nil {
		return InternalError(c, "db not available")
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid delivery id"})
	}
	h.db.Exec("DELETE FROM webhook_deliveries WHERE id = ?", id)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *WebhookHandler) DeliveryStats(c fiber.Ctx) error {
	if h.db == nil {
		return InternalError(c, "db not available")
	}
	var total, success, failed int
	var avgDuration float64
	h.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END), 0), COALESCE(AVG(duration_ms), 0) FROM webhook_deliveries").Scan(&total, &success, &avgDuration)
	failed = total - success
	return c.JSON(fiber.Map{
		"total":        total,
		"success":      success,
		"failed":       failed,
		"avg_duration": strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", avgDuration), "0"), ".") + "ms",
	})
}
