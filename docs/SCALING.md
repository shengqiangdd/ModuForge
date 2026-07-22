# ModuForge Lite 升迁指南

> 从 1GB 单机版扩展到生产级集群的路径。

---

## 升迁路线

```mermaid
graph LR
    A[Lite: 1GB单机] --> B[Pro: Docker Compose]
    B --> C[Scale: K8s 集群]
    A --> D[降级: 嵌入到现有环境]
```

---

## 1. 数据库: SQLite → PostgreSQL 16

**改动**: 设置环境变量 `DATABASE_TYPE=postgres`

```bash
export DATABASE_TYPE=postgres
export DATABASE_URL="postgres://user:pass@host:5432/moduforge?sslmode=disable"
```

SQLite 数据迁移：
```bash
# 导出
sqlite3 data/moduforge.db .dump > dump.sql
# 导入 PostgreSQL（需手动调整语法差异）
psql $DATABASE_URL < dump.sql
```

---

## 2. 存储: 本地磁盘 → MinIO

**改动**: 设置 `STORAGE_BACKEND=s3`

```bash
export STORAGE_BACKEND=s3
export S3_ENDPOINT=http://minio:9000
export S3_ACCESS_KEY=xxx
export S3_SECRET_KEY=xxx
export S3_BUCKET=moduforge
```

---

## 3. 队列: Channel → Redis

**改动**: 设置 `QUEUE_BACKEND=redis`

```bash
export QUEUE_BACKEND=redis
export REDIS_URL=redis://redis:6379/0
```

---

## 4. 搜索: FTS5 → Meilisearch

**改动**: 设置 `MEILI_HOST`

```bash
export MEILI_HOST=http://meilisearch:7700
export MEILI_MASTER_KEY=xxx
```

---

## 5. 前端: embed → CDN

前端独立部署：
```bash
cd frontend && pnpm build
# 产物输出到 dist/，丢到 CDN 或 Nginx
```

后端：关闭 embed 模式，设置 `FRONTEND_URL` 指向 CDN。

---

## 6. 部署形态变迁

### Lite (1GB) — 当前
```bash
# 一个二进制 + sqlite 文件
./moduforge
```

### Pro (Docker Compose)
```yaml
services:
  postgres:    # 数据库
  redis:       # 缓存/队列
  minio:       # 对象存储
  moduforge:   # API + Web
  builder:     # 单独的构建工作者
```

### Scale (K8s)
```
API 无状态部署 3+ 副本
构建队列独立 Deployment
数据库托管 RDS
存储托管 S3
```
