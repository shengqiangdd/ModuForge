# ModuForge 提示词系统

## 概述

ModuForge 使用 MD 文件来管理系统提示词，支持热加载和模块化维护。

## 文件结构

```
backend/internal/agent/prompts/
├── base.md          # 基础角色定义（所有模式共享）
├── act.md           # Act 模式提示词（代码修改模式）
├── plan.md          # Plan 模式提示词（只读分析模式）
├── free.md          # 免费模型提示词（精简版）
├── tools.md         # 工具参考文档
├── errors.md        # 错误处理参考
└── prompts.go       # Go 嵌入包（自动加载 MD 文件）
```

## 工作原理

1. **嵌入式加载**: Go 使用 `embed` 包将 MD 文件编译到二进制中
2. **缓存机制**: 首次加载后缓存，避免重复读取
3. **优雅降级**: MD 加载失败时自动回退到内置硬编码提示词
4. **热重载**: 调用 `prompts.Reload()` 可清除缓存重新加载

## 修改提示词

### 方法 1: 修改 MD 文件（推荐）

1. 编辑对应的 MD 文件（如 `act.md`）
2. 重新构建并部署：
   ```bash
   python deploy_prompts.py
   ```

### 方法 2: 运行时重载

如果不想重启服务，可以通过 API 触发重载（需要实现）：

```go
// 在代码中调用
prompts.Reload()
```

## 模式说明

### Act 模式（默认）
- 完整的代码修改权限
- 强制要求使用工具
- 详细的错误处理指南

### Plan 模式
- 只读分析
- 生成实施计划
- 不允许文件修改

### Free 模式
- 精简版提示词
- 适用于上下文窗口受限的模型
- 核心工作流保留

## 添加新模式

1. 在 `prompts/` 目录创建新的 MD 文件（如 `debug.md`）
2. 在 `prompts.go` 的 `Load()` 函数中添加模式映射：
   ```go
   case "debug":
       modeFile = "debug.md"
   ```
3. 重新构建部署

## 最佳实践

1. **保持简洁**: 每个 MD 文件专注于一个方面
2. **使用 Markdown**: 利用标题、列表、代码块提高可读性
3. **版本控制**: MD 文件变更应纳入 Git 管理
4. **测试验证**: 修改后运行 `go run test_prompts.go` 验证

## 示例：修改 Act 模式提示词

```bash
# 编辑 act.md
vim backend/internal/agent/prompts/act.md

# 重新部署
python deploy_prompts.py
```

## 故障排查

### 提示词未生效

1. 检查 MD 文件是否正确嵌入：
   ```bash
   go run test_prompts.go
   ```

2. 查看服务日志是否有错误：
   ```bash
   docker logs moduforge-app-1 | grep Prompts
   ```

3. 确认部署成功：
   ```bash
   curl http://localhost:8086/health
   ```

### 回退到硬编码提示词

如果 MD 加载失败，系统会自动使用内置提示词。查看日志：
```
[Prompts] Failed to load MD prompts, using fallback: ...
```

## 性能影响

- **启动时间**: 增加约 10ms（嵌入文件加载）
- **内存占用**: 增加约 20KB（6 个 MD 文件）
- **运行时**: 无影响（缓存后直接读取）

## 未来改进

1. **动态加载**: 支持从文件系统读取 MD 文件（无需重新部署）
2. **版本管理**: 提示词版本控制和回滚
3. **A/B 测试**: 支持多版本提示词对比
4. **用户自定义**: 允许用户自定义提示词模板
