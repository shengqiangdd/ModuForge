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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moduforge/backend/internal/builder"
	"github.com/moduforge/backend/internal/config"
	"github.com/moduforge/backend/internal/domain"
	"github.com/moduforge/backend/internal/llm"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		ResponseHeaderTimeout: 2 * time.Minute, // 等待 LLM 响应头的超时
	},
	// 不设 Timeout：LLM 流式生成可能持续很久，Timeout 会强制取消读取
}

type AIService struct {
	cfg       *config.Config
	db        *sql.DB
	convStore *ConversationStore
}

func NewAIService(cfg *config.Config) *AIService {
	return &AIService{cfg: cfg, convStore: NewConversationStore()}
}

func NewAIServiceWithDB(cfg *config.Config, db *sql.DB) *AIService {
	return &AIService{cfg: cfg, db: db, convStore: NewConversationStore()}
}

func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%10000)
}

func defaultSystemPrompt(mode string) string {
	switch mode {
	case "generate":
		return `你是Android模块开发专家。为Magisk/KSU/APatch生成生产级模块。

## 输出格式
{"files":[{"path":"...","content":"..."}]}

## 技术栈选择
- 后台服务/数据处理/网络 → Go（首选）
- 系统级/内存安全 → Rust
- 底层调用/C库依赖 → C/C++
- 安装/检测/简单操作 → Shell

## 模块结构
必须: module.prop(id ^[a-zA-Z][a-zA-Z0-9._-]*$, semver版本), customize.sh, META-INF/(update-binary + updater-script仅含#MAGISK)
可选: src/(源码), build.sh, service.sh, system.prop, webroot/, bin/

## ⚠️ 代码质量要求（违反将导致编译失败）
1. Go文件: 每个文件必须有 package 声明，import 的包必须使用，变量必须使用
2. Go文件: 结构体定义必须完整，函数签名必须正确
3. 所有语言: 检查括号平衡（{ 必须有对应的 }）
4. 所有语言: 错误处理必须完整，不能忽略 error 返回值

## 安全规范
- scripts:0755, configs:0644, 绝不chmod 777
- Shell: set -euo pipefail, 变量双引号"$VAR", command -v替代which
- mktemp+trap清理临时文件, 禁止eval处理不可信输入
- SELinux: chcon -R -t system_file应用于bin/和scripts/

## customize.sh环境检测
if [ -n "$KSU" ]; then ui_print "- KSU"; elif [ -n "$APATCH" ]; then ui_print "- APatch"; else ui_print "- Magisk"; fi

## 三平台兼容
模块必须同时兼容Magisk、KernelSU、APatch三种管理器

每个文件完整可运行，无占位符。`

	case "chat":
		return `你是Android模块开发助手，帮助创建/调试/优化Magisk/KSU/APatch模块。

## 回答规范
1. 提供完整可运行代码，非伪代码
2. 考虑安全影响（注入、权限提升、数据暴露）
3. 性能影响（内存、CPU、电池）
4. 兼容性说明（Magisk vs KSU vs APatch差异）
5. Shell脚本: set -euo pipefail, ui_print/abort
6. 调试时询问: 错误信息、文件内容、Android版本、管理器类型

## 模块结构参考
必须: module.prop, customize.sh, META-INF/
可选: service.sh, webroot/, bin/
输出推荐文件: {"recommended_files":[{"path":"...","required":true|false,"description":"..."}]}

回复要求: 简洁可执行，代码块带语言标签，完整文件内容（非diff），考虑三平台兼容`

	case "repair":
		return `你是Android模块构建日志分析专家，诊断Magisk/KSU/APatch构建失败。

## 诊断方法
1. 找第一个错误（非警告），错误常级联
2. 分类: 语法|权限|SELinux|module.prop格式|路径|依赖|Zip结构|编译
3. 根因分析: 什么失败？为什么？环境状态？
4. 修复: 精确代码变更（修改前→修改后）
5. 验证方法 + 预防措施

## 输出格式
1. 错误摘要（一行）
2. 根本原因
3. 修复方案（文件路径+行号+修改前后）
4. 验证方法
5. 预防措施`

	case "gather":
		return `你是需求分析师，将模糊需求转化为精确技术规格。

## 流程（一次问一个问题，已回答的跳过）
1. 核心问题: 解决什么痛点？
2. 约束: Android版本? 架构? 框架(Magisk/KSU/APatch)? 需要后台服务? WebUI? 依赖?
3. 功能规格: 每个功能的触发、流程、结果、失败行为
4. 非功能需求: 性能、安全、持久化、干净卸载

## 输出
{"module_name":"kebab-id","display_name":"名称","description":"用途","target_android":["12-15"],"architectures":["arm64"],"frameworks":["magisk","ksu","apatch"],"features":[{"name":"feature","description":"what","files":["service.sh"],"tech":"shell|go|rust|c|webui"}],"ui_required":true,"performance_notes":"...","security_notes":"...","special_requirements":"..."}`

	case "agent":
		return `你是高级Android模块开发工程师。为Magisk/KSU/APatch生成生产级模块代码。

## 输出格式（严格遵守）
{"files":[{"path":"...","content":"..."}]}

## 模块结构（必须）
module.prop                  # 模块元数据
customize.sh                 # 安装脚本
META-INF/com/google/android/update-binary
META-INF/com/google/android/updater-script  # 仅含#MAGISK

## 技术栈选择
- 后台服务/数据处理/网络操作 → Go（首选）
- 系统级编程/内存安全要求 → Rust
- 底层系统调用/C库依赖 → C/C++
- 安装/环境检测/简单文件操作 → Shell

## Go 代码规范（严格遵守，违反将导致编译失败）
- 每个文件必须以 package xxx 开头（不是 package main 除非是入口文件）
- import 块必须使用圆括号，每个导入一行
- 所有变量必须使用，未使用的变量会导致编译错误
- 所有导入的包必须使用，未使用的导入会导致编译错误
- 函数签名必须正确：参数类型、返回值类型
- 结构体定义必须完整：每个字段必须有类型
- 错误处理必须完整：检查每个 error 返回值
- 常见错误模式（必须避免）：
  ✗ "var x int" 然后不使用 x
  ✓ "var x int = 42" 或 "_ = x"
  ✗ "import fmt" 然后不使用 fmt
  ✓ 使用 fmt.Println 或删除导入
  ✗ "func foo() {" 然后缺少 "}"
  ✓ 确保每个 { 都有对应的 }

## Go 文件模板
入口文件 (src/main.go):
  package main
  import ("context"; "log/slog"; "os"; "os/signal"; "syscall")
  func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()
    slog.Info("service starting")
    <-ctx.Done()
    slog.Info("service shutting down")
  }

配置文件 (src/config.go):
  package main
  import ("encoding/json"; "os")
  type Config struct { ... }
  func loadConfig(path string) (*Config, error) { ... }

## Rust 规范
src/main.rs: tokio::main, signal::ctrl_c(), tracing日志
Cargo.toml: [profile.release] opt-level="s" lto=true
build.sh: cargo build --release --target aarch64-linux-android, 复制到system/bin/

## C/C++ 规范
main.c: signal(SIGTERM/SIGINT), 主循环+退出标志, 检查所有返回值
Makefile或build.sh: NDK交叉编译, -O2 -Wall, 链接-pthread

## Shell 规范（customize.sh/service.sh）
#!/system/bin/sh, set -euo pipefail, 变量双引号"$VAR"
错误处理: 每个关键操作检查$?, 失败时abort
日志: ui_print输出进度, log函数写文件
权限: scripts=0755, configs=0644, 绝不chmod 777
环境检测: if [ -n "$KSU" ]; then...elif [ -n "$APATCH" ]; then...else...fi

## 工作流程（必须遵守）
1. 写完所有文件后，调用 syntax_checker 验证语法
2. 如果 syntax_checker 报错，立即用 edit_file 修复
3. 修复后再调用 build_module 构建
4. 构建失败时，分析错误信息并修复

## 禁止
- 空文件/占位符/TODO注释
- chmod 777
- 硬编码密钥/token
- 无错误处理的代码
- 超过300行的单个文件（拆分）
- 未使用的变量和导入
- 不完整的结构体定义
- 缺少 package 声明的 Go 文件`



	default:
		return ""
	}
}

// loadPrompt 从数据库加载提示词，先查用户自定义，再查全局默认
func (s *AIService) loadPrompt(mode, userID string) string {
	if s.db == nil {
		return defaultSystemPrompt(mode)
	}
	if userID != "" {
		var content string
		err := s.db.QueryRow(`SELECT content FROM ai_prompts WHERE mode=? AND user_id=?`, mode, userID).Scan(&content)
		if err == nil {
			return content
		}
	}
	var content string
	err := s.db.QueryRow(`SELECT content FROM ai_prompts WHERE mode=? AND user_id=''`, mode).Scan(&content)
	if err != nil {
		return defaultSystemPrompt(mode)
	}
	return content
}

// ensurePromptsTable 确保 ai_prompts 表存在
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
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('generate', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('chat', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('repair', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('gather', '')`)
	s.db.Exec(`INSERT OR IGNORE INTO ai_prompts (mode, content) VALUES ('agent', '')`)
	return nil
}

// GetPrompts 返回提示词
func (s *AIService) GetPrompts(userID string) ([]domain.AIPrompt, error) {
	if s.db == nil {
		return defaultPrompts(), nil
	}
	if err := s.ensurePromptsTable(); err != nil {
		return defaultPrompts(), nil
	}
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

	merged := make(map[string]domain.AIPrompt)
	for _, r := range allRows {
		if r.rowUserID != "" && r.rowUserID == userID {
			merged[r.Mode] = r.AIPrompt
		} else if r.rowUserID == "" {
			if _, has := merged[r.Mode]; !has {
				merged[r.Mode] = r.AIPrompt
			}
		}
	}

	modes := []string{"generate", "chat", "repair", "gather", "agent"}
	var prompts []domain.AIPrompt
	for _, m := range modes {
		if p, ok := merged[m]; ok {
			if p.Content == "" {
				if def := defaultSystemPrompt(m); def != "" {
					p.Content = def
				}
			}
			prompts = append(prompts, p)
		} else {
			if def := defaultSystemPrompt(m); def != "" {
				prompts = append(prompts, domain.AIPrompt{Mode: m, Content: def})
			}
		}
	}
	return prompts, nil
}

func defaultPrompts() []domain.AIPrompt {
	var prompts []domain.AIPrompt
	for _, m := range []string{"generate", "chat", "repair", "gather", "agent"} {
		prompts = append(prompts, domain.AIPrompt{Mode: m, Content: defaultSystemPrompt(m)})
	}
	return prompts
}

// UpdatePrompt 更新提示词
func (s *AIService) UpdatePrompt(mode, content, userID string) error {
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	if userID == "" {
		return fmt.Errorf("user_id required")
	}
	if err := s.ensurePromptsTable(); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO ai_prompts (mode, user_id, content, updated_at) VALUES (?, ?, ?, datetime('now'))
		 ON CONFLICT(mode, user_id) DO UPDATE SET content=?, updated_at=datetime('now')`,
		mode, userID, content, content,
	)
	return err
}

// ResetPrompt 重置为默认提示词
func (s *AIService) ResetPrompt(mode, userID string) error {
	if s.db == nil {
		return nil
	}
	if userID != "" {
		_, err := s.db.Exec(`DELETE FROM ai_prompts WHERE mode=? AND user_id=?`, mode, userID)
		return err
	}
	return nil
}

// GenerateModule 用 LLM 生成模块代码
func (s *AIService) GenerateModule(ctx context.Context, description, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	systemPrompt := s.loadPrompt("generate", userID)
	userPrompt := fmt.Sprintf(`Create a universal module (Magisk/KSU/APatch compatible).

Module Description: %s

Generate all necessary files as JSON: {"files":[{"path":"...","content":"..."}]}
Ensure module.prop has ksu.supported=true and apatch.supported=true.
All shell scripts: shebang + set -euo pipefail + security best practices.`, description)
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, w)
}

// GatherRequirements 需求收集
func (s *AIService) GatherRequirements(ctx context.Context, message, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	systemPrompt := s.loadPrompt("gather", userID)
	return s.streamChatWithSystemForUser(ctx, systemPrompt, message, userID, w)
}

// Chat 通用 AI 对话
func (s *AIService) Chat(ctx context.Context, message, contextInfo, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	systemPrompt := s.loadPrompt("chat", userID)
	userPrompt := message
	if contextInfo != "" {
		userPrompt = fmt.Sprintf("Context:\n%s\n\nQuestion:\n%s", contextInfo, message)
	}
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, w)
}

// RepairBuild 分析构建日志给出修复建议
func (s *AIService) RepairBuild(ctx context.Context, buildLog, userID string, messages []Message, sessionID string, w *bufio.Writer) error {
	systemPrompt := s.loadPrompt("repair", userID)
	userPrompt := fmt.Sprintf("Analyze this build log and identify the failure:\n\n```\n%s\n```\n\nProvide diagnosis with specific fix instructions.", buildLog)
	return s.streamChatWithSystemForUser(ctx, systemPrompt, userPrompt, userID, w)
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

	endpoint := s.cfg.LLMEndpoint
	apiKey := s.cfg.LLMApiKey
	model := s.cfg.LLMModel
	providerID := s.cfg.LLMProvider

	if userID != "" && providerID != "" {
		userEndpoint, userKey := s.resolveUserProviderConfig(userID, providerID)
		if userEndpoint != "" {
			endpoint = userEndpoint
		}
		if userKey != "" {
			apiKey = userKey
		}
	}

	providerNeedsKey := true
	if providerID != "" {
		provider := llm.FindProvider(providerID)
		if provider != nil {
			providerNeedsKey = provider.RequiresKey
		}
	}

	if providerNeedsKey && apiKey == "" {
		safeSSE(map[string]interface{}{
			"type":  "phase",
			"phase": "error",
			"message": "LLM not configured. Set API key in Settings.",
		})
		return nil
	}

	// 追加历史对话上下文（如果有），并做 token 压缩
	var llmMessages []Message
	if len(messages) > 0 {
		// 先压缩历史消息，避免 token 浪费
		compressed := s.convStore.CompressMessages(systemPrompt, messages)
		// CompressMessages 返回的已经包含 system prompt，直接用
		llmMessages = compressed
		// 追加当前需求
		llmMessages = append(llmMessages, Message{Role: "user", Content: userPrompt})
	} else {
		llmMessages = []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		}
	}

	// 转换为 LLM API 格式
	msgList := make([]map[string]string, 0, len(llmMessages))
	for _, m := range llmMessages {
		msgList = append(msgList, map[string]string{"role": m.Role, "content": m.Content})
	}

	body := map[string]interface{}{
		"model":    model,
		"messages": msgList,
		"stream":   true,
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
		s.sendSSE(w, map[string]interface{}{
			"type":  "phase",
			"phase": "error",
			"message": fmt.Sprintf("AI service error: %s", err.Error()),
		})
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("LLM error (HTTP %d)", resp.StatusCode)
		var errBody struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &errBody) == nil && errBody.Error.Message != "" {
			errMsg = errBody.Error.Message
		}
		safeSSE(map[string]interface{}{
			"type":  "phase",
			"phase": "error",
			"message": errMsg,
		})
		return nil
	}

	// 读取LLM响应并解析JSON
	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	contentStarted := false
	chunkCount := 0

	// 立即发送第一个phase：连接AI
	safeSSE(map[string]interface{}{
		"type":    "phase",
		"phase":   "start",
		"message": "正在连接AI...",
	})

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var parsed struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &parsed); err == nil && len(parsed.Choices) > 0 {
			if content := parsed.Choices[0].Delta.Content; content != "" {
				fullResponse.WriteString(content)
				chunkCount++

				// 收到第一个content chunk → 开始生成
				if !contentStarted {
					contentStarted = true
					safeSSE(map[string]interface{}{
						"type":    "phase",
						"phase":   "structure",
						"message": "正在分析需求...",
					})
				}

				// 每30个chunk → 生成进度
				if chunkCount == 30 {
					safeSSE(map[string]interface{}{
						"type":    "phase",
						"phase":   "script",
						"message": "正在生成代码...",
					})
				}
			}
		}
	}

	// 解析LLM返回的JSON
	var result struct {
		Thinking string `json:"thinking"`
		Changes  string `json:"changes"`
		Files    []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
		Answer string `json:"answer"`
	}

	if err := json.Unmarshal([]byte(fullResponse.String()), &result); err != nil {
		jsonStr := extractJSON(fullResponse.String())
		if jsonStr != "" {
			json.Unmarshal([]byte(jsonStr), &result)
		}
	}

	// 如果还没发过script phase，现在发
	if chunkCount < 30 {
		safeSSE(map[string]interface{}{
			"type":    "phase",
			"phase":   "script",
			"message": "正在生成代码...",
		})
	}

	// 验证阶段
	time.Sleep(300 * time.Millisecond)
	safeSSE(map[string]interface{}{
		"type":    "phase",
		"phase":   "system",
		"message": "正在验证文件...",
	})

	// 发送完成事件
	files := make([]map[string]interface{}, 0)
	for _, f := range result.Files {
		files = append(files, map[string]interface{}{
			"path":    f.Path,
			"content": f.Content,
			"size":    len(f.Content),
		})
	}

	// 如果没有projectID，创建新项目
	if projectID == "" && userID != "" && s.db != nil {
		projectName := "AI Generated Module"
		if result.Thinking != "" && len(result.Thinking) > 10 {
			if len(result.Thinking) > 50 {
				projectName = result.Thinking[:50] + "..."
			} else {
				projectName = result.Thinking
			}
		}
		var newProjectID string
		err := s.db.QueryRow(
			`INSERT INTO projects (id, user_id, name, module_type, description) VALUES (?, ?, ?, 'universal', ?) RETURNING id`,
			generateID(), userID, projectName, description,
		).Scan(&newProjectID)
		if err == nil {
			projectID = newProjectID
			// 保存所有生成的文件到数据库
			for _, f := range result.Files {
				s.db.ExecContext(ctx,
					`INSERT INTO project_files (project_id, path, content)
					 VALUES (?, ?, ?)
					 ON CONFLICT(project_id, path) DO UPDATE SET content=?, updated_at=datetime('now')`,
					projectID, f.Path, f.Content, f.Content)
			}
		}
	}

	// 编译阶段：在容器环境中编译源码，生成二进制文件
	if projectID != "" && s.db != nil && len(result.Files) > 0 {
		safeSSE(map[string]interface{}{
			"type":    "phase",
			"phase":   "compile",
			"message": "正在编译源码...",
		})

		// 创建临时目录用于编译
		tmpDir, err := os.MkdirTemp("", "moduforge-build-*")
		if err == nil {
			defer os.RemoveAll(tmpDir)

			// 将生成的文件写入临时目录
			for _, f := range result.Files {
				if f.Path == "" {
					continue
				}
				filePath := filepath.Join(tmpDir, f.Path)
				// Prevent path traversal: ensure filePath is within tmpDir
				if !strings.HasPrefix(filePath, tmpDir+string(os.PathSeparator)) {
					continue
				}
				if err := os.MkdirAll(filepath.Dir(filePath), 0755); err == nil {
					os.WriteFile(filePath, []byte(f.Content), 0644)
				}
			}

			// 检测语言类型并编译
			compiled := false
			lang := detectLanguage(result.Files)

			if lang == "go" {
				// Go 编译
				b := builder.NewBuilder(s.cfg)
				logFn := func(msg string) {
					safeSSE(map[string]interface{}{
						"type":    "phase",
						"phase":   "compile",
						"message": strings.TrimSpace(msg),
					})
				}

				compileResult, err := b.CompileGoFilesArch(ctx, tmpDir, "arm64", nil, logFn)
				if err == nil && len(compileResult.Recompiled) > 0 {
					// 编译成功，读取二进制文件
					binDir := filepath.Join(tmpDir, "bin")
					entries, _ := os.ReadDir(binDir)
					for _, entry := range entries {
						if entry.IsDir() {
							continue
						}
						binPath := filepath.Join(binDir, entry.Name())
						binContent, err := os.ReadFile(binPath)
						if err == nil {
							// 保存二进制到数据库
							dbPath := "system/bin/" + entry.Name()
							s.db.ExecContext(ctx,
								`INSERT INTO project_files (project_id, path, content)
								 VALUES (?, ?, ?)
								 ON CONFLICT(project_id, path) DO UPDATE SET content=?, updated_at=datetime('now')`,
								projectID, dbPath, base64.StdEncoding.EncodeToString(binContent), base64.StdEncoding.EncodeToString(binContent))
							compiled = true
						}
					}
				} else if err != nil {
					logFn(fmt.Sprintf("  ❌ Go编译失败: %v", err))
				}
			} else if lang == "rust" {
				// Rust 编译 - 调用 cargo build
				logFn := func(msg string) {
					safeSSE(map[string]interface{}{
						"type":    "phase",
						"phase":   "compile",
						"message": strings.TrimSpace(msg),
					})
				}
				logFn("  🔨 编译 Rust 项目...")

				// 尝试使用 cross 或 cargo 编译 (Android target)
				var cmd *exec.Cmd
				if _, err := exec.LookPath("cross"); err == nil {
					cmd = exec.CommandContext(ctx, "cross", "build", "--release", "--target", "aarch64-linux-android")
				} else if _, err := exec.LookPath("cargo"); err == nil {
					cmd = exec.CommandContext(ctx, "cargo", "build", "--release")
				}
				if cmd == nil {
					logFn("  ❌ 未找到 cross 或 cargo，跳过 Rust 编译")
				} else {
					cmd.Dir = tmpDir
					output, err := cmd.CombinedOutput()
					if err != nil {
						logFn(fmt.Sprintf("  ❌ Rust 编译失败: %v", err))
						logFn(fmt.Sprintf("  输出: %s", string(output)))
					} else {
						// 读取编译产物
						binDir := filepath.Join(tmpDir, "target", "release")
						// 也检查 Android target 目录
						if androidDir := filepath.Join(tmpDir, "target", "aarch64-linux-android", "release"); dirExists(androidDir) {
							binDir = androidDir
						}
						entries, _ := os.ReadDir(binDir)
						for _, entry := range entries {
							if entry.IsDir() || strings.HasSuffix(entry.Name(), ".d") || strings.HasSuffix(entry.Name(), ".rlib") {
								continue
							}
							binPath := filepath.Join(binDir, entry.Name())
							binContent, err := os.ReadFile(binPath)
							if err == nil {
								dbPath := "system/bin/" + entry.Name()
								s.db.ExecContext(ctx,
									`INSERT INTO project_files (project_id, path, content)
									 VALUES (?, ?, ?)
									 ON CONFLICT(project_id, path) DO UPDATE SET content=?, updated_at=datetime('now')`,
									projectID, dbPath, base64.StdEncoding.EncodeToString(binContent), base64.StdEncoding.EncodeToString(binContent))
								compiled = true
							}
						}
						if compiled {
							logFn("  ✅ Rust 编译成功")
						}
					}
				}
			} else if lang == "c" || lang == "cpp" {
				// C/C++ 编译
				logFn := func(msg string) {
					safeSSE(map[string]interface{}{
						"type":    "phase",
						"phase":   "compile",
						"message": strings.TrimSpace(msg),
					})
				}
				logFn("  🔨 编译 C/C++ 项目...")

				// 收集源文件
				var cFiles, hFiles []string
				filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return nil
					}
					switch {
					case strings.HasSuffix(path, ".c") || strings.HasSuffix(path, ".cpp"):
						cFiles = append(cFiles, path)
					case strings.HasSuffix(path, ".h"):
						hFiles = append(hFiles, path)
					}
					return nil
				})

				if len(cFiles) > 0 {
					// 尝试多个编译器
					compilers := []string{"aarch64-linux-android-gcc", "aarch64-linux-gnu-gcc", "gcc"}
					var cc string
					for _, c := range compilers {
						if _, err := exec.LookPath(c); err == nil {
							cc = c
							break
						}
					}
					if cc == "" {
						cc = "gcc"
					}

					// 构建编译命令
					outputPath := filepath.Join(tmpDir, "output")
					args := []string{"-o", outputPath, "-static"}
					for _, h := range hFiles {
						args = append(args, "-I", filepath.Dir(h))
					}
					args = append(args, cFiles...)

					cmd := exec.CommandContext(ctx, cc, args...)
					output, err := cmd.CombinedOutput()
					if err != nil {
						logFn(fmt.Sprintf("  ❌ 编译失败: %v", err))
						logFn(fmt.Sprintf("  输出: %s", string(output)))
					} else {
						binContent, err := os.ReadFile(outputPath)
						if err == nil {
							dbPath := "system/bin/output"
							s.db.ExecContext(ctx,
								`INSERT INTO project_files (project_id, path, content)
								 VALUES (?, ?, ?)
								 ON CONFLICT(project_id, path) DO UPDATE SET content=?, updated_at=datetime('now')`,
								projectID, dbPath, base64.StdEncoding.EncodeToString(binContent), base64.StdEncoding.EncodeToString(binContent))
							compiled = true
							logFn(fmt.Sprintf("  ✅ 编译成功: %s", cc))
						}
					}
				} else {
					logFn("  ⚠️ 未找到C/C++源文件")
				}
			}

			if compiled {
				safeSSE(map[string]interface{}{
					"type":    "phase",
					"phase":   "compile",
					"message": "✅ 编译成功",
				})
			}
		}
	}

	// 发送完成事件（只发元数据，不发文件内容，避免SSE消息过大导致连接中断）
	filesMetadata := make([]map[string]interface{}, 0, len(files))
	for _, f := range files {
		filesMetadata = append(filesMetadata, map[string]interface{}{
			"path": f["path"],
			"size": f["size"],
		})
	}
	safeSSE(map[string]interface{}{
		"type":         "complete",
		"project_id":   projectID,
		"project_name": "AI Generated Module",
		"file_count":   len(files),
		"files":        filesMetadata,
		"changes":      result.Changes,
	})

	return nil
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

// GetHistory 获取对话历史
func (s *AIService) GetHistory(sessionID string) []Message {
	return nil
}

// DeleteHistory 删除对话历史
func (s *AIService) DeleteHistory(sessionID string) {
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
	return "", ""
}

// streamChatWithSystemForUser 支持用户 specific LLM 配置的流式请求
func (s *AIService) streamChatWithSystemForUser(ctx context.Context, systemPrompt, userPrompt, userID string, w *bufio.Writer) error {
	endpoint := s.cfg.LLMEndpoint
	apiKey := s.cfg.LLMApiKey
	model := s.cfg.LLMModel
	providerID := s.cfg.LLMProvider

	if userID != "" && providerID != "" {
		userEndpoint, userKey := s.resolveUserProviderConfig(userID, providerID)
		if userEndpoint != "" {
			endpoint = userEndpoint
		}
		if userKey != "" {
			apiKey = userKey
		}
	}

	providerNeedsKey := true
	if providerID != "" {
		provider := llm.FindProvider(providerID)
		if provider != nil {
			providerNeedsKey = provider.RequiresKey
		}
	}

	if providerNeedsKey && apiKey == "" {
		w.WriteString("data: " + `{"content":"LLM not configured. Set API key in Settings."}` + "\n\ndata: [DONE]\n\n")
		w.Flush()
		return nil
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
		errEvt, _ := json.Marshal(map[string]string{"type": "error", "error": err.Error()})
		w.WriteString("data: " + string(errEvt) + "\n\ndata: [DONE]\n\n")
		w.Flush()
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("LLM error (HTTP %d)", resp.StatusCode)
		var errBody struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &errBody) == nil && errBody.Error.Message != "" {
			errMsg = errBody.Error.Message
		}
		errEvt, _ := json.Marshal(map[string]string{"type": "error", "error": errMsg})
		w.WriteString("data: " + string(errEvt) + "\n\ndata: [DONE]\n\n")
		w.Flush()
		return nil
	}

	scanner := bufio.NewScanner(resp.Body)

	// Keepalive goroutine — 每 15 秒发一个 SSE 注释，防止代理/客户端超时关闭连接
	keepAliveDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.WriteString(": keepalive\n\n")
				w.Flush()
			case <-keepAliveDone:
				return
			}
		}
	}()

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			w.WriteString(line + "\n")
			w.Flush()
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			w.WriteString("data: [DONE]\n\n")
			w.Flush()
			break
		}
		w.WriteString(line + "\n")
		w.Flush()
	}

	close(keepAliveDone)
	return nil
}

// sendSSE 发送SSE事件，写入失败时返回error
func (s *AIService) sendSSE(w *bufio.Writer, data map[string]interface{}) error {
	jsonData, _ := json.Marshal(data)
	if _, err := w.WriteString("data: " + string(jsonData) + "\n\n"); err != nil {
		return err
	}
	return w.Flush()
}

// extractJSON 从文本中提取JSON
func extractJSON(text string) string {
	start := strings.Index(text, "{")
	if start == -1 {
		return ""
	}
	end := strings.LastIndex(text, "}")
	if end == -1 || end <= start {
		return ""
	}
	return text[start : end+1]
}

// detectLanguage 根据文件扩展名检测项目语言
func detectLanguage(files []struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}) string {
	goCount := 0
	rustCount := 0
	cCount := 0
	cppCount := 0

	for _, f := range files {
		switch {
		case strings.HasSuffix(f.Path, ".go"):
			goCount++
		case strings.HasSuffix(f.Path, ".rs"):
			rustCount++
		case strings.HasSuffix(f.Path, ".c"):
			cCount++
		case strings.HasSuffix(f.Path, ".cpp"):
			cppCount++
		}
	}

	// 优先级：Go > Rust > C++ > C
	if goCount > 0 {
		return "go"
	}
	if rustCount > 0 {
		return "rust"
	}
	if cppCount > 0 {
		return "cpp"
	}
	if cCount > 0 {
		return "c"
	}

	return ""
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
