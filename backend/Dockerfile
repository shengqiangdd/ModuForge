# ============================================================
# ModuForge — Multi-stage Build (Go + Rust + Android NDK)
# ============================================================
# syntax=docker/dockerfile:1

# Stage 0: Frontend build
FROM node:22-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --include=dev
COPY frontend/ .
RUN --mount=type=cache,target=/app/node_modules/.cache npm run build

# ============================================================
# Stage 1: Go backend (CGO_ENABLED=1 for go-sqlite3)
FROM golang:1.25-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY backend/ .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -ldflags="-s -w" -trimpath -o /server ./cmd/moduforge/

# ============================================================
# Stage 2: Runtime (Go compiler + Rust + Android NDK)
FROM golang:1.25-alpine

LABEL org.opencontainers.image.title="ModuForge"
LABEL org.opencontainers.image.description="Modular software forge platform"
LABEL org.opencontainers.image.source="https://github.com/moduforge/ModuForge"

# --- Runtime deps + gcompat (NDK glibc compatibility on Alpine) ---
RUN apk add --no-cache wget ca-certificates tzdata openssl curl bash xz zip unzip cmake make gcompat gcc musl-dev \
    && addgroup -S moduforge && adduser -S moduforge -G moduforge

# --- Rust toolchain ---
ENV CARGO_HOME=/usr/local/cargo RUSTUP_HOME=/usr/local/rustup
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | \
    env CARGO_HOME=/usr/local/cargo RUSTUP_HOME=/usr/local/rustup sh -s -- -y \
    --default-toolchain stable \
    && /usr/local/cargo/bin/rustup target add aarch64-linux-android \
    && /usr/local/cargo/bin/rustup component add clippy rustfmt \
    && chown -R moduforge:moduforge /usr/local/cargo /usr/local/rustup

# --- Android NDK r27c (trimmed: clang + sysroot) ---
RUN mkdir -p /opt/android-ndk && cd /tmp \
    && wget -q https://dl.google.com/android/repository/android-ndk-r27c-linux.zip -O ndk.zip \
    && unzip -q ndk.zip -d ndk_tmp \
    && mv ndk_tmp/android-ndk-r27c/toolchains/llvm/prebuilt/linux-x86_64/bin /opt/android-ndk/bin \
    && mv ndk_tmp/android-ndk-r27c/toolchains/llvm/prebuilt/linux-x86_64/lib /opt/android-ndk/lib \
    && mv ndk_tmp/android-ndk-r27c/toolchains/llvm/prebuilt/linux-x86_64/sysroot /opt/android-ndk/sysroot \
    && rm -rf /tmp/ndk.zip /tmp/ndk_tmp \
    && chmod -R 755 /opt/android-ndk

# --- Copy backend binary ---
COPY --from=backend-builder --chown=moduforge:moduforge /server /app/server

# --- Copy frontend dist ---
COPY --from=frontend-builder --chown=moduforge:moduforge /app/dist /app/dist

# --- Entrypoint ---
COPY --chown=moduforge:moduforge docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# --- Runtime defaults ---
ENV PORT=:8080 \
    DB_PATH=/data/moduforge.db \
    BUILD_DIR=/data/builds \
    MODULES_DIR=/data/modules \
    PROJECTS_DIR=/data/projects \
    GIN_MODE=release \
    DIST_DIR=/app/dist \
    PATH="/usr/local/cargo/bin:/usr/local/go/bin:${PATH}" \
    CARGO_HOME="/usr/local/cargo" \
    RUSTUP_HOME="/usr/local/rustup"

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1

RUN mkdir -p /data /app/uploads && chown -R moduforge:moduforge /data /app/uploads

EXPOSE 8080
USER moduforge
ENTRYPOINT ["/docker-entrypoint.sh"]
