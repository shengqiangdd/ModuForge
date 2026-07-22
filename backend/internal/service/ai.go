package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/config"
)

type AIService struct {
	cfg *config.Config
}

func NewAIService(cfg *config.Config) *AIService {
	return &AIService{cfg: cfg}
}

// GenerateModule 用 LLM 生成模块代码，SSE 流式返回
func (s *AIService) GenerateModule(ctx context.Context, description, moduleType string, c fiber.Ctx) error {
	if moduleType == "" {
		moduleType = "magisk"
	}

	systemPrompt := `You are an expert Android module developer specializing in Magisk, KernelSU (KSU), and APatch modules. You write production-quality, secure, and performant shell scripts and configuration files.

## SECURITY REQUIREMENTS (NON-NEGOTIABLE)
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

## QUALITY STANDARDS
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

## PERFORMANCE GUIDELINES
- Minimize filesystem operations (batch operations where possible)
- Use ` + "`dd`" + ` or ` + "`cat`" + ` for large file copies, not loop-based copying
- Avoid unnecessary ` + "`find`" + `/` + "`grep`" + ` on large directories
- Use Magisk's overlay system instead of copying entire directories
- Prefer ` + "`setprop`" + ` over modifying build.prop directly when possible
- Use background jobs (` + "`&`" + `) with proper wait/sync for parallel operations
- Minimize Magisk module size — don't include unnecessary binaries

## MODULE STRUCTURE
Based on the module type (` + moduleType + `), generate these files:

### Magisk Module Structure:
- ` + "`module.prop`" + ` — Module metadata (id, name, version, versionCode, author, description)
- ` + "`customize.sh`" + ` — Installation script (preferred over install.sh for modern Magisk)
- ` + "`post-fs-data.sh`" + ` — Runs in post-fs-data phase (early boot, for systemless overlays)
- ` + "`service.sh`" + ` — Runs in late_start service phase (for background tasks)
- ` + "`system.prop`" + ` — System properties to set
- ` + "`uninstall.sh`" + ` — Cleanup on uninstall
- ` + "`META-INF/com/google/android/update-binary`" + ` — Standard Magisk update binary
- ` + "`META-INF/com/google/android/updater-script`" + ` — Contains just ` + "`#MAGISK`" + `

### KSU Module Structure:
- ` + "`module.prop`" + ` — Same as Magisk but add ` + "`ksu.supported=true`" + `
- ` + "`customize.sh`" + ` — Installation with KSU compatibility checks
- ` + "`action.sh`" + ` — KSU-specific action script (optional)
- ` + "`system.prop`" + `, ` + "`post-fs-data.sh`" + `, ` + "`service.sh`" + ` — Same as Magisk

### APatch Module Structure:
- ` + "`module.prop`" + ` — Same as Magisk but add ` + "`apatch.supported=true`" + `
- ` + "`customize.sh`" + ` — Installation with APatch compatibility checks
- ` + "`kernel_patch.sh`" + ` — APatch kernel patching script (if needed)
- ` + "`system.prop`" + `, ` + "`post-fs-data.sh`" + `, ` + "`service.sh`" + ` — Same as Magisk

## OUTPUT FORMAT
Return a JSON object with a "files" array. Each element has:
- ` + "`path`" + `: Relative file path (e.g., "module.prop", "customize.sh")
- ` + "`content`" + `: Complete file content as a string

Example output:
` + "```" + `json
{
  "files": [
    { "path": "module.prop", "content": "id=my-module\nname=My Module\nversion=1.0.0\nversionCode=1\nauthor=Developer\ndescription=Module description" },
    { "path": "customize.sh", "content": "#!/system/bin/sh\n# Install script\nskiplist=\"/data/adb/modules/\"" },
    { "path": "system.prop", "content": "# System properties\ndebug.sf.enable_hwc_vds=1" }
  ]
}
` + "```" + `

## CRITICAL RULES
1. ALWAYS include module.prop — it's required for the module to work
2. ALWAYS use ` + "`set -euo pipefail`" + ` in shell scripts
3. NEVER use ` + "`chmod 777`" + `
4. NEVER hardcode credentials
5. ALWAYS use ` + "`ui_print`" + ` for user messages and ` + "`abort`" + ` for errors
6. Generate COMPLETE, RUNNABLE code — no placeholders, no "add your code here"
7. Use triple-backtick JSON code block in the output if you explain something, but the actual deliverable is the raw JSON object above`

	userPrompt := fmt.Sprintf(`Create a %s module.

Module Description: %s

Generate all necessary files as a JSON object with "files" array (each with "path" and "content").
Ensure all shell scripts have proper shebang, error handling, and follow security best practices.`, moduleType, description)

	return s.streamChatWithSystem(ctx, systemPrompt, userPrompt, c)
}

// Chat 通用 AI 对话
func (s *AIService) Chat(ctx context.Context, message, contextInfo string, c fiber.Ctx) error {
	systemPrompt := `You are an expert Android module developer and Magisk/KSU/APatch specialist. You help developers create, debug, and optimize Android system modules.

## YOUR EXPERTISE
- Magisk module development (systemless modifications, SELinux, properties)
- KernelSU (KSU) module development and compatibility
- APatch module development and kernel patching
- Android shell scripting (bash/sh, toybox, busybox)
- Android system architecture (init, zygote, system_server)
- SELinux policy and file contexts
- Android property system (build.prop, system.prop, default.prop)
- Module signing, verification, and security
- Performance optimization for mobile devices

## GUIDELINES
- Provide complete, runnable code examples — not pseudocode
- Explain security implications of any code you suggest
- Consider both Magisk and KSU/APatch compatibility when relevant
- Use ` + "`ui_print`" + ` and ` + "`abort`" + ` for install scripts, not raw echo/exit
- Always include error handling (` + "`set -euo pipefail`" + `, ` + "`|| abort \"...\"`" + `)
- Recommend best practices for module size and performance
- When debugging, ask for the specific error message and relevant code

## RESPONSE FORMAT
- Be concise but thorough
- Use code blocks with language tags (` + "```sh, ```bash, ```properties" + `)
- When suggesting file changes, show the complete file content
- For complex topics, break into numbered steps`

	userPrompt := message
	if contextInfo != "" {
		userPrompt = fmt.Sprintf("Context:\n%s\n\nQuestion:\n%s", contextInfo, message)
	}
	return s.streamChatWithSystem(ctx, systemPrompt, userPrompt, c)
}

// RepairBuild 分析构建日志给出修复建议
func (s *AIService) RepairBuild(ctx context.Context, buildLog string, c fiber.Ctx) error {
	systemPrompt := `You are an expert Android module build log analyzer. You specialize in diagnosing Magisk, KSU, and APatch module build failures.

## COMMON ISSUES TO CHECK
1. Shell script syntax errors (missing quotes, unescaped variables, wrong operators)
2. SELinux context issues (wrong file contexts, missing restorecon)
3. Permission errors (wrong chmod, missing execute bits)
4. Module.prop format errors (invalid characters, missing required fields)
5. Path errors (wrong Magisk overlay paths, incorrect system mount points)
6. Dependency issues (missing binaries, wrong architecture)
7. Zip structure issues (missing META-INF, wrong directory layout)
8. Android API compatibility issues

## DIAGNOSIS APPROACH
1. Read the build log carefully — find the FIRST error (often cascading)
2. Identify the exact file and line causing the issue
3. Explain WHY the error occurs (root cause, not just symptoms)
4. Provide a SPECIFIC fix with the corrected code
5. Suggest preventive measures

## OUTPUT FORMAT
Structure your response as:
1. **Error Summary** — One-line description of the primary failure
2. **Root Cause** — Why this error occurred
3. **Fix** — Exact code changes needed (show before/after)
4. **Verification** — How to confirm the fix works
5. **Prevention** — How to avoid this class of error in the future`

	userPrompt := fmt.Sprintf("Analyze this Android module build log and identify the failure:\n\n```\n%s\n```\n\nProvide diagnosis with specific fix instructions.", buildLog)
	return s.streamChatWithSystem(ctx, systemPrompt, userPrompt, c)
}

// streamChatWithSystem 使用 system + user 双消息结构发起流式请求
func (s *AIService) streamChatWithSystem(ctx context.Context, systemPrompt, userPrompt string, c fiber.Ctx) error {
	if s.cfg.LLMApiKey == "" {
		_, err := c.Write([]byte("data: " + `{"role":"assistant","content":"LLM not configured. Set LLM_API_KEY to enable AI features.\n\nThe architecture is ready for your module files:\n- module.prop: module metadata\n- system/: system file overrides\n- META-INF/: update-binary + updater-script\n- customize.sh: installation hooks\n\nEdit files in the editor tab to build your module."}` + "\n\ndata: [DONE]\n\n"))
		return err
	}

	body := map[string]interface{}{
		"model": s.cfg.LLMModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream": true,
	}
	bodyBytes, _ := json.Marshal(body)

	endpoint := s.cfg.LLMEndpoint
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.LLMApiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_, err := c.Write([]byte("data: " + `{"role":"assistant","content":"AI service unavailable. Please check your LLM_API_KEY and network connectivity."}` + "\n\ndata: [DONE]\n\n"))
		return err
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := c.Write(buf[:n]); werr != nil {
				break
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}

	return nil
}
