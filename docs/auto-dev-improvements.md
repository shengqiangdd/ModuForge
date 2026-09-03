# ModuForge 中型模块自动开发能力 — 改进报告

## 改进日期
2026-08-29

## 改进摘要

对 ModuForge 的自动开发中型模块能力进行了全面测试和优化，共发现并修复 **7 个关键问题**，新增 **3 个核心功能模块**。

---

## 1. 发现的问题

### 问题 1：Intent Compiler V1 生成的是死代码（严重）
**现象**：`checkOnce()` 函数体只包含注释，不包含任何可执行逻辑
**影响**：生成的守护进程每 30 秒执行一次，但什么都不做
**根因**：V1 的 `synthesizeGo` 仅将触发条件输出为注释，而非生成真实的 `if` 条件判断

### 问题 2：无逐文件语法验证
**现象**：错误仅在最终构建阶段（Stage 4）被捕获
**影响**：多次迭代浪费 LLM token，无法在早期发现问题

### 问题 3：Linter 规则不足
**现象**：仅 6 条规则，缺少 Go 语法检查
**影响**：明显的 Go 语法错误（空赋值、未闭合字符串）未被检测

### 问题 4：Auto-fix 功能有限
**现象**：仅处理 postprocess 正则修复
**影响**：无法修复 go.mod 依赖、缺失导入等常见问题

### 问题 5：RAG 上下文注入不足
**现象**：每个提示注入仅 2 个示例
**影响**：LLM 缺乏足够的模式参考

### 问题 6：Config 值未在代码合成中使用
**现象**：配置的 `check_interval`、`threshold` 等值未被引用
**影响**：生成的守护进程使用硬编码值

### 问题 7：原始 intent_compiler.go 存在字符串转义错误
**现象**：多处 `fmt.Sprintf` 中的引号未转义
**影响**：整个 builder 包无法编译

---

## 2. 改进措施

### 改进 1：Intent Compiler V2 — 生成可执行逻辑
**文件**：`internal/builder/intent_compiler_v2.go`（新增 700+ 行）

**核心改进**：
- `synthesizeGoV2()` 替代 `synthesizeGo()`
- `generateCheckOnceV2()` 生成真实的 `if` 条件判断
- `generateConditionCode()` 将自然语言条件转换为 Go 代码
- `generateActionCodeV2()` 将动作描述转换为 `log.Printf` 等调用
- `generateMainV2()` 生成配置感知的守护进程主函数
- `generateConfigConstants()` 从配置提取常量

**示例输出**：
```go
// Before (V1):
func checkOnce() {
    // Check: temperature > threshold
    // Action: log warning
}

// After (V2):
func checkOnce(cfg interface{}) {
    c, ok := cfg.(Config)
    if !ok { log.Printf("Warning: invalid config type\n"); return }
    _ = c
    
    // Trigger 1: temperature > threshold
    if readSysfsInt("/sys/class/thermal/thermal_zone0/temp") / 1000 > c.Threshold {
        logActivity("Trigger 1: log warning about overheating")
        log.Printf("⚠️  WARNING: log warning about overheating")
    }
}
```

### 改进 2：逐文件语法验证
**文件**：`internal/builder/stage_validate.go`（新增 170+ 行）

**功能**：
- `ValidateGoSyntax()` — 快速语法检查（包声明、大括号平衡、字符串闭合）
- `ValidateGoModule()` — 使用 `go vet` 进行模块级验证
- `QuickValidateAllFiles()` — 批量快速验证

**集成**：在 Stage 2 每个文件合成后立即验证，失败时触发自动修复

### 改进 3：扩展 Linter 规则
**文件**：`internal/quality/linter.go`（新增 6 条规则）

**新增规则**：
| 规则 | 严重性 | 说明 |
|------|--------|------|
| `go-package-declaration` | Error | Go 文件必须有 package 声明 |
| `go-balanced-braces` | Error | 大括号必须匹配 |
| `go-no-empty-assignments` | Error | 禁止空赋值 (= ; ) |
| `go-no-unterminated-strings` | Error | 字符串必须闭合 |
| `module-prop-required` | Error | module.prop 必须包含 id/name/version/versionCode |
| `service-sh-root-check` | Warning | service.sh 应检查 root 权限 |

### 改进 4：单文件自动修复
**文件**：`internal/service/multi_stage.go`（新增 `autoRepairSingleFile`）

**功能**：
- 检测到语法错误时，发送单个文件到 LLM 进行快速修复
- 支持 JSON 响应和代码围栏提取
- 修复后自动更新数据库

### 改进 5：RAG 上下文增强
**修改**：将每个提示的 RAG 注入从 2 个示例增加到 4 个

### 改进 6：修复原始编译错误
**文件**：`internal/builder/intent_compiler.go`

**修复**：
- 修复 8 处 `fmt.Sprintf` 字符串转义错误
- 修复 `generateGoStruct` 中的反引号问题
- 修复 `generateInitCode`、`generateCleanupCode` 中的引号问题
- 修复 C 代码生成中的引号问题
- 添加缺失的辅助函数（`toSnakeCase`、`toPascalCase`、`fixIntentJSON`）

---

## 3. 测试验证

### 新增测试
**文件**：`internal/builder/intent_compiler_v2_test.go`（8 个测试用例）

| 测试 | 验证内容 |
|------|----------|
| `TestSynthesizeGoV2_BasicDaemon` | 基础守护进程生成 |
| `TestSynthesizeGoV2_WithTriggers` | 多触发器逻辑 |
| `TestGenerateConditionCode` | 条件到代码的映射 |
| `TestGenerateMainV2` | 主函数结构 |
| `TestGenerateCheckOnceV2_MultipleTriggers` | 多触发器 checkOnce |
| `TestGenerateConfigConstants` | 配置常量提取 |
| `TestSelectGoHelpersV2` | 辅助函数选择 |
| `TestIntentCompilerV2_FullPipeline` | 完整流水线 |

### 测试结果
```
ok   github.com/moduforge/backend/internal/builder   0.024s
ok   github.com/moduforge/backend/internal/quality    0.003s
```

---

## 4. 文件变更清单

| 文件 | 操作 | 行数 |
|------|------|------|
| `internal/builder/intent_compiler_v2.go` | 新增 | +700 |
| `internal/builder/intent_compiler_v2_test.go` | 新增 | +450 |
| `internal/builder/stage_validate.go` | 新增 | +170 |
| `internal/builder/intent_compiler.go` | 修改 | ~50 处修复 |
| `internal/builder/self_repair.go` | 修改 | 1 处 |
| `internal/quality/linter.go` | 修改 | +120 行 |
| `internal/quality/linter_test.go` | 修改 | 1 处 |
| `internal/service/multi_stage.go` | 修改 | +150 行 |
| `docs/auto-dev-improvements.md` | 新增 | 文档 |

---

## 5. 后续优化建议

### 优先级 P0（建议立即实施）
1. **go.mod 自动修复** — 自动清理 stdlib require、添加缺失依赖
2. **增量编译验证** — Stage 1.5 后增加 `go build -p 1` 验证

### 优先级 P1（短期）
3. **RAG 示例质量** — 基于模块类型动态选择最相关的示例
4. **触发条件语义分析** — 支持中文条件（如 "温度超过阈值"）
5. **多文件依赖分析** — 自动检测并生成缺失的辅助文件

### 优先级 P2（中期）
6. **模板版本管理** — 支持模板的 A/B 测试和自动选择
7. **构建缓存** — 缓存中间产物，避免重复合成
8. **质量评分** — 对生成的代码进行自动化质量评分

---

## 6. 架构改进前后对比

```
改进前：
  Stage 0 → Stage 1 → Stage 1.5 → Stage 2 (死代码) → Stage 4 (编译) → Auto-fix
  
改进后：
  Stage 0 → Stage 1 → Stage 1.5 → Stage 2 (可执行逻辑) → 逐文件验证 → Stage 4 → Auto-fix
                                    ↑                            ↓
                                    └──── 自动修复 ←────── 失败时 ─────┘
```
