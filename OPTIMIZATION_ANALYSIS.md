# ModuForge Agent 代码精简优化分析

## 🔴 严重重复问题

### 1. `resolvePath` 方法重复（9处！）
以下文件都有完全相同的 `resolvePath` 方法：
- `bash.go`
- `build_module.go`
- `delete_dir.go`
- `delete_file.go`
- `edit_file.go`
- `glob_search.go`
- `grep_search.go`
- `move_file.go`
- `write_file.go`

**解决方案：** 删除所有重复的 `resolvePath`，统一使用 `pathutil.go` 中的 `ResolveProjectPath`

---

## 🟡 冗余技能（可删除）

### 1. `create_dir` (1476 bytes)
- **原因：** `write_file` 已自动创建父目录
- **证据：** detect.go 中明确警告 "create_dir is NOT working"
- **状态：** 系统提示词已告诉 Agent 不要使用
- **建议：** 删除

### 2. `memory_manager` (3879 bytes)
- **原因：** `memory_v2` 是其增强版本（语义搜索、分层存储）
- **状态：** 两套记忆系统并存
- **建议：** 删除 `memory_manager`，保留 `memory_v2`

### 3. `skill_registry.go` vs `skill_manager.go`
- **原因：** 两个技能管理系统
- **建议：** 合并为一个

---

## 🟡 可合并的小文件

### 1. `meta.go` (226 bytes)
- **内容：** 仅类型重导出
- **建议：** 删除，更新导入

### 2. `pathutil.go` (603 bytes)
- **内容：** 单一工具函数
- **建议：** 保持，但移到 `internal/agent/` 公共包

---

## 🟢 过度工程化模块

### 1. `agent_preset.go` (19KB)
- **问题：** 19KB 的预设系统过于复杂
- **建议：** 简化为配置文件

### 2. `code_pipeline.go` (26KB)
- **问题：** 26KB 的代码管道
- **建议：** 拆分为更小的模块或合并到 `generate_code`

### 3. `build_module.go` (18KB)
- **问题：** 18KB 的构建模块
- **建议：** 简化核心逻辑

### 4. `lint_code.go` (17KB)
- **问题：** 17KB 的代码检查
- **建议：** 使用外部工具（如 golangci-lint）

---

## 📊 优化优先级

| 优先级 | 问题 | 影响 | 工作量 |
|--------|------|------|--------|
| P0 | 删除重复的 `resolvePath` | 减少 9 处重复代码 | 中 |
| P0 | 删除 `create_dir` | 清理冗余技能 | 小 |
| P1 | 删除 `memory_manager` | 统一记忆系统 | 小 |
| P1 | 删除 `meta.go` | 清理类型导出 | 小 |
| P2 | 简化 `agent_preset.go` | 减少复杂度 | 大 |
| P2 | 简化 `code_pipeline.go` | 减少复杂度 | 大 |
| P2 | 简化 `build_module.go` | 减少复杂度 | 中 |
| P3 | 合并 `skill_registry` 和 `skill_manager` | 统一管理 | 中 |

---

## 🎯 预期收益

1. **代码量减少：** ~15KB（删除重复代码和冗余技能）
2. **维护成本降低：** 统一的路径解析逻辑
3. **Agent 决策更清晰：** 减少冗余选项
4. **启动速度提升：** 更少的技能注册

---

## ⚠️ 注意事项

1. 删除技能前需检查是否有外部依赖
2. `resolvePath` 重构需确保所有调用方都使用新函数
3. 删除 `memory_manager` 需确保 `memory_v2` 功能完整
