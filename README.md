# ModuForge Lite

> 1GB 内存可运行的 Android 模块可视化 AI 开发平台

**核心理念**: 单二进制 · 零外部依赖 · 升迁无痛

## 快速开始 (Docker)

```bash
# 一键启动
docker-compose up -d --build

# 查看日志
docker-compose logs -f

# 停止
docker-compose down
```

访问 http://localhost:3000

## 本地开发

```bash
# 后端
cd backend
go mod download
go run ./cmd/moduforge/

# 前端（新终端）
cd frontend
npm install
npm run dev
```

后端: http://localhost:8080
前端: http://localhost:5173

## 系统要求

- **最低**: 1GB RAM, 1 CPU, 200MB 磁盘
- **推荐**: 2GB RAM, 2 CPU, 1GB 磁盘

## 功能

- AI 自然语言生成模块代码
- 在线代码编辑器 (CodeMirror 6)
- 云端构建 (Docker 可选)
- WebUI 实时预览
- 开源仓库追踪与 AI 改造
- PWA 移动端 (MD3 设计)
- SQLite 持久化存储
- WebSocket 实时通信

## 文档

| 文档 | 用途 |
|------|------|
| [DESIGN_DOC.md](DESIGN_DOC.md) | 完整架构设计 |
| [docs/AGENTS.md](docs/AGENTS.md) | AI Agent 开发指南 |
| [docs/SCALING.md](docs/SCALING.md) | 升迁指南 |

## 技术栈

| 后端 | 前端 | 数据库 |
|------|------|--------|
| Go + Fiber v3 | Svelte 5 + MD3 | SQLite (→PostgreSQL) |
