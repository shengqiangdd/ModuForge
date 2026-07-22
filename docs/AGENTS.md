# AGENTS.md — ModuForge Lite AI Agent 开发指南

> 面向参与编码的 AI Agent（含 mimocode）。描述代码库组织、编码规范、ADR、任务流程。

---

## 1. 项目速览

```
项目:     ModuForge Lite
描述:     单二进制 Android 模块可视化 AI 开发平台
技术栈:   Go (Fiber v3) + Svelte 5 + SQLite + Docker SDK (可选)
内存:     1GB 可运行，空闲基线 < 60MB
入口:     backend/cmd/moduforge/main.go
```

### 1.1 核心概念

| 概念 | 说明 |
|------|------|
| Module | Android 刷机模块（脚本 + 配置 + WebUI） |
| Project | 用户在平台的模块开发项目 |
| Build | 打包成可刷入的 zip 包 |
| Template | 预置模块骨架 |
| Repo | GitHub/Gitee 开源模块追踪 |

### 1.2 模块类型

```go
const (
    ModuleMagisk ModuleType = "magisk"   // module.prop + system/ + META-INF
    ModuleKSU    ModuleType = "ksu"      // module.prop + webroot/ + ksu.sh
    ModuleAPatch ModuleType = "apatch"   // module.prop + action.sh + webui/
    ModuleHybrid ModuleType = "hybrid"   // Magisk + KSU 兼容
)
```

---

## 2. 代码库地图

```
backend/cmd/moduforge/main.go         -- 入口：Server 初始化 + 启动
backend/internal/
├── config/config.go                   -- 环境变量配置
├── database/
│   ├── database.go                    -- SQLite 初始化 + 迁移 + 接口
│   └── migrations.go                  -- 数据库迁移 SQL
├── domain/models.go                   -- 领域模型
├── middleware/middleware.go           -- JWT 认证 + Logger + CORS
├── handler/
│   ├── routes.go                      -- 路由注册
│   ├── auth.go                        -- 认证 API
│   ├── project.go                     -- 项目 CRUD API
│   ├── build.go                       -- 构建 API
│   └── ai.go                          -- AI API (SSE)
├── service/
│   ├── project.go                     -- 项目业务逻辑
│   ├── build.go                       -- 构建调度
│   └── ai.go                          -- AI 服务 (LLM)
├── builder/
│   └── builder.go                     -- 构建引擎 (Docker/copy)
└── llm/
    └── gateway.go                     -- LLM 网关
```

---

## 3. 编码规范

### 3.1 Go

```go
// 分层依赖: handler → service → (database | llm | builder)
// handler: 只做参数解析和错误响应
// service: 核心业务逻辑
// database/builder/llm: 基础设施

// 错误处理: errors.New 常量
var ErrNotFound = errors.New("not found")

// 依赖注入: 构造函数 + 接口
type ProjectService struct {
    db *sql.DB
}
func NewProjectService(db *sql.DB) *ProjectService { ... }
```

### 3.2 配置

```go
// config.go — 环境变量读取
type Config struct {
    Port           string  // :8080
    JWTSecret      string
    DatabasePath   string  // data/moduforge.db
    StoragePath    string  // data/storage
    LLMApiKey      string
    LLMEndpoint    string
    LLMModel       string
    DockerEndpoint string  // 空 = 不用 Docker
}
```

---

## 4. BuildTask 状态机

```
pending → running → success
              ↓
           failed / cancelled
```

---

## 5. 关键 ADR

### ADR-001: 单二进制部署

为什么不是微服务？—— 本项目 1GB 内存起步，微服务光进程开销就吃掉 50%。单二进制部署，后续用反向代理升迁。

### ADR-002: SQLite 起步

为什么不直接 PostgreSQL？—— 1GB 机器 PostgreSQL 内存基线 ~200MB。SQLite 零额外进程，100 并发内完全够用。到 1000 并发再切 PostgreSQL。

### ADR-003: 进程内队列

为什么不 Redis？—— 对于个人/小团队场景，一个 Go channel + 后台 goroutine 就够。到需要持久化任务队列时再引入 Redis。

### ADR-004: 前端 Embed

前端编译后 `embed` 进 Go 二进制，部署时无前端资源依赖。

---

## 6. 常见任务

### 6.1 添加新 API

1. `domain/models.go` — 定义数据模型（如有新实体）
2. `database/migrations.go` — 添加建表 SQL
3. `handler/routes.go` — 注册路由
4. `handler/xxx.go` — 实现 Handler
5. `service/xxx.go` — 实现业务逻辑

### 6.2 切换数据库到 PostgreSQL

- 在 `database/database.go` 中将 `*sql.DB` 换为 `*pgxpool.Pool`
- SQL 语法兼容：去掉 sqlite 专属语法（`datetime('now')` → `NOW()`）
- 修改 `config.go` 中的 `DATABASE_URL`

### 6.3 添加 LLM Provider

- 在 `llm/gateway.go` 的 `Chat()` 方法中加新 case
- 或者实现 `Provider` 接口

---

## 7. 安全红线

1. JWT Secret 必须通过环境变量注入
2. 构建脚本执行前必须扫描危险命令
3. WebUI 预览必须 iframe sandbox + CSP
4. 密码必须 bcrypt
5. API 速率限制不能绕过
