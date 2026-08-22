package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/domain"
)

// MultiStageBuild is the enhanced build pipeline for free models.
// Instead of generating ALL files in one shot (which truncates at ~30K tokens),
// it splits generation into 4 focused stages, each producing 1-2 files (~5K tokens).
//
// Stage 0: Architecture Planning (JSON plan, no code)
// Stage 1: Shell Layer (module.prop, customize.sh, META-INF) — ~100% success
// Stage 2: Core Logic (Go/C source, ONE FILE AT A TIME) — ~70% success
// Stage 3: Build System (go.mod, build.sh) — ~90% success
// Stage 4: Compile + AutoFix (existing pipeline)
func (s *AIService) MultiStageBuild(
	ctx context.Context,
	description, projectID, userID string,
	messages []Message, sessionID string,
	w *bufio.Writer,
) error {
	mu := &sync.Mutex{}
	safeSSE := func(data map[string]interface{}) {
		mu.Lock()
		s.sendSSE(w, data)
		mu.Unlock()
	}

	log.Printf("[MultiStage] Starting multi-stage build: %s", description)

	// Resolve LLM config
	endpoint, apiKey, model, _ := s.resolveLLMConfig(userID)

	// ===== Stage 0: Architecture Planning =====
	safeSSE(map[string]interface{}{
		"type":    "phase",
		"phase":   "planning",
		"message": "阶段0: 分析需求，规划架构...",
	})

	planPrompt := builder.MultiStageBuildPrompt(description)
	planJSON, err := s.callLLMForJSON(ctx, endpoint, apiKey, model, planPrompt)
	if err != nil {
		return fmt.Errorf("architecture planning failed: %w", err)
	}

	var plan builder.StagePlan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		log.Printf("[MultiStage] Plan parse failed, using fallback: %v", err)
		plan = fallbackPlan(description)
	}

	safeSSE(map[string]interface{}{
		"type":     "arch_plan",
		"plan":     plan,
		"message":  fmt.Sprintf("架构: %s (%s)", plan.Name, plan.Languages),
		"file_count": len(plan.ShellFiles) + len(plan.GoFiles) + len(plan.CFiles) + len(plan.BuildFiles) + len(plan.ExtraFiles),
	})

	// Create project
	projectID, projectDir, err := s.ensureProject(ctx, projectID, userID, plan.Name, description)
	if err != nil {
		return err
	}

	// Collect all generated files across stages
	allFiles := make(map[string]string) // path → content
	planJSONCompact, _ := json.Marshal(plan)

	// ===== Stage 1: Shell Layer =====
	safeSSE(map[string]interface{}{
		"type":    "phase",
		"phase":   "shell",
		"message": "阶段1: 生成 Shell 脚本层（module.prop, customize.sh, META-INF）...",
	})

	shellPrompt := builder.ShellStagePrompt(string(planJSONCompact), description)
	shellJSON, err := s.callLLMForJSON(ctx, endpoint, apiKey, model, shellPrompt)
	if err != nil {
		safeSSE(map[string]interface{}{
			"type":    "phase",
			"phase":   "warning",
			"message": fmt.Sprintf("Shell 生成失败: %v，继续后续阶段...", err),
		})
	} else {
		shellFiles := parseFilesJSON(shellJSON)
		for path, content := range shellFiles {
			allFiles[path] = content
			safeSSE(map[string]interface{}{"type": "file_saved", "path": path, "stage": "shell"})
		}
		log.Printf("[MultiStage] Stage 1: %d Shell files generated", len(shellFiles))
	}

	// ===== Stage 2: Core Logic (one file at a time) =====
	coreFiles := append(plan.GoFiles, plan.CFiles...)
	if len(coreFiles) > 0 {
		safeSSE(map[string]interface{}{
			"type":    "phase",
			"phase":   "core",
			"message": fmt.Sprintf("阶段2: 生成核心代码（%d 个源文件，逐个生成避免截断）...", len(coreFiles)),
		})

		shellFilesCompact := filesMapToJSON(allFiles)

		for i, fileInfo := range coreFiles {
			safeSSE(map[string]interface{}{
				"type":    "phase",
				"phase":   "core",
				"message": fmt.Sprintf("  生成 %s (%d/%d)...", fileInfo.Path, i+1, len(coreFiles)),
			})

			var corePrompt string
			if strings.HasSuffix(fileInfo.Path, ".go") {
				corePrompt = builder.GoStagePrompt(string(planJSONCompact), shellFilesCompact, description, fileInfo)
			} else {
				corePrompt = builder.CStagePrompt(string(planJSONCompact), shellFilesCompact, description, fileInfo)
			}

			coreJSON, err := s.callLLMForJSON(ctx, endpoint, apiKey, model, corePrompt)
			if err != nil {
				safeSSE(map[string]interface{}{
					"type":    "phase",
					"phase":   "warning",
					"message": fmt.Sprintf("  %s 生成失败: %v", fileInfo.Path, err),
				})
				continue
			}

			coreFilesMap := parseFilesJSON(coreJSON)
			for path, content := range coreFilesMap {
				allFiles[path] = content
				safeSSE(map[string]interface{}{"type": "file_saved", "path": path, "stage": "core"})
			}
		}
		log.Printf("[MultiStage] Stage 2: core files generated, total files now: %d", len(allFiles))
	}

	// ===== Stage 3: Build System =====
	safeSSE(map[string]interface{}{
		"type":    "phase",
		"phase":   "build_system",
		"message": "阶段3: 生成构建系统...",
	})

	sourceFilesJSON := filesMapToJSON(allFiles)
	buildPrompt := builder.BuildSystemPrompt(string(planJSONCompact), sourceFilesJSON, description)
	buildJSON, err := s.callLLMForJSON(ctx, endpoint, apiKey, model, buildPrompt)
	if err != nil {
		safeSSE(map[string]interface{}{
			"type":    "phase",
			"phase":   "warning",
			"message": fmt.Sprintf("构建脚本生成失败: %v", err),
		})
	} else {
		buildFiles := parseFilesJSON(buildJSON)
		for path, content := range buildFiles {
			allFiles[path] = content
			safeSSE(map[string]interface{}{"type": "file_saved", "path": path, "stage": "build"})
		}
		log.Printf("[MultiStage] Stage 3: %d build files generated", len(buildFiles))
	}

	// ===== Extra files (service.sh, uninstall.sh, config, etc.) =====
	if len(plan.ExtraFiles) > 0 {
		extraJSON, err := s.callLLMForJSON(ctx, endpoint, apiKey, model,
			builder.ShellStagePrompt(string(planJSONCompact), description))
		if err == nil {
			extraFiles := parseFilesJSON(extraJSON)
			for path, content := range extraFiles {
				// Only save files that are in ExtraFiles list
				for _, ef := range plan.ExtraFiles {
					if ef.Path == path {
						allFiles[path] = content
						safeSSE(map[string]interface{}{"type": "file_saved", "path": path, "stage": "extra"})
						break
					}
				}
			}
		}
	}

	// ===== Save all files to project =====
	safeSSE(map[string]interface{}{
		"type":    "phase",
		"phase":   "saving",
		"message": fmt.Sprintf("保存 %d 个文件到项目...", len(allFiles)),
	})

	for path, content := range allFiles {
		absPath := filepath.Join(projectDir, path)
		os.MkdirAll(filepath.Dir(absPath), 0755)
		os.WriteFile(absPath, []byte(content), 0644)
		s.db.ExecContext(ctx,
			`INSERT OR REPLACE INTO project_files (project_id, path, content, updated_at) VALUES (?,?,?,datetime('now'))`,
			projectID, path, content)
	}

	// ===== Stage 4: Compile =====
	safeSSE(map[string]interface{}{
		"type":    "phase",
		"phase":   "building",
		"message": "阶段4: 编译打包...",
	})

	buildSvc := NewBuildService(s.db, s.cfg)
	build, _, err := buildSvc.Create(ctx, projectID, "auto-multistage")
	if err != nil {
		return fmt.Errorf("build creation failed: %w", err)
	}

	safeSSE(map[string]interface{}{
		"type":       "build_started",
		"build_id":   build.ID,
		"project_id": projectID,
	})

	// Stream build and wait
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			var status string
			s.db.QueryRow("SELECT status FROM builds WHERE id = ?", build.ID).Scan(&status)
			mu.Lock()
			s.sendSSE(w, map[string]interface{}{
				"type":     "build_status",
				"build_id": build.ID,
				"status":   status,
			})
			mu.Unlock()

			if status == "success" || status == "error" {
				log.Printf("[MultiStage] Build completed: %s (status: %s)", build.ID, status)
				return nil
			}

			if status == "failed" {
				// Model fallback: if free model failed, retry with paid
				_, _, currentModel, _ := s.resolveLLMConfig(userID)
				if isFreeModel(currentModel) {
					log.Printf("[MultiStage] Build failed with free model %s, attempting paid fallback...", currentModel)
					safeSSE(map[string]interface{}{
						"type":    "phase",
						"phase":   "fallback",
						"message": "免费模型编译失败，使用付费模型重试...",
					})
					_, fbErr := s.modelFallbackRegenerate(
						ctx, projectID, userID, description, messages, sessionID, w, mu)
					if fbErr != nil {
						log.Printf("[MultiStage] Fallback error: %v", fbErr)
					}
				}
				return nil
			}
		}
	}
}

// callLLMForJSON calls the LLM and extracts a JSON response.
// Handles streaming, extracts the largest JSON block, validates.
func (s *AIService) callLLMForJSON(
	ctx context.Context,
	endpoint, apiKey, model, prompt string,
) (string, error) {
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream":          true,
		"max_tokens":      8192,
		"response_format": map[string]string{"type": "json_object"},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	chatURL := ensureChatCompletionsURL(endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	httpResp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(httpResp.Body)
		return "", fmt.Errorf("LLM HTTP %d: %s", httpResp.StatusCode, string(respBody[:min(len(respBody), 500)]))
	}

	// Parse streaming response
	var full strings.Builder
	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
			text := chunk.Choices[0].Delta.Content
			if text != "" {
				full.WriteString(text)
			}
		}
	}

	content := full.String()
	content = extractJSONBlock(content)

	// Validate it's valid JSON
	var test interface{}
	if err := json.Unmarshal([]byte(content), &test); err != nil {
		// Try fix truncated JSON
		fixed := fixTruncatedJSON(content)
		if fixed != content {
			if err2 := json.Unmarshal([]byte(fixed), &test); err2 == nil {
				return fixed, nil
			}
		}
		return "", fmt.Errorf("JSON parse failed after %d chars: %w", len(content), err)
	}

	return content, nil
}

// ensureProject creates the project if it doesn't exist, returns projectID and dir.
func (s *AIService) ensureProject(
	ctx context.Context,
	projectID, userID, name, description string,
) (string, string, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&exists)
	if err != nil {
		projectSvc := NewProjectService(s.db, s.cfg.StoragePath)
		if len(name) > 50 {
			name = name[:50]
		}
		proj, projErr := projectSvc.Create(ctx, userID, &domain.CreateProjectInput{
			Name:        name,
			Description: description,
		})
		if projErr != nil {
			return "", "", projErr
		}
		// Clean temp dir
		tempDir := filepath.Join(s.cfg.StoragePath, "projects", projectID)
		newDir := filepath.Join(s.cfg.StoragePath, "projects", proj.ID)
		if tempDir != newDir {
			os.RemoveAll(tempDir)
		}
		return proj.ID, newDir, nil
	}

	dir := filepath.Join(s.cfg.StoragePath, "projects", projectID)
	os.MkdirAll(dir, 0755)
	return projectID, dir, nil
}

// parseFilesJSON extracts {path: content} map from {"files":[...]} JSON.
func parseFilesJSON(jsonStr string) map[string]string {
	var result struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}
	files := make(map[string]string)
	for _, f := range result.Files {
		if f.Path != "" && f.Content != "" {
			files[f.Path] = f.Content
		}
	}
	return files
}

// filesMapToJSON converts {path: content} to a compact JSON string for LLM context.
func filesMapToJSON(files map[string]string) string {
	type fileEntry struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	var entries []fileEntry
	for path, content := range files {
		// Truncate very long files for context
		if len(content) > 3000 {
			content = content[:3000] + "\n// ... truncated ..."
		}
		entries = append(entries, fileEntry{Path: path, Content: content})
	}
	b, _ := json.Marshal(entries)
	return string(b)
}

// fallbackPlan creates a basic plan when LLM plan generation fails.
func fallbackPlan(description string) builder.StagePlan {
	return builder.StagePlan{
		ID:         "module",
		Name:       "AI Module",
		ModuleType: "tool",
		Languages:  []string{"shell"},
		ShellFiles: []builder.StageFileInfo{
			{Path: "module.prop", Description: "module metadata"},
			{Path: "customize.sh", Description: "installer"},
			{Path: "META-INF/com/google/android/update-binary", Description: "magisk template"},
			{Path: "META-INF/com/google/android/updater-script", Description: "#MAGISK"},
		},
		GoFiles:    nil,
		CFiles:     nil,
		BuildFiles: nil,
		ExtraFiles: nil,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
