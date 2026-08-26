package handler

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/code"
)

// HandleAnalyzeDependencies 分析项目依赖
func (h *AIHandler) HandleAnalyzeDependenciesNew(c fiber.Ctx) error {
	type request struct {
		RootDir string `json:"root_dir"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if req.RootDir == "" {
		req.RootDir = "." // 默认当前目录
	}

	pm := code.NewPackageManager(req.RootDir)
	result, err := pm.AnalyzeDependencies()
	if err != nil {
		return BadRequest(c, err.Error())
	}

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}

// HandleAnalyzeComplexity 分析代码复杂度
func (h *AIHandler) HandleAnalyzeComplexity(c fiber.Ctx) error {
	type request struct {
		Files map[string]string `json:"files"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	if len(req.Files) == 0 {
		return BadRequest(c, "Files are required")
	}

	pa := code.NewProjectAnalytics()
	result := pa.AnalyzeComplexity(req.Files)

	return c.Status(200).JSON(fiber.Map{
		"valid":  true,
		"result": result,
	})
}

// HandleCheckCustomRules 执行自定义规则检查
func (h *AIHandler) HandleCheckCustomRules(c fiber.Ctx) error {
	type request struct {
		Files    map[string]string `json:"files"`
		Language string            `json:"language"`
		Rules    []code.CustomRule `json:"rules"`
	}

	var req request
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return BadRequest(c, "Invalid request body")
	}

	engine := code.NewCustomRuleEngine()
	for _, rule := range req.Rules {
		engine.AddRule(rule)
	}

	results := engine.CheckFiles(req.Files, req.Language)

	return c.Status(200).JSON(fiber.Map{
		"valid":   true,
		"results": results,
		"count":   len(results),
	})
}
