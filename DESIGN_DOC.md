# ModuForge Lite — Android 模块可视化 AI 开发平台

> **设计文档 v2.0 | 2026-07-21**
> 核心理念：**1GB 可运行，单进程，零依赖，升迁无痛**

---

## 1. 设计哲学

### 1.1 轻量十六字

**单进程、零依赖、嵌入式、可升迁。**

每层组件都定义了接口，起步用最轻的实现，升级时换一个实现类即可。

### 1.2 1GB 内存预算表

| 占用项目 | 预估值 | 备注 |
|---------|--------|------|
| OS + 系统基础 | ~250MB | Alpine/Debian slim |
| Go 二进制 (含 embed 前端) | ~20MB | 编译优化后 |
| SQLite (含 FTS5) | ~30MB | WAL + cache |
| 构建沙箱进程 (按需) | ~200MB | 构建时临时启动 |
| **合计** | **~500MB** | |
| **余量** | **~500MB** | 可开 1-2 个额外构建 |

### 1.3 升迁路径

| 组件 | 轻量版 (Lite) | Pro 版 (加组件) |
|------|--------------|----------------|
| 数据库 | SQLite (内嵌) | → PostgreSQL 16 |
| 缓存/队列 | Go channel (进程内) | → Redis 7 + Asynq |
| 搜索 | SQLite FTS5 | → Meilisearch |
| 存储 | 本地磁盘 | → MinIO / S3 |
| 可观测 | 结构化日志 | → Prometheus + Grafana |
| 部署 | 单机/单进程 | → Docker Compose / K8s |

**切换接口全部预定义好**，改配置即可升迁，不改代码。

---

## 2. 架构全景

```
┌─────────────────────────────────────┐
│       单一 Go 二进制  (moduforge)     │
│  ┌───────────────────────────────┐  │
│  │  Go Fiber v3 HTTP Server       │  │
│  │  + embed 前端静态文件           │  │
│  ├──────┬──────┬──────┬──────────┤  │
│  │AI    │Project│Build │Repo     │  │
│  │Svc   │Svc    │Svc   │Svc      │  │
│  ├──────┴──────┴──────┴──────────┤  │
│  │         Store 层               │  │
│  │  ┌──────┐ ┌──────┐ ┌───────┐ │  │
│  │  │SQLite│ │本地  │ │ LLM   │ │  │
│  │  │+FTS5 │ │磁盘  │ │Gateway│ │  │
│  │  └──────┘ └──────┘ └───────┘ │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
         │       │
         ▼       ▼
    Docker沙箱    LLM API
   (按需启动)   (OpenAI/Claude等)
```

### 2.1 技术栈

| 层级 | 技术 | 版本 | 理由 |
|------|------|------|------|
| 前端框架 | **Svelte 5** | ^5.x | 编译型，~5KB gzip |
| UI | **Material Web** | ^2.x | Google 官方 MD3 组件 |
| CSS | **UnoCSS** | ^0.64 | 零运行时，按需生成 |
| 编辑器 | **CodeMirror 6** | ^6.x | 轻量、高可定制 |
| 后端 | **Go + Fiber v3** | ^3.x | 单二进制部署，极致性能 |
| 数据库 | **SQLite** (CGo-free) | ^3.x | 零额外进程 |
| 搜索 | **SQLite FTS5** | 内建 | 中文 tokenizer 支持 |
| 构建沙箱 | **Docker SDK** (可选) | latest | cgroups 隔离 |
| LLM | **自研 LLM Gateway** | - | 统一接口多 provider |

---

## 3. 目录结构

```
ModuForge/
├── backend/                    # Go 单二进制
│   ├── cmd/moduforge/main.go   # 入口（HTTP + job queue）
│   └── internal/
│       ├── domain/             # 领域模型
│       │   └── models.go
│       ├── database/           # 数据库
│       │   ├── database.go     # SQLite 初始化 + 迁移
│       │   └── migrations.go   # 迁移 SQL
│       ├── handler/            # HTTP 处理器
│       │   ├── routes.go       # 路由注册
│       │   ├── auth.go
│       │   ├── project.go
│       │   ├── build.go
│       │   └── ai.go
│       ├── middleware/         # 中间件
│       │   └── middleware.go
│       ├── service/            # 业务逻辑
│       │   ├── project.go
│       │   ├── build.go
│       │   └── ai.go
│       ├── builder/            # 构建引擎（Docker 可选）
│       │   ├── builder.go
│       │   └── templates/
│       ├── llm/                # LLM 网关
│       │   └── gateway.go
│       └── config/             # 配置
│           └── config.go
├── frontend/                   # Svelte 5 PWA
│   ├── src/
│   │   ├── routes/             # 页面路由
│   │   │   ├── +layout.svelte
│   │   │   ├── +page.svelte    # Dashboard
│   │   │   ├── projects/
│   │   │   │   ├── +page.svelte
│   │   │   │   └── [id]/
│   │   │   │       ├── +page.svelte     # 编辑器
│   │   │   │       ├── build/+page.svelte
│   │   │   │       └── preview/+page.svelte
│   │   │   ├── ai/+page.svelte
│   │   │   ├── repo/+page.svelte
│   │   │   ├── market/+page.svelte
│   │   │   └── settings/+page.svelte
│   │   └── lib/
│   │       ├── components/     # MD3 组件
│   │       ├── api/            # API 客户端
│   │       ├── stores/         # Runes 状态管理
│   │       └── md3/            # MD3 主题
│   ├── uno.config.ts
│   ├── vite.config.ts
│   └── package.json
├── containers/                 # 构建镜像
│   ├── magisk/
│   ├── ksu/
│   └── apatch/
├── data/                       # 运行时数据（本地存储）
│   ├── moduforge.db            # SQLite 数据库
│   └── storage/                # 文件存储
├── docs/
│   ├── AGENTS.md               # AI Agent 开发指南
│   └── SCALING.md              # 升迁指南
├── scripts/
│   ├── dev.sh                  # 开发环境启动
│   └── build.sh                # 生产构建
├── go.mod, go.sum
└── README.md
```

---

## 4. 数据库设计 (SQLite)

### 4.1 表结构

```sql
CREATE TABLE users (
    id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    username      TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE projects (
    id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id     TEXT NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    module_type TEXT NOT NULL CHECK(module_type IN ('magisk','ksu','apatch','hybrid')),
    description TEXT DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at  TEXT
);

CREATE TABLE project_files (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id  TEXT NOT NULL REFERENCES projects(id),
    path        TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, path)
);

CREATE TABLE build_tasks (
    id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    project_id  TEXT NOT NULL REFERENCES projects(id),
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK(status IN ('pending','running','success','failed','cancelled')),
    target      TEXT NOT NULL,
    log         TEXT DEFAULT '',
    artifact_path TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- FTS5 全文搜索
CREATE VIRTUAL TABLE projects_fts USING fts5(
    name, description, content='projects', content_rowid='rowid'
);
```

### 4.2 升迁到 PostgreSQL

切换点：`internal/database/database.go` 中的 `DB` 接口。
Lite 版：`*sql.DB` (SQLite)。Pro 版：`*pgxpool.Pool`。
接口签名一致，改一行 import 即可。

---

## 5. API 契约

Base: `/api/v1` · 格式: JSON · 认证: Bearer JWT

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/auth/register` | 注册 |
| POST | `/auth/login` | 登录 |
| GET | `/projects` | 项目列表 |
| POST | `/projects` | 创建项目 |
| GET | `/projects/{id}` | 项目详情 |
| PUT | `/projects/{id}` | 更新项目 |
| DELETE | `/projects/{id}` | 删除项目 |
| GET | `/projects/{id}/files` | 文件树 |
| GET | `/projects/{id}/files/*path` | 文件内容 |
| PUT | `/projects/{id}/files/*path` | 写入文件 |
| POST | `/projects/{id}/build` | 触发构建 |
| GET | `/builds/{id}` | 构建状态 |
| GET | `/builds/{id}/logs` | 构建日志 (SSE) |
| GET | `/builds/{id}/download` | 下载产物 |
| POST | `/ai/generate` | AI 生成模块 (SSE) |
| POST | `/ai/chat` | AI 对话 (SSE) |
| POST | `/ai/repair` | AI 诊断构建错误 |
| GET | `/templates` | 模块模板列表 |

---

## 6. 升迁路径（SCALING.md 已独立成章）

详见 `docs/SCALING.md`。核心原则：

- **数据库**: SQLite → PostgreSQL 16（改 DATABASE_URL 环境变量）
- **存储**: 本地磁盘 → MinIO（改 STORAGE_BACKEND 环境变量）
- **队列**: Channel → Redis + Asynq（改 QUEUE_BACKEND 环境变量）
- **搜索**: FTS5 → Meilisearch（开 MEILI_HOST 环境变量即启用）

**全在配置，不在一行代码。**

---

## 7. 性能目标

| 指标 | Lite 版 | Pro 版 |
|------|---------|--------|
| 内存基线 | < 60MB (不含构建) | ~300MB |
| 首屏加载 | < 30KB gzip JS | 同 |
| API (p95) | < 30ms | < 10ms |
| 并发构建 | 1-2 (按需) | 16+ |
| 并发用户 | 50+ | 1000+ |
