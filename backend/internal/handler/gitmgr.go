package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type GitHandler struct {
	svc     *service.GitManagerService
	authSvc *service.AuthService
}

func NewGitHandler(svc *service.GitManagerService) *GitHandler {
	return &GitHandler{svc: svc}
}

func (h *GitHandler) SetAuthService(authSvc *service.AuthService) {
	h.authSvc = authSvc
}

type CommitRequest struct {
	ProjectID string `json:"project_id"`
	Message   string `json:"message"`
}

type CheckoutRequest struct {
	ProjectID string `json:"project_id"`
	Hash      string `json:"hash"`
}

func (h *GitHandler) Commit(c fiber.Ctx) error {
	var req CommitRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	info, err := h.svc.AddAndCommit(c.Context(), req.ProjectID, req.Message)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

func (h *GitHandler) ListCommits(c fiber.Ctx) error {
	projectID := c.Query("project_id", "")
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	commits, err := h.svc.ListCommits(c.Context(), projectID, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(commits)
}

func (h *GitHandler) GetDiff(c fiber.Ctx) error {
	projectID := c.Query("project_id", "")
	hash := c.Query("hash", "")
	diff, err := h.svc.GetDiff(c.Context(), projectID, hash)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"diff": diff})
}

func (h *GitHandler) Checkout(c fiber.Ctx) error {
	var req CheckoutRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := h.svc.CheckoutVersion(c.Context(), req.ProjectID, req.Hash); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "checked out"})
}

func (h *GitHandler) GetCurrentHash(c fiber.Ctx) error {
	projectID := c.Query("project_id", "")
	info, err := h.svc.GetCurrentHash(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

// ===== Branch Management =====

func (h *GitHandler) ListBranches(c fiber.Ctx) error {
	projectID := c.Query("project_id", "")
	branches, err := h.svc.ListBranches(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"branches": branches})
}

func (h *GitHandler) CreateBranch(c fiber.Ctx) error {
	var req struct {
		ProjectID  string `json:"project_id"`
		BranchName string `json:"branch_name"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.BranchName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "branch_name is required"})
	}
	if err := h.svc.CreateBranch(c.Context(), req.ProjectID, req.BranchName); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "branch created", "name": req.BranchName})
}

func (h *GitHandler) CheckoutBranch(c fiber.Ctx) error {
	var req struct {
		ProjectID  string `json:"project_id"`
		BranchName string `json:"branch_name"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.BranchName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "branch_name is required"})
	}
	if err := h.svc.CheckoutBranch(c.Context(), req.ProjectID, req.BranchName); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "checked out", "branch": req.BranchName})
}

func (h *GitHandler) GetCurrentBranch(c fiber.Ctx) error {
	projectID := c.Query("project_id", "")
	branch, err := h.svc.GetCurrentBranch(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"branch": branch})
}

func (h *GitHandler) Push(c fiber.Ctx) error {
	var req struct {
		ProjectID string `json:"project_id"`
		Remote    string `json:"remote"`
		Branch    string `json:"branch"`
		Token     string `json:"token"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	// Auto-fallback: if no token provided, use the user's stored GitHub token
	token := req.Token
	if token == "" && h.authSvc != nil {
		userID := safeUserID(c)
		if userID != "" {
			if storedToken, err := h.authSvc.GetGitHubToken(userID); err == nil && storedToken != "" {
				token = storedToken
			}
		}
	}

	output, err := h.svc.PushWithToken(c.Context(), req.ProjectID, req.Remote, req.Branch, token)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "output": output})
	}
	return c.JSON(fiber.Map{"status": "pushed", "output": output})
}

// PushOptimized pushes with advanced options including file filtering
func (h *GitHandler) PushOptimized(c fiber.Ctx) error {
	var req struct {
		ProjectID       string   `json:"project_id"`
		Remote          string   `json:"remote"`
		Branch          string   `json:"branch"`
		Token           string   `json:"token"`
		IncludePatterns []string `json:"include_patterns"`
		ExcludePatterns []string `json:"exclude_patterns"`
		CommitMessage   string   `json:"commit_message"`
		DryRun          bool     `json:"dry_run"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	// Auto-fallback: if no token provided, use the user's stored GitHub token
	token := req.Token
	if token == "" && h.authSvc != nil {
		userID := safeUserID(c)
		if userID != "" {
			if storedToken, err := h.authSvc.GetGitHubToken(userID); err == nil && storedToken != "" {
				token = storedToken
			}
		}
	}

	opts := service.PushOptions{
		IncludePatterns: req.IncludePatterns,
		ExcludePatterns: req.ExcludePatterns,
		CommitMessage:   req.CommitMessage,
		DryRun:          req.DryRun,
	}

	output, err := h.svc.PushWithOptions(c.Context(), req.ProjectID, req.Remote, req.Branch, token, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "output": output})
	}
	return c.JSON(fiber.Map{"status": "pushed", "output": output})
}

// PreviewFilesToPush returns list of files that would be pushed
func (h *GitHandler) PreviewFilesToPush(c fiber.Ctx) error {
	var req struct {
		ProjectID       string   `json:"project_id"`
		IncludePatterns []string `json:"include_patterns"`
		ExcludePatterns []string `json:"exclude_patterns"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	opts := service.PushOptions{
		IncludePatterns: req.IncludePatterns,
		ExcludePatterns: req.ExcludePatterns,
	}

	files, err := h.svc.GetFilesToPush(c.Context(), req.ProjectID, opts)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"files": files, "count": len(files)})
}

func (h *GitHandler) Pull(c fiber.Ctx) error {
	var req struct {
		ProjectID string `json:"project_id"`
		Remote    string `json:"remote"`
		Branch    string `json:"branch"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	output, err := h.svc.Pull(c.Context(), req.ProjectID, req.Remote, req.Branch)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "output": output})
	}
	return c.JSON(fiber.Map{"status": "pulled", "output": output})
}
