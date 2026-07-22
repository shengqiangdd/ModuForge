package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type PluginHandler struct {
	plugin *service.PluginService
}

func NewPluginHandler(plugin *service.PluginService) *PluginHandler {
	return &PluginHandler{plugin: plugin}
}

func (h *PluginHandler) Install(c fiber.Ctx) error {
	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
		Author      string `json:"author"`
		Version     string `json:"version"`
		Config      string `json:"config"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	plugin, err := h.plugin.InstallPlugin(c.Context(), req.Name, req.Slug, req.Description, req.Author, req.Version, req.Config)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(plugin)
}

func (h *PluginHandler) List(c fiber.Ctx) error {
	list, err := h.plugin.ListPlugins(c.Context())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"plugins": list})
}

func (h *PluginHandler) Enable(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.plugin.EnablePlugin(c.Context(), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *PluginHandler) Disable(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.plugin.DisablePlugin(c.Context(), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *PluginHandler) Uninstall(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.plugin.UninstallPlugin(c.Context(), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *PluginHandler) RegisterHook(c fiber.Ctx) error {
	pluginID := c.Params("id")
	var req struct {
		HookName   string `json:"hook_name"`
		HookType   string `json:"hook_type"`
		EntryPoint string `json:"entry_point"`
		Config     string `json:"config"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	hook, err := h.plugin.RegisterHook(c.Context(), pluginID, req.HookName, req.HookType, req.EntryPoint, req.Config)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(hook)
}

func (h *PluginHandler) GetHooks(c fiber.Ctx) error {
	pluginID := c.Params("id")
	hooks, err := h.plugin.GetPluginHooks(c.Context(), pluginID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"hooks": hooks})
}

func (h *PluginHandler) ExecuteHook(c fiber.Ctx) error {
	var req struct {
		HookName string                 `json:"hook_name"`
		Input    map[string]interface{} `json:"input"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	result, err := h.plugin.ExecuteHook(c.Context(), req.HookName, req.Input)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}
