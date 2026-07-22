# ModuForge — MiMoCode 编码委派工作流指南

> 本文档描述了如何将 ModuForge 的代码生成任务委派给 MiMoCode（mimo acp），
> 并确保其运行不影响工作区中的其他项目（Wrench、OpsPilot 等）。

---

## 1. 原理

MiMoCode ACP 模式 (`mimo acp`) 启动一个 agent server，通过 `delegate_external_agent` 工具调用。
关键约束：

- MiMoCode 的 **cwd 默认是 QwenPaw 工作目录**（`/app/working/workspaces/default`）
- 如果 MiMoCode 同时服务多个项目，它们的文件变更会互相干扰
- 通过 `delegate_external_agent` 的 `cwd` 参数或 `--cwd` 启动参数隔离

---

## 2. 隔离方案

### 方案 A：使用 `delegate_external_agent` 的 cwd 参数（推荐）

```json
// agent.json 中的 mimocode ACP 配置（已存在）
"mimocode": {
  "enabled": true,
  "command": "mimo",
  "args": ["acp"],
  "trusted": true,
  ...
}
```

调用时指定 cwd：

```bash
# 通过 delegate_external_agent 工具
delegate_external_agent(
  action="start",
  runner="mimocode",
  message="为 ModuForge 项目实现用户认证模块，项目在 /app/working/workspaces/default/ModuForge",
  cwd="/app/working/workspaces/default/ModuForge"
)
```

这告诉 MiMoCode 的 ACP server 将工作目录隔离到 ModuForge 子目录，
MiMoCode 的文件读写、搜索、git 操作等都不会影响上层工作区。

### 方案 B：为 ModuForge 启动独立的 mimo ACP server

```bash
# 在 ModuForge 目录内启动独占的 ACP server
cd /app/working/workspaces/default/ModuForge
mimo acp --port 0 --cwd /app/working/workspaces/default/ModuForge
```

然后在 agent.json 中注册为独立 runner：

```json
"moduforge-mimo": {
  "enabled": true,
  "command": "mimo",
  "args": ["acp", "--cwd", "/app/working/workspaces/default/ModuForge"],
  "trusted": true,
  ...
}
```

**缺点**：需要额外注册一个 runner。

### 推荐：方案 A

方案 A 用同一个 ACP server，通过 `cwd` 参数隔离，零配置变更，不影响其他项目。

---

## 3. 典型委派任务

### 3.1 生成 Go 后端代码

```
委派给 mimocode：
  项目: ModuForge
  目录: /app/working/workspaces/default/ModuForge
  任务: 实现 backend/internal/handler/project.go 中 List/Create/Get/Update/Delete 五个 handler
  约束:
    - 使用 Fiber v3
    - 错误处理统一返回 {error: string}
    - 所有方法接收 *fiber.Ctx 返回 error
    - 遵循 AGENTS.md 中的编码规范
```

### 3.2 生成 Svelte 5 前端页面

```
委派给 mimocode：
  项目: ModuForge
  目录: /app/working/workspaces/default/ModuForge
  任务: 实现前端项目列表页面 frontend/src/routes/projects/+page.svelte
  约束:
    - 使用 Svelte 5 runes 语法 ($state, $derived, $effect)
    - 使用 Material Web MD3 组件 (<md-button>, <md-list>)
    - 使用 UnoCSS 原子类
    - 调用 /api/v1/projects 获取数据
```

### 3.3 编写 SQL 查询

```
委派给 mimocode：
  项目: ModuForge
  目录: /app/working/workspaces/default/ModuForge
  任务: 为 project_files 表添加 batch write 查询
  约束:
    - SQLite 语法
    - 使用 INSERT OR REPLACE
```

---

## 4. 工作流

```
你（架构师）
├── 1. 分析需求，设计接口签名
├── 2. 将实现任务拆分为独立子任务
├── 3. 通过 delegate_external_agent 委派给 mimocode
│     └── cwd: /app/working/workspaces/default/ModuForge
├── 4. 审查 mimocode 生成的代码
├── 5. 运行测试/编译检查
└── 6. 合并到主分支
```

### 性能建议

- 每个子任务不超过 3-5 个文件，否则 MiMoCode 可能超时
- 复杂逻辑分批委派，每批聚焦一个模块
- 使用 `max_runtime: 600` 避免超时中断

---

## 5. 验证

委派后检查：

```bash
# 1. 确认文件在 ModuForge 目录内（未污染其他项目）
ls /app/working/workspaces/default/ModuForge/backend/internal/handler/

# 2. 编译检查
cd /app/working/workspaces/default/ModuForge/backend && go build ./...

# 3. 运行测试
cd /app/working/workspaces/default/ModuForge/backend && go test ./...
```
