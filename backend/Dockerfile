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
# Stage 1: Go 后端编译 (CGO_ENABLED=0 → 纯静态二进制)
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app
# 先拷贝依赖文件，利用 Docker 缓存层
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
# 静态编译：CGO_ENABLED=0 避免 glibc 依赖，-trimpath 缩小体积，-s -w 剥离符号表
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /server ./cmd/moduforge/

# ============================================================
# Stage 2: 运行时镜像 (最小化 Alpine)
FROM alpine:3.20

# 安全元数据标签
LABEL org.opencontainers.image.title="ModuForge"
LABEL org.opencontainers.image.description="Modular software forge platform"
LABEL org.opencontainers.image.source="https://github.com/moduforge/ModuForge"
LABEL org.opencontainers.image.vendor="ModuForge"
LABEL org.opencontainers.image.licenses="MIT"

# 安装运行时的最小依赖（仅 wget 用于健康检查）
RUN apk add --no-cache wget ca-certificates tzdata && \
    addgroup -S app && adduser -S app -G app

# 从构建阶段拷贝产物
COPY --from=backend-builder --chown=app:app /server /server
COPY --from=frontend-builder --chown=app:app /app/dist /app/dist

# 运行时配置默认值
ENV PORT=8080 \
    DB_PATH=/data/moduforge.db \
    BUILD_DIR=/data/builds \
    MODULES_DIR=/data/modules \
    PROJECTS_DIR=/data/projects \
    GIN_MODE=release \
    DIST_DIR=/app/dist

# 健康检查：通过 /health HTTP 端点
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1

# 服务端口
EXPOSE 8080

# 非 root 用户运行
USER app

ENTRYPOINT ["/server"]