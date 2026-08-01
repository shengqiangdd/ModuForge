package handler

import (
	"bufio"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/service"
)

type BuildHandler struct {
	svc         *service.BuildService
	notifSvc    *service.NotificationService
	activitySvc *service.ActivityService
}

func NewBuildHandler(svc *service.BuildService) *BuildHandler {
	return &BuildHandler{svc: svc}
}

func (h *BuildHandler) SetNotifSvc(s *service.NotificationService) { h.notifSvc = s }
func (h *BuildHandler) SetActivitySvc(s *service.ActivityService) { h.activitySvc = s }

func (h *BuildHandler) Create(c fiber.Ctx) error {
	projectID := c.Params("id")
	var req domain.BuildRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Target == "" {
		req.Target = "universal"
	}
	trigger := req.Trigger
	if trigger == "" {
		trigger = "manual"
	}
	arch := req.Arch
	if arch == "" {
		arch = "arm64"
	}
	task, err := h.svc.CreateWithTrigger(c.Context(), projectID, req.Target, trigger, "", arch)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	if h.notifSvc != nil {
		if userID, err := parseUserID(c); err == nil {
			h.notifSvc.Create(userID, "build_started", "构建已开始", "项目 "+projectID+" 的构建已开始", "/projects/"+projectID+"/build")
		}
	}
	if h.activitySvc != nil {
		if userID, err := parseUserID(c); err == nil {
			pid, _ := parseInt64(projectID)
			h.activitySvc.Log(userID, pid, "build_started", "启动了构建")
		}
	}

	service.NotifyUser(safeUserID(c), "build_started", fiber.Map{
		"project_id": projectID, "build_id": task.ID, "status": task.Status,
	})

	return c.Status(201).JSON(task)
}

func (h *BuildHandler) ClearBuildCache(c fiber.Ctx) error {
	projectID := c.Params("id")
	if err := h.svc.ClearBuildCache(c.Context(), projectID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// GetBuildCacheStatus returns cache statistics for a project.
func (h *BuildHandler) GetBuildCacheStatus(c fiber.Ctx) error {
	projectID := c.Params("id")
	status, err := h.svc.GetBuildCacheStatus(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(status)
}

// GetSupportedArchitectures returns the list of supported build architectures.
func (h *BuildHandler) GetSupportedArchitectures(c fiber.Ctx) error {
	archs := builder.GetSupportedArchitectures()
	return c.JSON(fiber.Map{"architectures": archs})
}

func (h *BuildHandler) CreateAuto(c fiber.Ctx) error {
	projectID := c.Params("id")
	var req struct {
		Target     string `json:"target"`
		CommitHash string `json:"commit_hash"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}
	if req.Target == "" {
		req.Target = "universal"
	}
	task, err := h.svc.CreateWithTrigger(c.Context(), projectID, req.Target, "git", req.CommitHash, "arm64")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	if h.notifSvc != nil {
		if userID, err := parseUserID(c); err == nil {
			h.notifSvc.Create(userID, "build_started", "自动构建已开始", "项目 "+projectID+" 的 Git 触发构建已开始", "/projects/"+projectID+"/build")
		}
	}
	if h.activitySvc != nil {
		if userID, err := parseUserID(c); err == nil {
			pid, _ := parseInt64(projectID)
			h.activitySvc.Log(userID, pid, "build_started", "启动了 Git 触发构建")
		}
	}

	return c.Status(201).JSON(task)
}

func (h *BuildHandler) Cancel(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.svc.CancelBuild(c.Context(), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *BuildHandler) Get(c fiber.Ctx) error {
	id := c.Params("id")
	task, err := h.svc.GetWithCommit(c.Context(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(task)
}

func (h *BuildHandler) StreamLogs(c fiber.Ctx) error {
	id := c.Params("id")
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	task, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	c.RequestCtx().SetBodyStreamWriter(func(w *bufio.Writer) {
		w.WriteString("data: " + task.Log + "\n\n")
		w.Flush()
		select {}
	})
	return nil
}

func safeUserID(c fiber.Ctx) string {
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		return uid
	}
	if uid, ok := c.Locals("uid").(string); ok && uid != "" {
		return uid
	}
	return ""
}

func parseUserID(c fiber.Ctx) (string, error) {
	uid := safeUserID(c)
	if uid == "" {
		return "", fiber.NewError(fiber.StatusUnauthorized, "missing user")
	}
	return uid, nil
}

func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fiber.NewError(fiber.StatusBadRequest, "invalid number")
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

func (h *BuildHandler) Download(c fiber.Ctx) error {
	id := c.Params("id")
	path, err := h.svc.GetArtifact(c.Context(), id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendFile(*path)
}

// ListByProject returns build history for a project.
func (h *BuildHandler) ListByProject(c fiber.Ctx) error {
	projectID := c.Params("id")
	tasks, err := h.svc.ListByProject(c.Context(), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(tasks)
}

// GetGlobalCacheStats returns cache statistics across all projects.
func (h *BuildHandler) GetGlobalCacheStats(c fiber.Ctx) error {
	stats := builder.GetGlobalCacheStats(h.svc.GetStoragePath())
	return c.JSON(stats)
}

// TriggerCacheCleanup manually triggers a cache cleanup pass.
func (h *BuildHandler) TriggerCacheCleanup(c fiber.Ctx) error {
	cfg := builder.DefaultCacheConfig(h.svc.GetStoragePath())
	mgr := builder.NewGlobalCacheManager(cfg)
	// Run cleanup synchronously
	mgr.RunCleanup()
	stats := builder.GetGlobalCacheStats(h.svc.GetStoragePath())
	return c.JSON(fiber.Map{
		"message": "Cache cleanup completed",
		"stats":   stats,
	})
}
