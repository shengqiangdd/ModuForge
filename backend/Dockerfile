# ============================================================
# ModuForge — 多阶段构建 (Multi-stage Build)
# ============================================================
# Stage 0: Frontend 构建
FROM node:22-alpine AS frontend-builder

WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --include=dev
COPY frontend/ .
RUN npm run build

# ============================================================
# Stage 1: Go 后端编译 (CGO_ENABLED=1 for go-sqlite3)
FROM golang:1.25-alpine AS backend-builder

# 安装 gcc 和 musl-dev（go-sqlite3 需要 CGO）
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
# 先拷贝依赖文件，利用 Docker 缓存层
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
# CGO_ENABLED=1 是必须的（go-sqlite3 依赖 CGO），-trimpath 缩小体积，-s -w 剥离符号表
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -trimpath -o /server ./cmd/moduforge/

# ============================================================
# Stage 2: 运行时镜像 (最小化 Alpine)
FROM alpine:3.20

# 安全元数据标签
LABEL org.opencontainers.image.title="ModuForge"
LABEL org.opencontainers.image.description="Modular software forge platform"
LABEL org.opencontainers.image.source="https://github.com/moduforge/ModuForge"
LABEL org.opencontainers.image.vendor="ModuForge"
LABEL org.opencontainers.image.licenses="MIT"

# 安装运行时的最小依赖
# wget: 健康检查 | openssl: entrypoint 自动生成 JWT_SECRET
RUN apk add --no-cache wget ca-certificates tzdata openssl && \
    addgroup -S moduforge && adduser -S moduforge -G moduforge

# 从构建阶段拷贝产物
COPY --from=backend-builder --chown=moduforge:moduforge /server /server
COPY --from=frontend-builder --chown=moduforge:moduforge /app/dist /app/dist

# 入口脚本：自动处理 JWT_SECRET 等环境变量
COPY --chown=moduforge:moduforge docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# 运行时配置默认值
ENV PORT=:8080 \
    DB_PATH=/data/moduforge.db \
    BUILD_DIR=/data/builds \
    MODULES_DIR=/data/modules \
    PROJECTS_DIR=/data/projects \
    GIN_MODE=release \
    DIST_DIR=/app/dist

# 健康检查：通过 /health HTTP 端点
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1

# 创建数据目录并设置权限（非 root 用户 moduforge 需要写入权限）
RUN mkdir -p /data /app/uploads && chown -R moduforge:moduforge /data /app/uploads

# 服务端口
EXPOSE 8080

# 非 root 用户运行
USER moduforge

ENTRYPOINT ["/docker-entrypoint.sh"]