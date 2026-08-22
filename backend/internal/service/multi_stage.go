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
	"regexp"
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
	log.Printf("[MultiStage] LLM config: endpoint=%s model=%s apiKeyLen=%d", endpoint, model, len(apiKey))
	freeModelExhausted := false

	// ===== Stage 0: Architecture Planning =====
	safeSSE(map[string]interface{}{
		"type":    "phase",
		"phase":   "planning",
		"message": "阶段0: 分析需求，规划架构...",
	})

	planPrompt := builder.MultiStageBuildPrompt(description)
	planJSON, err := s.callLLMForJSON(ctx, endpoint, apiKey, model, planPrompt)
	var plan builder.StagePlan
	if err != nil {
		if strings.Contains(err.Error(), "FREE_QUOTA_EXHAUSTED") {
			log.Printf("[MultiStage] Free model quota exhausted, falling back to paid model")
			freeModelExhausted = true
			// Fall through — will retry all stages with paid model below
		} else {
			log.Printf("[MultiStage] Stage 0 plan failed: %v, using fallback", err)
			plan = fallbackPlan(description)
		}
	} else {
		if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
			log.Printf("[MultiStage] Stage 0 plan parse failed: %v, planJSON=%s", err, truncate(planJSON, 500))
			plan = fallbackPlan(description)
		} else {
			log.Printf("[MultiStage] Stage 0 plan OK: %s (%d shell, %d go, %d c files)",
				plan.Name, len(plan.ShellFiles), len(plan.GoFiles), len(plan.CFiles))
		}
	}

	// If free model quota exhausted, switch to paid model and retry everything
	if freeModelExhausted {
		paidEndpoint, paidKey, paidModel, _ := s.getPaidModelConfig(userID)
		if paidModel != "" {
			safeSSE(map[string]interface{}{
				"type":    "phase",
				"phase":   "fallback",
				"message": fmt.Sprintf("免费模型配额耗尽，切换到付费模型 %s 重试...", paidModel),
			})
			endpoint, apiKey, model = paidEndpoint, paidKey, paidModel
			freeModelExhausted = false

			// Retry Stage 0 with paid model
			planJSON, err = s.callLLMForJSON(ctx, endpoint, apiKey, model, planPrompt)
			if err != nil {
				log.Printf("[MultiStage] Paid model plan failed: %v, using fallback", err)
				plan = fallbackPlan(description)
			} else if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
				plan = fallbackPlan(description)
			}
		}
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
		"message": "阶段1: 生成 Shell 脚本层...",
	})

	// Delay between stages to respect free model rate limits
	time.Sleep(5 * time.Second)

	shellPrompt := builder.ShellStagePrompt(string(planJSONCompact), description)
	shellJSON, err := s.callLLMForJSON(ctx, endpoint, apiKey, model, shellPrompt)
	if err != nil {
		log.Printf("[MultiStage] Stage 1 shell failed: %v", err)
		safeSSE(map[string]interface{}{
			"type":    "phase",
			"phase":   "warning",
			"message": fmt.Sprintf("Shell 生成失败: %v，继续后续阶段...", err),
		})
	} else {
		log.Printf("[MultiStage] Stage 1 raw JSON (%d chars): %s", len(shellJSON), truncate(shellJSON, 1000))
		shellFiles := parseFilesJSON(shellJSON)
		log.Printf("[MultiStage] Stage 1: Shell LLM returned %d chars, parsed %d files", len(shellJSON), len(shellFiles))
		for path, content := range shellFiles {
			allFiles[path] = content
			safeSSE(map[string]interface{}{"type": "file_saved", "path": path, "stage": "shell"})
		}
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
			// Delay between file generations to respect rate limits
			if i > 0 {
				time.Sleep(10 * time.Second)
			}

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

	time.Sleep(10 * time.Second)

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
// Retries on 429 (rate limit) with exponential backoff up to 3 times.
func (s *AIService) callLLMForJSON(
	ctx context.Context,
	endpoint, apiKey, model, prompt string,
) (string, error) {
	maxRetries := 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Don't retry FreeUsageLimitError — quota exhausted, not transient
			backoff := time.Duration(1<<uint(attempt-1)) * 15 * time.Second // 15s, 30s, 60s
			log.Printf("[callLLMForJSON] Retry %d/%d after %v backoff", attempt, maxRetries, backoff)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		content, err := s.doLLMRequest(ctx, endpoint, apiKey, model, prompt)
		if err != nil && strings.Contains(err.Error(), "429") {
			if strings.Contains(err.Error(), "FreeUsageLimit") {
				return "", fmt.Errorf("FREE_QUOTA_EXHAUSTED: %w", err)
			}
			log.Printf("[callLLMForJSON] Rate limited (429), will retry: %v", err)
			continue
		}
		if err != nil && (strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "502")) {
			log.Printf("[callLLMForJSON] Model overloaded (5xx), will retry: %v", err)
			continue
		}
		if err != nil && strings.Contains(err.Error(), "401") {
			return "", fmt.Errorf("CREDITS_EXHAUSTED: %w", err)
		}
		return content, err
	}
	return "", fmt.Errorf("LLM request failed after %d retries", maxRetries)
}

// doLLMRequest performs a single LLM request (no retry).
func (s *AIService) doLLMRequest(
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
	log.Printf("[doLLMRequest] endpoint=%s apiKeyLen=%d model=%s keyPrefix=%s", chatURL, len(apiKey), model, apiKey[:min(20, len(apiKey))])
	req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	log.Printf("[doLLMRequest] headers=%v", req.Header)

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
		// Last resort: extract files via regex pattern matching
		// This handles cases where the LLM outputs JSON with unescaped
		// shell characters ($, }, etc.) that break strict JSON parsing
		files := extractFilesByPattern(content)
		if len(files) > 0 {
			// Rebuild a valid JSON from extracted files
			type fileEntry struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			var entries []fileEntry
			for p, c := range files {
				entries = append(entries, fileEntry{Path: p, Content: c})
			}
			rebuilt, _ := json.Marshal(map[string]interface{}{"files": entries})
			log.Printf("[doLLMRequest] Regex extracted %d files from malformed JSON", len(files))
			return string(rebuilt), nil
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
// Handles multiple output formats from different LLM models:
// 1. Standard: {"files":[{"path":"a","content":"1"},{"path":"b","content":"2"}]}
// 2. Flat keys: {"files":[{"path":"a","content":"1","path":"b","content":"2"}]}
// 3. Truncated/malformed JSON with regex fallback
func parseFilesJSON(jsonStr string) map[string]string {
	// First, try standard JSON parse
	var result struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err == nil && len(result.Files) > 0 {
		files := make(map[string]string)
		for _, f := range result.Files {
			if f.Path != "" && f.Content != "" {
				files[f.Path] = f.Content
			}
		}
		if len(files) > 0 {
			return files
		}
	}

	// Fallback: extract file entries via regex
	// Handles both flat-key objects and truncated JSON
	files := extractFilesByPattern(jsonStr)
	if len(files) > 0 {
		log.Printf("[parseFilesJSON] Regex pattern recovered %d files", len(files))
		return files
	}
	return nil
}

// extractFilesByPattern extracts path/content pairs from various malformed JSON formats.
// Uses sequential scanning rather than JSON parsing to handle:
// - Duplicate keys in single objects
// - Truncated JSON
// - Missing commas between entries
func extractFilesByPattern(s string) map[string]string {
	files := make(map[string]string)

	// Strategy 1: Find all "path":"..." and "content":"..." pairs sequentially
	// Use a state machine to pair them up
	pathRe := regexp.MustCompile(`"path"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	contentRe := regexp.MustCompile(`"content"\s*:\s*"((?:[^"\\]|\\.)*)"`)

	paths := pathRe.FindAllStringSubmatch(s, -1)
	contents := contentRe.FindAllStringSubmatch(s, -1)

	if len(paths) > 0 && len(contents) > 0 {
		// Pair them by index (path[0] with content[0], etc.)
		count := len(paths)
		if len(contents) < count {
			count = len(contents)
		}
		for i := 0; i < count; i++ {
			path := unescapeJSONString(paths[i][1])
			content := unescapeJSONString(contents[i][1])
			if path != "" && content != "" {
				files[path] = content
			}
		}
	}
	return files
}

// unescapeJSONString unescapes common JSON string escapes.
func unescapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\\n`, "\n")
	s = strings.ReplaceAll(s, `\\t`, "\t")
	s = strings.ReplaceAll(s, `\\"`, `"`)
	s = strings.ReplaceAll(s, `\\\\`, "\\")
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\"`, `"`)
	return s
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

// getPaidModelConfig returns available paid/free models in priority order.
// Tries DB configs first, then hardcoded fallbacks.
func (s *AIService) getPaidModelConfig(userID string) (endpoint, apiKey, model, providerID string) {
	// Try DB: look for any provider config with an API key
	rows, err := s.db.Query(`
		SELECT provider_id, base_url, api_key, model_id
		FROM llm_configs WHERE user_id = '' OR user_id IS NULL
		UNION ALL
		SELECT provider_id, base_url, api_key, model_id
		FROM llm_configs WHERE user_id = ?
		ORDER BY user_id DESC
	`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pid, url, key, mid string
			if rows.Scan(&pid, &url, &key, &mid) == nil && key != "" && !isFreeModel(mid) {
				return url, key, mid, pid
			}
		}
	}

	// Hardcoded fallback: try multiple models in order
	key := s.cfg.EffectiveLLMKey()
	if key != "" {
		// Priority: deepseek-v4-flash > qwen3.8-max > mimo-v2.5
		return "https://opencode.ai/zen/v1/chat/completions",
			key, "deepseek-v4-flash", "opencode-zen-paid"
	}

	return "", "", "", ""
}

// getAlternativeFreeModels returns a list of free models to try when the primary is exhausted.
func getAlternativeFreeModels() []string {
	return []string{"poolside/laguna-s-2.1-free", "mimo-v2.5-free"}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
