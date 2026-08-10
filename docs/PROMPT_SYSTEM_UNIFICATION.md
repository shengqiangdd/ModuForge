# ModuForge 提示词系统统一完成

## 概述

成功将 ModuForge 的两套提示词系统（数据库 + MD文件）统一到 MD 文件系统，数据库作为用户覆盖层。

## 变更内容

### 1. 新增 MD 提示词文件

| 文件 | 模式 | 用途 |
|------|------|------|
| `base.md` | - | 核心身份 + 通用规则（所有模式共享） |
| `tools.md` | agent | 工具使用指南 |
| `errors.md` | - | 错误参考 |
| `generate.md` | generate | 模块生成模式 |
| `chat.md` | chat | 对话模式 |
| `repair.md` | repair | 修复模式 |
| `gather.md` | gather | 需求收集模式 |
| `agent.md` | agent | Agent自主模式 |
| `plan.md` | plan | 规划模式 |
| `act.md` | act | 执行模式 |
| `free.md` | free | 自由模式 |

### 2. 后端代码修改

**`backend/internal/agent/prompts/prompts.go`**:
- 更新 `Load()` 函数支持所有 8 种模式（5个数据库模式 + 3个Agent模式）
- 保持向后兼容，支持内存覆盖

**`backend/internal/service/ai.go`**:
- 添加 `prompts` 包导入
- 重写 `defaultSystemPrompt()` 函数，使用 MD 系统
- 重写 `loadPrompt()` 函数：
  - 优先级：用户数据库覆盖 → MD文件默认
  - 删除硬编码的提示词（~800行）
- 保持 `defaultPrompts()` 函数兼容

### 3. 前端组件（已存在）

**`frontend/src/routes/ai/components/modals/MDPromptsModal.svelte`**:
- MD 编辑器组件
- 侧边栏文件列表
- 保存/重置/重新加载功能

**后端 API**:
- `GET /api/v1/md-prompts` - 列出所有 MD 提示词文件
- `GET /api/v1/md-prompts/:name` - 获取特定提示词
- `PUT /api/v1/md-prompts/:name` - 更新提示词（内存覆盖）
- `DELETE /api/v1/md-prompts/:name` - 重置提示词

## 架构设计

```
用户请求
    ↓
loadPrompt(mode, userID)
    ↓
┌─────────────────────────────────────┐
│ 1. 检查用户数据库覆盖（ai_prompts表）   │
│    → 有则返回用户自定义内容           │
└─────────────────────────────────────┘
    ↓ 无
┌─────────────────────────────────────┐
│ 2. 加载 MD 文件系统（prompts.Load）   │
│    → base.md + mode.md + tools.md   │
└─────────────────────────────────────┘
    ↓ 成功
┌─────────────────────────────────────┐
│ 3. 返回组装后的完整提示词              │
└─────────────────────────────────────┘
```

## 优势

1. **维护性提升**：编辑 MD 文件无需重新编译二进制
2. **版本控制友好**：Git 追踪提示词变更
3. **模块化设计**：base + mode + tools + errors 分层
4. **用户覆盖**：数据库保留用户自定义能力
5. **热更新**：通过 API 更新内存覆盖，无需重启

## 编译状态

✅ `go build ./...` 编译通过
✅ 所有 MD 文件就位（11个文件，12.7KB）
✅ 后端 API 正常工作

## 下一步

1. 部署到服务器验证
2. 测试前端 MD 编辑器功能
3. 监控提示词加载日志
4. 考虑添加持久化存储选项（可选）
