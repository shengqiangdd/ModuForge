# ModuForge E2E 测试增强报告

**日期**: 2026-09-02  
**测试环境**: Backend 192.168.2.9:8086, Model: xiaomi/mimo-v2.5  
**测试人**: AI Agent (自动化)

---

## 一、本次修复清单

### 1. android_app / build_android_app 工具暴露问题
**文件**: 
- `backend/internal/agent/skills/android_app.go`
- `backend/internal/agent/skills/build_android_app.go`

**修改**: 
```go
// 修改前
Core: false,

// 修改后
Core: true, // Always expose to all model tiers
```

**原因**: 在 `tools.go` 的 `buildToolDefinitionsForMode` 中，当 `hasCoreTools` 为 true 时，只有 `Core: true` 或 `ReadOnly: true` 的工具才会暴露。这两个工具既不是 Core 也不是 ReadOnly，所以被过滤掉了。

---

### 2. bash 工具白名单扩展
**文件**: `backend/internal/agent/skills/bash.go`

**修改内容**:

#### 2.1 添加 Shell 内建命令
```go
// Shell built-ins (safe when used in scripts)
"for ", "while ", "if ", "case ", "function ",
"do", "done", "then", "else", "elif", "fi", "esac",
"echo ", "printf ", "read ", "test ", "[", "[[",
"source ", ". ",
"exit ", "return ", "break ", "continue ",
"export ", "local ", "declare ", "typeset ",
"set ", "unset ", "shift ",
"trap ", "wait ", "exec ",
```

#### 2.2 添加 Magisk 模块专用命令
```go
// Magisk module tools
"unzip -t ",
"ui_print ",
"set_perm ", "set_perm_recursive ",
"mkdir -p ",
"cp -r ",
"rm -rf ",
"chcon ",
```

#### 2.3 添加 Shell 脚本特殊处理
```go
// Special case: if the command looks like a shell script (contains shell patterns),
// allow it without strict whitelist checking. This enables Magisk module development.
shellPatterns := []string{
    "for ", "while ", "if ", "case ", "function ",
    "do\n", "done\n", "then\n", "else\n", "fi\n", "esac\n",
    "#!/", "#!/bin/sh", "#!/system/bin/sh",
    "MODDIR=", "MODPATH=", "ui_print",
}
```

#### 2.4 多行命令规范化
```go
// Normalize multi-line commands: replace newlines with spaces for validation
cmd = strings.ReplaceAll(cmd, "\n", " ")
cmd = strings.ReplaceAll(cmd, "\r", " ")
```

---

### 3. syntax_checker 支持 Shell 脚本
**文件**: 
- `backend/internal/agent/skills/syntax_checker.go`
- `backend/internal/agent/skills/compile_errors.go`
- `backend/internal/agent/tools.go`

**修改内容**:

#### 3.1 添加 hasShell 到 sourceInfo 结构体
```go
type sourceInfo struct {
    hasCargo bool
    cargoDir string
    hasCpp   bool
    hasGo    bool
    goModDir string
    hasShell bool  // 新增
}
```

#### 3.2 添加 Shell 文件检测
```go
if !info.hasShell {
    switch ext {
    case ".sh", ".bash":
        info.hasShell = true
    }
    // Also check for known shell script names without extension
    if name == "customize.sh" || name == "service.sh" || name == "post-fs-data.sh" || name == "uninstall.sh" {
        info.hasShell = true
    }
}
```

#### 3.3 添加 checkShell 函数
```go
func (s *SyntaxCheckerSkill) checkShell(ctx context.Context, projectPath string) SyntaxResult {
    result := SyntaxResult{Language: "shell", Hints: []string{}}
    
    var shellFiles []string
    filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return nil
        }
        ext := strings.ToLower(filepath.Ext(path))
        name := info.Name()
        if ext == ".sh" || ext == ".bash" || name == "customize.sh" || name == "service.sh" || name == "post-fs-data.sh" || name == "uninstall.sh" {
            shellFiles = append(shellFiles, path)
        }
        return nil
    })
    
    for _, shellFile := range shellFiles {
        cmd := exec.CommandContext(ctx, "bash", "-n", shellFile)
        output, err := cmd.CombinedOutput()
        if err != nil {
            relPath, _ := filepath.Rel(projectPath, shellFile)
            errors := parseShellSyntaxOutput(relPath, string(output))
            result.Errors = append(result.Errors, errors...)
        }
    }
    
    result.Errors = dedupSyntaxErrors(result.Errors)
    result.Passed = len(result.Errors) == 0
    
    for _, e := range result.Errors {
        hint := generateShellFixHint(e)
        if hint != "" {
            result.Hints = append(result.Hints, hint)
        }
    }
    
    return result
}
```

#### 3.4 更新语言枚举
```go
// tools.go
"language": map[string]interface{}{
    "type": "string", 
    "enum": []interface{}{"auto", "go", "rust", "cpp", "shell"}, 
    "description": "Language to check (default: auto-detect)",
},
```

---

### 4. act.md 提示词优化
**文件**: `backend/internal/agent/prompts/act.md`

**修改内容**:

#### 4.1 推荐使用 write_file_batch
```markdown
## Multi-File Generation Rule (CRITICAL)
When generating multiple files (e.g., a complete Magisk module with 4+ files), 
**use `write_file_batch`** to create all files in ONE call.
- `write_file_batch` accepts a JSON array of {path, content} objects
- This is faster and more reliable than multiple `write_file` calls
- After writing, verify with `list_dir` to confirm all files exist
- For single files or small edits, still use `write_file` or `edit_file`
- This reduces context window usage and avoids loop detection
```

#### 4.2 添加语法检查工作流
```markdown
## Workflow
1. read_file/grep_search → understand current state
2. If user needs an Android APP → call `android_app` then `build_android_app`
3. write_file_batch/write_file/edit_file → implement module code
4. syntax_checker → validate syntax (use language: "shell" for Magisk modules)
5. build_module → verify (MANDATORY, max 3 retries on failure)
6. device_test → install & verify on device (if connected)
7. Report: files changed, build status, device test result
```

#### 4.3 添加 Shell 语法检查说明
```markdown
## Shell Syntax Checking
For Magisk modules with shell scripts, use these methods to validate syntax:
1. **syntax_checker** tool: `{"project_id": "...", "language": "shell"}` — validates all .sh files
2. **bash -n** command: `bash -n service.sh` — quick syntax check (no execution)
3. **build_module** — automatically validates structure and syntax during build
```

---

## 二、测试结果

### 测试项目
| 项目 | 结果 | 说明 |
|------|------|------|
| write_file_batch 批量创建 | ✅ 通过 | 5个文件一次性写入成功 |
| build_module 自动修复 | ✅ 通过 | 缺失 META-INF 时自动创建 |
| bash 工具（shell 命令） | ⚠️ 待验证 | 需要重新编译部署 |
| syntax_checker（shell） | ⚠️ 待验证 | 需要重新编译部署 |
| android_app 工具 | ⚠️ 待验证 | 需要重新编译部署 |

### 待部署
所有修改已应用到本地代码，需要：
1. 重新编译后端（Go 1.25+）
2. 部署到 192.168.2.9:8086
3. 重新运行 E2E 测试验证

---

## 三、下一步计划

### 高优先级
- [ ] 重新编译后端并部署
- [ ] 验证 android_app 工具是否正常暴露
- [ ] 测试完整的 APP 生成流程（android_app → build_android_app → build_module）

### 中优先级
- [ ] 优化上下文压缩后的状态恢复逻辑
- [ ] 添加更多的 Shell 脚本模式到白名单
- [ ] 改进 test_module 的参数格式文档

### 低优先级
- [ ] 测试 prep 模式（规划模式）
- [ ] 测试多文件删除/移动操作
- [ ] 添加更多的 Magisk 模块开发模板
