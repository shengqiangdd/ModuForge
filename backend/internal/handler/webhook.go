package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/logger"
	"github.com/moduforge/backend/internal/service"
)

// WebhookConfig holds configuration for CI/CD webhooks.
type WebhookConfig struct {
	Secret        string
	AutoBuild     bool
	DefaultArch   string
	DefaultTarget string
}

// WebhookHandler processes push events from GitHub and GitLab.
type WebhookHandler struct {
	cfg      *WebhookConfig
	buildSvc *service.BuildService
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(cfg *WebhookConfig, buildSvc *service.BuildService) *WebhookHandler {
	return &WebhookHandler{cfg: cfg, buildSvc: buildSvc}
}

// HandleGitHub processes GitHub push webhooks with HMAC-SHA256 verification.
func (h *WebhookHandler) HandleGitHub(c fiber.Ctx) error {
	body := c.Body()
	if len(body) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}

	repoName := "-"
	var payload GitHubPushEvent
	json.Unmarshal(body, &payload) // best-effort parse for logging
	if payload.Repository.Name != "" {
		repoName = payload.Repository.Name
	}
	logger.Info("Webhook received: type=%s repo=%s", "github", repoName)

	secret := h.cfg.Secret
	if secret == "" {
		return c.Status(401).JSON(fiber.Map{"error": "webhook secret not configured"})
	}

	// Verify HMAC-SHA256 signature
	sigHeader := c.Get("X-Hub-Signature-256")
	if sigHeader == "" {
		return c.Status(401).JSON(fiber.Map{"error": "missing X-Hub-Signature-256 header"})
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sigHeader), []byte(expected)) {
		return c.Status(403).JSON(fiber.Map{"error": "invalid signature"})
	}

	// Parse GitHub push payload
	var pushPayload GitHubPushEvent
	if err := json.Unmarshal(body, &pushPayload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON payload"})
	}

	// Extract branch from ref
	branch := strings.TrimPrefix(pushPayload.Ref, "refs/heads/")
	if branch == "" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid ref format"})
	}

	// Only process master/main branches
	switch branch {
	case "master", "main":
		// OK
	default:
		return c.JSON(fiber.Map{
			"status":  "ignored",
			"reason":  fmt.Sprintf("unsupported branch: %s", branch),
			"repo":    pushPayload.Repository.Name,
			"branch":  branch,
			"commit":  "-",
		})
	}

	// Use first commit data or tag info
	var commitID, message, author string
	if len(pushPayload.Commits) > 0 {
		commitID = pushPayload.Commits[0].ID
		message = pushPayload.Commits[0].Message
		author = pushPayload.Commits[0].Author.Name
	} else {
		commitID = "-"
		message = "-"
		author = "-"
	}

	h.triggerBuildFromWebhook(pushPayload.Repository.Name, branch, commitID, author)

	return c.JSON(fiber.Map{
		"status":  "received",
		"repo":    pushPayload.Repository.Name,
		"branch":  branch,
		"commit":  commitID,
		"message": message,
		"author":  author,
	})
}

// HandleGitLab processes GitLab push webhooks with simple token auth.
func (h *WebhookHandler) HandleGitLab(c fiber.Ctx) error {
	eventType := c.Get("X-Gitlab-Event")
	if eventType != "Push Hook" && eventType != "" {
		return c.Status(200).JSON(fiber.Map{"status": "ignored", "event": eventType})
	}

	secret := h.cfg.Secret
	if secret == "" {
		return c.Status(401).JSON(fiber.Map{"error": "webhook secret not configured"})
	}

	token := c.Get("X-Gitlab-Token")
	if token != secret {
		return c.Status(403).JSON(fiber.Map{"error": "invalid token"})
	}

	body := c.Body()
	if len(body) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}

	// Parse GitLab push payload
	var payload GitLabPushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON payload"})
	}

	// Skip pushes to tags
	if strings.HasPrefix(payload.Ref, "refs/tags/") {
		return c.JSON(fiber.Map{"status": "tag_push_ignored", "ref": payload.Ref})
	}

	// Extract branch
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	if branch == "" {
		return c.Status(400).JSON(fiber.Map{"error": "invalid ref format"})
	}

	// Filter to allowed branches
	switch branch {
	case "master", "main":
		// OK
	default:
		return c.JSON(fiber.Map{
			"status": "ignored",
			"reason": fmt.Sprintf("unsupported branch: %s", branch),
			"repo":   payload.Project.PathWithNamespace,
			"branch": branch,
		})
	}

	// Ignore force deletes
	if payload.After == "0000000000000000000000000000000000000000" {
		return c.Status(200).JSON(fiber.Map{"status": "force_delete_ignored"})
	}

	var commitID, message, author string
	if len(payload.Commits) > 0 {
		commitID = payload.Commits[0].ID
		message = payload.Commits[0].Message
		author = payload.Commits[0].Author.Name
	} else {
		commitID = payload.After
		message = "-"
		author = payload.User.Username
	}

	h.triggerBuildFromWebhook(payload.Project.PathWithNamespace, branch, commitID, author)

	return c.JSON(fiber.Map{
		"status":  "received",
		"repo":    payload.Project.PathWithNamespace,
		"branch":  branch,
		"commit":  commitID,
		"message": message,
		"author":  author,
	})
}

// triggerBuildFromWebhook schedules an async build triggered by a push event.
func (h *WebhookHandler) triggerBuildFromWebhook(repoName, branch, commit, author string) {
	if !h.cfg.AutoBuild {
		return
	}

	logFn := func(msg string) {
		fmt.Fprintf(os.Stderr, "[webhook] [%s:%s] %s", branch, repoName, msg)
	}

	projectID := resolveProjectID(h.buildSvc.DB(), repoName)
	if projectID == "" {
		logFn(fmt.Sprintf("WARNING: No project mapping found for repo=%s\n", repoName))
		return
	}

	target := h.cfg.DefaultTarget
	if target == "" {
		target = "universal"
	}
	arch := h.cfg.DefaultArch
	if arch == "" {
		arch = "arm64"
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		task, err := h.buildSvc.TriggerBuildFromGit(ctx, projectID, commit)
		if err != nil {
			logFn(fmt.Sprintf("BUILD FAILED: %v\n", err))
			return
		}
		logFn(fmt.Sprintf("build submitted taskID=%s\n", task.ID))
	}()
}

// resolveProjectID tries to find a project matching the repository name.
// It checks git_url in projects table. Simple mapping; production would use
// a dedicated webhook_project_id column per project.
func resolveProjectID(db any, repoName string) string {
	type dbQuerier interface {
		Query(query string, args ...any) (*sql.Rows, error)
	}
	q, ok := db.(dbQuerier)
	if !ok {
		return ""
	}

	rows, err := q.Query(
		`SELECT id FROM projects WHERE deleted_at IS NULL AND auto_build=1 AND git_url != '' LIMIT 1`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			return id
		}
	}
	return ""
}

// ── Event Payload Types ────────────────────────────────────────────────

// GitHubPushEvent represents a GitHub push webhook payload.
type GitHubPushEvent struct {
	Ref        string `json:"ref"`
	Repository struct {
		URL   string `json:"url"`
		Name  string `json:"name"`
		Owner string `json:"owner"`
	} `json:"repository"`
	Commits []struct {
		ID       string `json:"id"`
		Message  string `json:"message"`
		Author   struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
}

// GitLabPushEvent represents a GitLab push webhook payload.
type GitLabPushEvent struct {
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
		Name              string `json:"name"`
		HTTPURL           string `json:"http_url_to_repo"`
	} `json:"project"`
	Ref    string `json:"ref"`
	After  string `json:"after"`
	Before string `json:"before"`
	User   struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"user"`
	Commits []struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		TreeID    string `json:"tree_id"`
		Duration  int    `json:"duration"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
		Added    []string `json:"added,omitempty"`
		Modified []string `json:"modified,omitempty"`
		Removed  []string `json:"removed,omitempty"`
	} `json:"commits"`
}
