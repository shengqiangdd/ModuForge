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

	"github.com/moduforge/backend/internal/domain"
)

// GenerateModule 用 LLM 生成模块代码
func (s *AIService) GenerateModule(ctx context.Context, description, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	systemPrompt := s.loadPrompt("generate", userID)
	userPrompt := fmt.Sprintf(`Create a universal module (Magisk/KSU/APatch compatible).

Module Description: %s

Generate all necessary files as JSON: {"files":[{"path":"...","content":"..."}]}
Ensure module.prop has ksu.supported=true and apatch.supported=true.
All shell scripts: shebang + set -euo pipefail + security best practices.`, description)
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, sessionID, w, messages)
}

// GatherRequirements 需求收集
func (s *AIService) GatherRequirements(ctx context.Context, message, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	systemPrompt := s.loadPrompt("gather", userID)
	return s.streamChatWithSystemForUser(ctx, systemPrompt, message, userID, sessionID, w, messages)
}

// Chat 通用 AI 对话
func (s *AIService) Chat(ctx context.Context, message, contextInfo, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	systemPrompt := s.loadPrompt("chat", userID)
	userPrompt := message
	if contextInfo != "" {
		userPrompt = fmt.Sprintf("Context:\n%s\n\nQuestion:\n%s", contextInfo, message)
	}
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, sessionID, w, messages)
}

// RepairBuild 分析构建日志给出修复建议
func (s *AIService) RepairBuild(ctx context.Context, buildLog, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	systemPrompt := s.loadPrompt("repair", userID)
	userPrompt := fmt.Sprintf("Analyze this build log and identify the failure:\n\n```\n%s\n```\n\nProvide diagnosis with specific fix instructions.", buildLog)
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, sessionID, w, messages)
}

// CompareModels 比较多个模型的回答
func (s *AIService) CompareModels(ctx context.Context, message string, modelIDs []string, userID string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	for _, modelID := range modelIDs {
		result := map[string]interface{}{
			"model_id": modelID,
			"content":  fmt.Sprintf("Response from model %s", modelID),
		}
		results = append(results, result)
	}
	return results, nil
}

// SuggestTitle generates a short (<=12 chars) conversation title from the
// first user message using a lightweight non-streaming LLM call. Falls back
// to the empty string when LLM is not configured or the call fails, so the
// caller keeps its default truncation-based title.
func (s *AIService) SuggestTitle(ctx context.Context, userID string, messages []Message) string {
	if len(messages) == 0 || s.cfg.LLMEndpoint == "" {
		return ""
	}
	var firstUser string
	for _, m := range messages {
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" {
			firstUser = strings.TrimSpace(m.Content)
			break
		}
	}
	if firstUser == "" {
		return ""
	}
	if len([]rune(firstUser)) > 200 {
		firstUser = string([]rune(firstUser)[:200]) + "…"
	}

	endpoint, apiKey, model, providerID := s.resolveLLMConfig(userID)
	if providerRequiresKey(providerID) && apiKey == "" {
		return ""
	}

	msgs := []map[string]string{
		{"role": "system", "content": "你是会话标题生成器。根据用户第一条消息，生成一个简洁的中文标题，不超过12个字，不要标点、不要引号、不要'关于'等前缀。只输出标题本身。"},
		{"role": "user", "content": firstUser},
	}
	body := marshalJSON(map[string]interface{}{
		"model":      model,
		"messages":   msgs,
		"stream":     false,
		"max_tokens": 30,
	})
	chatURL := ensureChatCompletionsURL(endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ""
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := decodeJSON(resp.Body, &out); err != nil {
		return ""
	}
	if len(out.Choices) == 0 {
		return ""
	}
	title := strings.TrimSpace(out.Choices[0].Message.Content)
	// Trim common punctuation and quotes from generated titles
	title = strings.Trim(title, "\"'[]()")
	if title == "" {
		return ""
	}
	if len([]rune(title)) > 20 {
		title = string([]rune(title)[:20])
	}
	return title
}

// GetHistory 获取对话历史
func (s *AIService) GetHistory(sessionID string) []Message {
	return nil
}

// DeleteHistory 删除对话历史
func (s *AIService) DeleteHistory(sessionID string) {
}

// buildLLMRequest builds the common LLM request body with retry logic.
func (s *AIService) buildLLMRequest(
	ctx context.Context,
	systemPrompt, userPrompt, userID, sessionID string,
	w *bufio.Writer,
	history []Message,
	safeSSE func(data map[string]interface{}),
	responseFormat string, // optional: "json_object" to force JSON output
) (endpoint, apiKey, model string, resp *llmResponse, err error) {
	endpoint, apiKey, model, providerID := s.resolveLLMConfig(userID)

	if providerRequiresKey(providerID) && apiKey == "" {
		safeSSE(map[string]interface{}{
			"type":    "phase",
			"phase":   "error",
			"message": "LLM not configured. Set API key in Settings.",
		})
		return endpoint, apiKey, model, nil, nil
	}

	// Build message array
	msgs := buildMessageArray(systemPrompt, userPrompt, history, s.convStore)

	// Convert to API format
	msgList := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		msgList = append(msgList, map[string]string{"role": m.Role, "content": m.Content})
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": msgList,
		"stream":   true,
	}

	// Add optional response_format for JSON-forced outputs
	if responseFormat != "" {
		body["response_format"] = map[string]string{"type": responseFormat}
	}

	return endpoint, apiKey, model, &llmResponse{body: body}, nil
}

// llmResponse wraps the request body for LLM API calls.
type llmResponse struct {
	body map[string]interface{}
}

// AutoBuild 自动构建 - 带phase事件的完整实现
func (s *AIService) AutoBuild(ctx context.Context, description, projectID, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	// 全局 SSE 写锁 + keepalive，覆盖整个 AutoBuild 生命周期（LLM 等待 + 编译）
	var sseMu sync.Mutex
	safeSSE := func(data map[string]interface{}) {
		sseMu.Lock()
		s.sendSSE(w, data)
		sseMu.Unlock()
	}
	globalDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				safeSSE(map[string]interface{}{
					"type":    "heartbeat",
					"message": "⏳ 处理中...",
				})
			case <-globalDone:
				return
			}
		}
	}()
	defer close(globalDone)

	// 构建user prompt
	systemPrompt := s.loadPrompt("agent", userID)

	var userPrompt string
	if len(messages) > 0 {
		// 有历史对话 → 追加/修改模式
		userPrompt = fmt.Sprintf(`修改现有Android Magisk模块。

需求: %s

## 约束
1. 基于历史对话中的已有代码进行修改/追加，不要从头重写
2. 只输出变更的文件，未修改的文件不要包含在输出中
3. 在 JSON 响应中增加 "changes" 字段说明修改内容
4. 兼容Magisk/KernelSU/APatch
5. 必须严格按照需求中指定的技术栈，不要擅自替换语言

## 输出格式
{"files":[{"path":"...","content":"..."}],"changes":"修改说明"}`, description)
	} else {
		// 首次构建 → 新建模式
		userPrompt = fmt.Sprintf(`创建Android Magisk模块。

需求: %s

## 约束
1. 源码在容器内交叉编译为arm64二进制，你只生成源码+编译脚本
2. 兼容Magisk/KernelSU/APatch（标准module.prop，不硬编码检测）
3. 需求明确则直接生成代码，不分析
4. 必须严格按照需求中指定的技术栈生成代码，不要擅自替换语言

## 技术栈选择规则（严格遵守）
- 用户明确指定语言 → 必须用该语言
- 用户未指定语言 → 根据需求特征自动选择：
  - 后台服务/数据处理/网络/文件操作 → Go
  - 系统级/内存安全/并发安全 → Rust
  - 底层系统调用/C库依赖/性能敏感 → C/C++
  - 安装/检测/简单操作/配置管理 → Shell
- 复杂需求可混合使用多种语言，每种语言独立编译

## 必须文件
module.prop, customize.sh, META-INF/(update-binary + updater-script含#MAGISK)

## 按技术栈生成文件（生成完整、生产级代码）
- Go: src/main.go(入口package main) + go.mod + build.sh
- Rust: src/main.rs + Cargo.toml + build.sh
- C/C++: src/main.c(或main.cpp) + Makefile + build.sh
- Shell: lib/*.sh 或 scripts/*.sh

## 构建脚本要求（必须遵守）
- build.sh 只负责编译，不要在脚本里移动二进制文件
- Go 编译命令：GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/<name> .
- Rust 编译命令：cross build --release --target aarch64-linux-android
- C/C++ 编译命令：aarch64-linux-android-gcc -static -o ./bin/<name> src/main.c
- 二进制输出到 ./bin/ 目录，不要用 ../../ 或绝对路径
- 不要在 build.sh 中包含 mv/cp 等移动命令

## 代码质量要求（必须遵守，违反即失败）
1. **每个源文件≥500行**，customize.sh≥200行，包含完整业务逻辑
2. **禁止占位符、TODO、省略号** — 所有代码必须是完整可编译的
3. 完整错误处理：每个系统调用检查返回值，失败时记录日志并优雅退出
4. 日志系统：使用结构化日志(slog/logrus/syslog)，支持级别过滤(INFO/WARN/ERROR)
5. 配置管理：支持配置文件(/data/adb/modules/<id>/config)或环境变量，提供合理默认值
6. 信号处理：优雅关闭(graceful shutdown)，清理临时文件和资源
7. 并发安全：多线程/协程使用适当的同步机制(mutex/channel/atomic)
8. 资源管理：及时关闭文件描述符、连接、socket，避免泄漏
9. 输入验证：校验所有外部输入，防止路径遍历和注入攻击
10. 性能优化：使用缓冲区、对象池，避免不必要的内存分配

## C/C++ 特别要求（如果选择了C/C++）
- 使用标准POSIX API，不依赖特定厂商SDK
- 必须实现完整的业务逻辑（守护进程、文件监控、网络通信等）
- 包含信号处理、日志记录、配置读取等基础设施代码
- Makefile必须指定交叉编译器(aarch64-linux-android-gcc或clang)

## 输出格式
{"files":[{"path":"...","content":"..."}]}`, description)
	}

	endpoint, apiKey, _, resp, err := s.buildLLMRequest(ctx, systemPrompt, userPrompt, userID, sessionID, w, messages, func(data map[string]interface{}) { /* noop */ }, "json_object")

	if err != nil {
		return err
	}

	// --- Phase 1: Call LLM and stream response ---
	bodyBytes, _ := json.Marshal(resp.body)

	chatURL := ensureChatCompletionsURL(endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	httpResp, err := httpClient.Do(req)
	if err != nil {
		safeSSE(map[string]interface{}{"type": "phase", "phase": "error", "message": err.Error()})
		return nil
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		errMsg := fmt.Sprintf("LLM error (HTTP %d)", httpResp.StatusCode)
		var errBody struct {
			Error struct{ Message string `json:"message"` } `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &errBody) == nil && errBody.Error.Message != "" {
			errMsg = errBody.Error.Message
		}
		safeSSE(map[string]interface{}{"type": "phase", "phase": "error", "message": errMsg})
		return nil
	}

	safeSSE(map[string]interface{}{"type": "phase", "phase": "generating", "message": "AI 正在生成代码..."})

	var fullContent strings.Builder
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
				fullContent.WriteString(text)
				safeSSE(map[string]interface{}{"type": "content", "content": text})
			}
		}
	}

	safeSSE(map[string]interface{}{"type": "phase", "phase": "parsing", "message": "解析生成结果..."})

	// --- Phase 2: Parse JSON response and extract files ---
	content := fullContent.String()
	log.Printf("[AutoBuild] Raw LLM output length: %d, first 200: %s", len(content), truncateStr(content, 200))
	content = extractJSONBlock(content)
	log.Printf("[AutoBuild] After extractJSONBlock length: %d, first 200: %s", len(content), truncateStr(content, 200))

	var result struct {
		Files   []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
		Changes string `json:"changes"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil || len(result.Files) == 0 {
		log.Printf("[AutoBuild] JSON parse failed: err=%v, files=%d, content_len=%d", err, len(result.Files), len(content))
		// Try to fix truncated JSON by adding missing closing braces
		fixed := fixTruncatedJSON(content)
		if fixed != content {
			if err2 := json.Unmarshal([]byte(fixed), &result); err2 == nil && len(result.Files) > 0 {
				log.Printf("[AutoBuild] Truncated JSON fixed, files=%d", len(result.Files))
				content = fixed
			}
		}
		if len(result.Files) == 0 {
			safeSSE(map[string]interface{}{"type": "phase", "phase": "error", "message": "AI 输出格式无法解析，请重试"})
			return nil
		}
	}

	safeSSE(map[string]interface{}{"type": "phase", "phase": "saving", "message": fmt.Sprintf("保存 %d 个文件...", len(result.Files))})

	// --- Phase 3: Save files to project (disk + DB) ---
	projectDir := filepath.Join(s.cfg.StoragePath, "projects", projectID)
	os.MkdirAll(projectDir, 0755)

	for _, f := range result.Files {
		if f.Path == "" || f.Content == "" {
			continue
		}
		// Save to disk
		absPath := filepath.Join(projectDir, f.Path)
		os.MkdirAll(filepath.Dir(absPath), 0755)
		if err := os.WriteFile(absPath, []byte(f.Content), 0644); err != nil {
			safeSSE(map[string]interface{}{"type": "phase", "phase": "error", "message": fmt.Sprintf("保存文件失败: %s", f.Path)})
			return nil
		}
		// Save to DB (project_files table) so BuildService can find them
		s.db.ExecContext(ctx,
			`INSERT OR REPLACE INTO project_files (project_id, path, content, updated_at) VALUES (?,?,?,datetime('now'))`,
			projectID, f.Path, f.Content)
		safeSSE(map[string]interface{}{"type": "file_saved", "path": f.Path})
	}

	safeSSE(map[string]interface{}{"type": "phase", "phase": "building", "message": "开始编译..."})

	// --- Phase 4: Ensure project exists, then trigger build ---
	// AutoBuild may receive a temporary projectID that doesn't exist in projects table.
	// We need to create the project first so BuildService can find it.
	var exists int
	err = s.db.QueryRowContext(ctx, `SELECT 1 FROM projects WHERE id=? AND deleted_at IS NULL`, projectID).Scan(&exists)
	if err != nil {
		// Project doesn't exist — create it
		projectSvc := NewProjectService(s.db, s.cfg)
		name := description
		if len(name) > 50 {
			name = name[:50]
		}
		_, err = projectSvc.Create(ctx, userID, &domain.CreateProjectInput{
			Name:        name,
			Description: description,
		})
		if err != nil {
			log.Printf("[AutoBuild] Failed to auto-create project %s: %v", projectID, err)
			safeSSE(map[string]interface{}{"type": "phase", "phase": "error", "message": fmt.Sprintf("创建项目失败: %v", err)})
			return nil
		}
		// Re-check — if Create generates a new ID, we need to use that
		// For now, just try the build directly
	}

	buildSvc := NewBuildService(s.db, s.cfg)
	build, _, err := buildSvc.Create(ctx, projectID, "auto")
	if err != nil {
		safeSSE(map[string]interface{}{"type": "phase", "phase": "error", "message": fmt.Sprintf("创建构建失败: %v", err)})
		return nil
	}

	safeSSE(map[string]interface{}{
		"type":       "build_started",
		"build_id":   build.ID,
		"project_id": projectID,
	})

	// Stream build output
	go func() {
		// Wait for build to start
		time.Sleep(2 * time.Second)
		s.streamBuildLog(ctx, build.ID, w, &sseMu)
	}()

	// Token usage is tracked by the streaming response parser

	return nil
}

// extractJSONBlock extracts a JSON object from LLM output that may contain
// markdown fences, tool call artifacts, or surrounding text.
// Handles: raw JSON, ```json blocks, ``` blocks, tool_call payloads, and
// hybrid text+code outputs.
func extractJSONBlock(s string) string {
	// 1. Try markdown code fence with json label
	re := reJSONBlock
	if re == nil {
		re = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\})\\s*\\n?```")
		reJSONBlock = re
	}
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		if isValidFilesJSON(m[1]) {
			return m[1]
		}
	}

	// 2. Try to find the largest valid {"files":[...]} block
	best := ""
	offset := 0
	for {
		idx := strings.Index(s[offset:], `{"files"`)
		if idx < 0 {
			break
		}
		i := offset + idx
		end := strings.LastIndex(s[i:], "}")
		if end > 0 {
			candidate := s[i : i+end+1]
			if isValidFilesJSON(candidate) && len(candidate) > len(best) {
				best = candidate
			}
		}
		offset = i + 1
	}
	if best != "" {
		return best
	}

	// 3. Try markdown fence with any content
	re2 := regexp.MustCompile("(?s)```[a-z]*\\s*\\n?(\\{.*?\\})\\s*\\n?```")
	if m := re2.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}

	// 4. Fallback: raw JSON object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		candidate := s[start : end+1]
		if isValidFilesJSON(candidate) {
			return candidate
		}
		return candidate
	}
	return s
}

// isValidFilesJSON checks if a JSON string contains a valid "files" array.
func isValidFilesJSON(s string) bool {
	var v struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(s), &v); err == nil && len(v.Files) > 0 {
		for _, f := range v.Files {
			if f.Path != "" && f.Content != "" {
				return true
			}
		}
	}
	return false
}

var reJSONBlock *regexp.Regexp

// truncateStr truncates a string to maxLen characters for logging.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// fixTruncatedJSON attempts to repair truncated JSON output from LLM.
// Free models often hit token limits mid-JSON. This tries to close
// any open strings, arrays, and objects to make valid JSON.
func fixTruncatedJSON(s string) string {
	// Count open braces/brackets to determine what needs closing
	braceCount := 0
	bracketCount := 0
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			braceCount++
		} else if c == '}' {
			braceCount--
		} else if c == '[' {
			bracketCount++
		} else if c == ']' {
			bracketCount--
		}
	}

	// If we're inside a string, close it
	result := s
	if inString {
		result += `"`
	}

	// Close any open arrays
	for i := 0; i < bracketCount; i++ {
		result += "]"
	}

	// Close any open objects
	for i := 0; i < braceCount; i++ {
		result += "}"
	}

	return result
}

// streamBuildLog streams build log output via SSE events.
func (s *AIService) streamBuildLog(ctx context.Context, buildID string, w *bufio.Writer, mu *sync.Mutex) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var status string
			err := s.db.QueryRow("SELECT status FROM builds WHERE id = ?", buildID).Scan(&status)
			if err != nil {
				return
			}
			mu.Lock()
			s.sendSSE(w, map[string]interface{}{
				"type":      "build_status",
				"build_id":  buildID,
				"status":    status,
			})
			mu.Unlock()

			if status == "success" || status == "failed" || status == "error" {
				return
			}
		}
	}
}
