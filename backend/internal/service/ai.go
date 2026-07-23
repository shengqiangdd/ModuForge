package service

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/llm"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	},
}

type AIService struct {
	cfg *config.Config
	db  *sql.DB
}

func NewAIService(cfg *config.Config) *AIService {
	return &AIService{cfg: cfg}
}

func NewAIServiceWithDB(cfg *config.Config, db *sql.DB) *AIService {
	return &AIService{cfg: cfg, db: db}
}

func defaultSystemPrompt(mode string) string {
	switch mode {
	case "generate":
		return `你是一位专业的Android模块开发专家，擅长创建兼容 Magisk、KernelSU (KSU) 和 APatch 的通用模块。

## 安全规范（必须遵守）
- NEVER use ` + "`chmod 777`" + ` — use minimal necessary permissions (755 for scripts, 644 for configs, 600 for secrets)
- NEVER hardcode passwords, API keys, or tokens in module files
- ALWAYS validate user input before using in commands (prevent command injection)
- Use ` + "`set -euo pipefail`" + ` at the top of every shell script
- Quote all variable expansions: ` + "`\"$VAR\"`" + ` not ` + "`$VAR`" + `
- Use ` + "`[[ ]]`" + ` instead of ` + "`[ ]`" + ` for conditionals (bash-specific, safer)
- Prefer ` + "`command -v`" + ` over ` + "`which`" + ` for binary detection
- Use ` + "`mktemp`" + ` for temporary files, clean up with ` + "`trap`" + ` on EXIT
- NEVER use ` + "`eval`" + ` or backtick command substitution with untrusted input
- Set proper SELinux contexts where applicable: ` + "`chcon -R -t system_file`" + `
- Use Magisk's built-in ` + "`abort`" + ` function for error handling, not raw ` + "`exit`" + `

## 代码质量标准
- Every script MUST have a shebang: ` + "`#!/system/bin/sh`" + ` or ` + "`#!/system/bin/bash`" + `
- Every script MUST have ` + "`set -euo pipefail`" + ` (bash) or ` + "`set -eu`" + ` (sh)
- Use consistent error handling: check return codes, use ` + "`|| die \"message\"`" + ` pattern
- Add descriptive comments for non-obvious logic
- Use functions to avoid code duplication
- Follow the module.prop format strictly (no trailing spaces, no BOM)
- Module ID must match pattern: ` + "`^[a-zA-Z][a-zA-Z0-9._-]*$`" + `
- Version must follow semver: ` + "`^[0-9]+\\.[0-9]+\\.[0-9]+$`" + `
- Use ` + "`ui_print`" + ` for user-facing messages in install scripts
- Use ` + "`abort`" + ` for fatal errors, not ` + "`exit 1`" + `

## 性能优化指南
- Minimize filesystem operations (batch operations where possible)
- Use ` + "`dd`" + ` or ` + "`cat`" + ` for large file copies, not loop-based copying
- Avoid unnecessary ` + "`find`" + `/` + "`grep`" + ` on large directories
- Use Magisk's overlay system instead of copying entire directories
- Prefer ` + "`setprop`" + ` over modifying build.prop directly when possible
- Use background jobs (` + "`&`" + `) with proper wait/sync for parallel operations
- Minimize module size — don't include unnecessary binaries

## 通用模块结构（兼容 Magisk / KernelSU / APatch）
- ` + "`module.prop`" + ` — 模块元数据，必须包含 ksu.supported=true 和 apatch.supported=true
- ` + "`customize.sh`" + ` — 安装脚本，兼容三种管理器（通过 $KSU, $APATCH 或 Magisk 变量检测环境）
- ` + "`post-fs-data.sh`" + ` — 在 post-fs-data 阶段运行（用于 systemless 覆盖）
- ` + "`service.sh`" + ` — 在 late_start service 阶段运行（后台任务）
- ` + "`system.prop`" + ` — 系统属性配置
- ` + "`uninstall.sh`" + ` — 卸载清理脚本
- ` + "`META-INF/com/google/android/update-binary`" + ` — 标准 Magisk 更新二进制
- ` + "`META-INF/com/google/android/updater-script`" + ` — 仅包含 ` + "`#MAGISK`" + `
- ` + "`action.sh`" + ` — KSU/APatch 操作脚本（可选，用于 WebUI 或操作按钮）
- ` + "`webroot/`" + ` — WebUI 资源目录（可选，用于图形界面模块）

## MODULE.PROP 要求
module.prop 必须包含以下字段：
- id, name, version, versionCode, author, description（标准 Magisk 字段）
- ksu.supported=true（KernelSU 兼容性）
- apatch.supported=true（APatch 兼容性）

## CUSTOMIZE.SH 要求
customize.sh 必须检测运行环境：
- If $KSU is set → KernelSU environment
- If $APATCH is set → APatch environment
- Otherwise → Magisk environment
- Use ui_print and abort for user feedback in all environments

## 输出格式
返回一个包含 "files" 数组的 JSON 对象。每个元素包含：
- ` + "`path`" + `: 相对文件路径（如 "module.prop", "customize.sh"）
- ` + "`content`" + `: 完整的文件内容字符串

## 关键规则
1. ALWAYS include module.prop — it's required for the module to work
2. ALWAYS use ` + "`set -euo pipefail`" + ` in shell scripts
3. NEVER use ` + "`chmod 777`" + `
4. NEVER hardcode credentials
5. ALWAYS use ` + "`ui_print`" + ` for user messages and ` + "`abort`" + ` for errors
6. Generate COMPLETE, RUNNABLE code — no placeholders, no "add your code here"
7. The module MUST work on ALL THREE platforms: Magisk, KernelSU, APatch`

	case "chat":
		return `你是一位专业的Android模块开发助手，帮助开发者创建、调试和优化 Android 系统模块，兼容 Magisk、KernelSU 和 APatch。

## 你的专业领域
- Magisk 模块开发（systemless 修改、SELinux、属性系统）
- KernelSU (KSU) 模块开发与兼容性
- APatch 模块开发与内核补丁
- Android shell 脚本编程（bash/sh、toybox、busybox）
- Android 系统架构（init、zygote、system_server）
- SELinux 策略与文件上下文
- Android 属性系统（build.prop、system.prop、default.prop）
- 模块签名、验证与安全
- 移动设备性能优化

## 回答指南
- 提供完整可运行的代码示例，而不是伪代码
- 解释你建议的任何代码的安全影响
- 在相关时考虑 Magisk、KSU 和 APatch 的兼容性
- 安装脚本中使用 ` + "`ui_print`" + ` 和 ` + "`abort`" + `，不要用 raw echo/exit
- 始终包含错误处理（` + "`set -euo pipefail`" + `、` + "`|| abort \"...\"`" + `）
- 推荐模块大小和性能的最佳实践
- 调试时，询问具体的错误信息和相关代码

## 模块结构推荐
当用户描述他们想创建的模块时，分析需求并推荐合适的文件结构。在回复末尾包含一个带有 "recommended_files" 数组的 JSON 块：
JSON format: {"recommended_files": [{"path": "module.prop", "required": true, "description": "Module metadata (id, name, version, author, description)"}, {"path": "customize.sh", "required": true, "description": "Installation script"}, {"path": "service.sh", "required": false, "description": "Late_start service daemon"}, {"path": "post-fs-data.sh", "required": false, "description": "Post-fs-data script for systemless overlays"}, {"path": "system.prop", "required": false, "description": "System property overrides"}, {"path": "uninstall.sh", "required": false, "description": "Uninstall cleanup script"}, {"path": "action.sh", "required": false, "description": "KSU/APatch action button script"}]}

Magisk/KSU/APatch 模块的常见文件：
- module.prop (必需) — 模块元数据
- customize.sh (必需) — 安装钩子
- service.sh — 在 late_start service 模式运行
- post-fs-data.sh — 在 post-fs-data 模式运行，用于 systemless 覆盖
- system.prop — 系统属性修改
- sepolicy.rule — SELinux 策略补丁
- uninstall.sh — 模块卸载清理
- action.sh — KernelSU/APatch 操作按钮支持
- WebUI 资源 (webroot/) — 用于带图形界面的模块

## 回复格式
- 简洁但全面
- 使用带语言标签的代码块（` + "```sh, ```bash, ```properties" + `）
- 建议文件修改时，显示完整的文件内容
- 复杂主题拆分为编号步骤
- 当用户想要创建模块时，始终以 recommended_files JSON 块结尾`

	case "repair":
		return `你是一位专业的Android模块构建日志分析专家，专注于诊断 Magisk、KSU 和 APatch 模块构建失败问题。

## 常见问题检查清单
1. Shell 脚本语法错误（缺少引号、未转义的变量、错误的运算符）
2. SELinux 上下文问题（错误的文件上下文、缺少 restorecon）
3. 权限错误（错误的 chmod、缺少执行位）
4. Module.prop 格式错误（无效字符、缺少必填字段）
5. 路径错误（错误的 Magisk 覆盖路径、不正确的系统挂载点）
6. 依赖问题（缺少二进制文件、错误的架构）
7. Zip 结构问题（缺少 META-INF、错误的目录布局）
8. Android API 兼容性问题

## 诊断方法
1. Read the build log carefully — find the FIRST error (often cascading)
2. Identify the exact file and line causing the issue
3. Explain WHY the error occurs (root cause, not just symptoms)
4. Provide a SPECIFIC fix with the corrected code
5. Suggest preventive measures

## 输出格式
按以下结构组织回复：
1. **错误摘要** — 主要失败的一行描述
2. **根本原因** — 错误发生的原因
3. **修复方案** — 需要的确切代码更改（显示修改前后）
4. **验证方法** — 如何确认修复有效
5. **预防措施** — 如何避免此类错误再次发生`

	default:
		return ""
	}
}

// loadPrompt 从数据库加载提示词，先查用户自定义，再查全局默认
func (s *AIService) loadPrompt(mode, userID string) string {
	if s.db == nil {
		return defaultSystemPrompt(mode)
	}
	// Try user-specific first
	if userID != "" {
		var content string
		err := s.db.QueryRow(
			`SELECT content FROM ai_prompts WHERE mode=? AND user_id=?`, mode, userID,
		).Scan(&content)
		if err == nil {
			return content
		}
	}
	// Fall back to global default (user_id='')
	var content string
	err := s.db.QueryRow(
		`SELECT content FROM ai_prompts WHERE mode=? AND user_id=''`, mode,
	).Scan(&content)
	if err != nil {
		return defaultSystemPrompt(mode)
	}
	return content
}

// ensurePromptsTable 确保 ai_prompts 表存在，不存在则创建
func (s *AIService) ensurePromptsTable() error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS ai_prompts (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		mode       TEXT NOT NULL,
		user_id    TEXT NOT NULL DEFAULT '',
		content    TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE(mode, user_id)
	)`)
	if err != nil {
		return err
	}
	// Insert default rows if table is empty
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('generate', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('chat', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('repair', '')`)
	return nil
}

// GetPrompts 返回提示词。如果指定了用户ID，优先返回用户自定义的，全局默认作为后备
func (s *AIService) GetPrompts(userID string) ([]domain.AIPrompt, error) {
	if s.db == nil {
		return defaultPrompts(), nil
	}

	if err := s.ensurePromptsTable(); err != nil {
		return defaultPrompts(), nil
	}

	// Query: get global defaults + user-specific overrides
	rows, err := s.db.Query(
		`SELECT id, mode, user_id, content, updated_at FROM ai_prompts WHERE user_id='' OR user_id=? ORDER BY mode, user_id`,
		userID,
	)
	if err != nil {
		return defaultPrompts(), nil
	}
	defer rows.Close()

	type promptRow struct {
		domain.AIPrompt
		rowUserID string
	}
	var allRows []promptRow
	for rows.Next() {
		var p promptRow
		if err := rows.Scan(&p.ID, &p.Mode, &p.rowUserID, &p.Content, &p.UpdatedAt); err != nil {
			continue
		}
		allRows = append(allRows, p)
	}

	// Merge: user-specific overrides global defaults
	merged := make(map[string]domain.AIPrompt)
	for _, r := range allRows {
		if r.rowUserID != "" && r.rowUserID == userID {
			// User-specific row takes priority
			merged[r.Mode] = r.AIPrompt
		} else if r.rowUserID == "" {
			// Global default — only use if no user override exists
			if _, has := merged[r.Mode]; !has {
				merged[r.Mode] = r.AIPrompt
			}
		}
	}

	// Ensure all three modes exist, fill empty content with defaults
	modes := []string{"generate", "chat", "repair"}
	var prompts []domain.AIPrompt
	for _, m := range modes {
		if p, ok := merged[m]; ok {
			if p.Content == "" {
				def := defaultSystemPrompt(m)
				if def != "" {
					p.Content = def
				}
			}
			prompts = append(prompts, p)
		} else {
			def := defaultSystemPrompt(m)
			if def != "" {
				prompts = append(prompts, domain.AIPrompt{Mode: m, Content: def})
			}
		}
	}
	return prompts, nil
}

func defaultPrompts() []domain.AIPrompt {
	var prompts []domain.AIPrompt
	for _, m := range []string{"generate", "chat", "repair"} {
		prompts = append(prompts, domain.AIPrompt{Mode: m, Content: defaultSystemPrompt(m)})
	}
	return prompts
}

// UpdatePrompt 更新指定模式的提示词（用户自定义）
func (s *AIService) UpdatePrompt(mode, content, userID string) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	if userID == "" {
		return fmt.Errorf("user_id required")
	}
	if err := s.ensurePromptsTable(); err != nil {
		return fmt.Errorf("failed to ensure prompts table: %w", err)
	}
	_, err := s.db.Exec(
		`INSERT INTO ai_prompts (mode, user_id, content, updated_at) VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(mode, user_id) DO UPDATE SET content=?, updated_at=datetime('now')`,
		mode, userID, content, content,
	)
	return err
}

// ResetPrompt 删除当前用户的自定义提示词（恢复到全局默认）
func (s *AIService) ResetPrompt(mode, userID string) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	if userID == "" {
		return fmt.Errorf("user_id required")
	}
	_, err := s.db.Exec(
		`DELETE FROM ai_prompts WHERE mode=? AND user_id=?`, mode, userID,
	)
	return err
}

// GenerateModule 用 LLM 生成模块代码，SSE 流式返回
func (s *AIService) GenerateModule(ctx context.Context, description, userID string, c fiber.Ctx) error {
	systemPrompt := s.loadPrompt("generate", userID)

	userPrompt := fmt.Sprintf(`Create a universal module (compatible with Magisk / KernelSU / APatch).

Module Description: %s

Generate all necessary files as a JSON object with "files" array (each with "path" and "content").
Ensure the module.prop includes both ksu.supported=true and apatch.supported=true.
Ensure all shell scripts have proper shebang, error handling, and follow security best practices.`, description)

	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, c)
}

// Chat 通用 AI 对话
func (s *AIService) Chat(ctx context.Context, message, contextInfo, userID string, c fiber.Ctx) error {
	systemPrompt := s.loadPrompt("chat", userID)

	userPrompt := message
	if contextInfo != "" {
		userPrompt = fmt.Sprintf("Context:\n%s\n\nQuestion:\n%s", contextInfo, message)
	}
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, c)
}

// RepairBuild 分析构建日志给出修复建议
func (s *AIService) RepairBuild(ctx context.Context, buildLog, userID string, c fiber.Ctx) error {
	systemPrompt := s.loadPrompt("repair", userID)

	userPrompt := fmt.Sprintf("Analyze this Android module build log and identify the failure:\n\n```\n%s\n```\n\nProvide diagnosis with specific fix instructions.", buildLog)
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, c)
}

// resolveUserProviderConfig 查询用户自定义的 endpoint 和 api_key
func (s *AIService) resolveUserProviderConfig(userID, providerID string) (endpoint, apiKey string) {
	if s.db == nil || userID == "" {
		return "", ""
	}
	var dbEndpoint, dbAPIKey string
	err := s.db.QueryRow(
		`SELECT COALESCE(endpoint,''), COALESCE(api_key,'') FROM provider_configs WHERE user_id=? AND id=?`, userID, providerID,
	).Scan(&dbEndpoint, &dbAPIKey)
	if err == nil {
		if dbAPIKey != "" {
			if b, err := base64.StdEncoding.DecodeString(dbAPIKey); err == nil {
				dbAPIKey = string(b)
			}
		}
		return dbEndpoint, dbAPIKey
	}

	// Check custom providers
	var customEndpoint, customKey string
	err = s.db.QueryRow(
		`SELECT COALESCE(endpoint,''), COALESCE(api_key,'') FROM custom_providers WHERE user_id=? AND id=?`, userID, providerID,
	).Scan(&customEndpoint, &customKey)
	if err == nil {
		if customKey != "" {
			if b, err := base64.StdEncoding.DecodeString(customKey); err == nil {
				customKey = string(b)
			}
		}
		return customEndpoint, customKey
	}

	return "", ""
}

// streamChatWithSystem 使用 system + user 双消息结构发起流式请求
// 直接管道透传 LLM 响应字节，前端负责解析 SSE 格式
func (s *AIService) streamChatWithSystem(ctx context.Context, systemPrompt, userPrompt string, c fiber.Ctx) error {
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, "", c)
}

// streamChatWithSystemForUser 支持用户 specific LLM 配置的流式请求
func (s *AIService) streamChatWithSystemForUser(ctx context.Context, systemPrompt, userPrompt, userID string, c fiber.Ctx) error {
	endpoint := s.cfg.LLMEndpoint
	apiKey := s.cfg.LLMApiKey
	model := s.cfg.LLMModel
	providerID := s.cfg.LLMProvider

	// Override with user-specific provider config
	if userID != "" && providerID != "" {
		userEndpoint, userKey := s.resolveUserProviderConfig(userID, providerID)
		if userEndpoint != "" {
			endpoint = userEndpoint
		}
		if userKey != "" {
			apiKey = userKey
		}
	}

	// Check if provider needs a key; some providers (Ollama, OpenCode free tier) work without one
	providerNeedsKey := true
	if providerID != "" {
		provider := llm.FindProvider(providerID)
		if provider != nil {
			providerNeedsKey = provider.RequiresKey
		}
	}

	if providerNeedsKey && apiKey == "" {
		_, err := c.Write([]byte("data: " + `{"content":"LLM not configured. Set API key in Settings to enable AI features.\n\nThe architecture is ready for your module files:\n- module.prop: module metadata\n- system/: system file overrides\n- META-INF/: update-binary + updater-script\n- customize.sh: installation hooks\n\nEdit files in the editor tab to build your module."}` + "\n\ndata: [DONE]\n\n"))
		return err
	}

	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream": true,
	}
	bodyBytes, _ := json.Marshal(body)

	chatURL := endpoint
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		chatURL = endpoint + "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		errEvt, _ := json.Marshal(map[string]string{"type": "error", "error": fmt.Sprintf("AI 服务连接失败: %s。请检查 LLM API Key 和网络连接。", err.Error())})
		_, werr := c.Write([]byte("data: " + string(errEvt) + "\n\ndata: [DONE]\n\n"))
		return werr
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("LLM 请求失败 (HTTP %d)", resp.StatusCode)
		var errBody struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &errBody) == nil && errBody.Error.Message != "" {
			errMsg = errBody.Error.Message
		}
		errEvt, _ := json.Marshal(map[string]string{"type": "error", "error": errMsg})
		_, werr := c.Write([]byte("data: " + string(errEvt) + "\n\ndata: [DONE]\n\n"))
		return werr
	}

	// 逐行解析 LLM SSE 响应，添加步骤事件和思考过程事件
	sentSteps := map[string]bool{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// 非 data: 行直接透传（空行等）
		if !strings.HasPrefix(line, "data: ") {
			if _, werr := c.Write([]byte(line + "\n")); werr != nil {
				break
			}
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// 发送完成步骤事件
			if !sentSteps["done"] {
				doneEvt, _ := json.Marshal(map[string]string{"type": "step", "step": "done", "message": "生成完成！"})
				c.Write([]byte("data: " + string(doneEvt) + "\n\n"))
				sentSteps["done"] = true
			}
			c.Write([]byte("data: [DONE]\n\n"))
			break
		}

		// 尝试解析 JSON delta
		var parsed struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &parsed); err == nil && len(parsed.Choices) > 0 {
			delta := parsed.Choices[0].Delta

			// 发送思考过程事件
			if delta.ReasoningContent != "" {
				reasoningEvt, _ := json.Marshal(map[string]string{"type": "reasoning", "content": delta.ReasoningContent})
				c.Write([]byte("data: " + string(reasoningEvt) + "\n\n"))
			}

			// 发送步骤事件（基于内容关键词检测）
			if delta.Content != "" {
				// 发送开始事件
				if !sentSteps["start"] {
					startEvt, _ := json.Marshal(map[string]string{"type": "step", "step": "start", "message": "正在连接AI..."})
					c.Write([]byte("data: " + string(startEvt) + "\n\n"))
					sentSteps["start"] = true
				}

				contentLower := strings.ToLower(delta.Content)

				// 检测步骤
				type stepDef struct {
					keywords []string
					message  string
				}
				stepDefs := []struct {
					step string
					def  stepDef
				}{
					{"structure", stepDef{[]string{"module.prop", "metadata", "module_id"}, "正在生成模块结构..."}},
					{"script", stepDef{[]string{"customize.sh", "install", "post-fs-data"}, "正在编写安装脚本..."}},
					{"system", stepDef{[]string{"system/", "system.prop", "build.prop"}, "正在配置系统文件..."}},
					{"optimize", stepDef{[]string{"optimize", "best practice", "security"}, "正在优化代码..."}},
				}

				for _, sd := range stepDefs {
					if !sentSteps[sd.step] {
						for _, kw := range sd.def.keywords {
							if strings.Contains(contentLower, kw) {
								stepEvt, _ := json.Marshal(map[string]string{"type": "step", "step": sd.step, "message": sd.def.message})
								c.Write([]byte("data: " + string(stepEvt) + "\n\n"))
								sentSteps[sd.step] = true
								break
							}
						}
					}
				}
			}
		}

		// 透传原始 SSE 行
		if _, werr := c.Write([]byte(line + "\n")); werr != nil {
			break
		}
	}

	// 流中途断开：检查是否正常结束
	if !sentSteps["done"] {
		if err := scanner.Err(); err != nil {
			errEvt, _ := json.Marshal(map[string]string{"type": "error", "error": fmt.Sprintf("AI 流式响应中断: %s", err.Error())})
			c.Write([]byte("data: " + string(errEvt) + "\n\n"))
		} else {
			// 正常结束但没收到 [DONE]，补一个完成事件
			doneEvt, _ := json.Marshal(map[string]string{"type": "step", "step": "done", "message": "生成完成！"})
			c.Write([]byte("data: " + string(doneEvt) + "\n\n"))
		}
		c.Write([]byte("data: [DONE]\n\n"))
	}

	return nil
}
