# Frontend build stage
FROM node:22-alpine AS frontend-builder

WORKDIR /app

# Step 1: deps — only reruns when package.json changes
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Step 2: source + build — reruns when any source file changes
COPY frontend/ .
RUN npm run build

# Backend build stage — use Debian for glibc compatibility with runtime
FROM golang:1.25-bookworm AS backend-builder

RUN apt-get update && apt-get install -y --no-install-recommends gcc && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go mod download

COPY backend/ .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -o /app/server ./cmd/moduforge/

# NDK extraction stage — extract compiler + full sysroot for Rust/C++ cross-compilation
FROM debian:bookworm-slim AS ndk-extractor

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl unzip && rm -rf /var/lib/apt/lists/*

ENV ANDROID_NDK_VERSION=r27c
ENV NDK_URL=https://dl.google.com/android/repository/android-ndk-${ANDROID_NDK_VERSION}-linux.zip
RUN --mount=type=cache,target=/ndk-cache,sharing=locked \
    CACHE_FILE="/ndk-cache/android-ndk-${ANDROID_NDK_VERSION}-linux.zip" && \
    if [ -f "$CACHE_FILE" ]; then \
        echo "✅ Using cached NDK zip" && \
        cp "$CACHE_FILE" /tmp/ndk.zip; \
    else \
        echo "⬇️  Downloading NDK from GitHub..." && \
        curl -L -o /tmp/ndk.zip "${NDK_URL}" && \
        cp /tmp/ndk.zip "$CACHE_FILE" && \
        echo "✅ Downloaded and cached"; \
    fi && \
    echo "Size: $(du -sh /tmp/ndk.zip | cut -f1)" && \
    unzip -q /tmp/ndk.zip -d /tmp && \
    cp -a /tmp/android-ndk-*/toolchains/llvm/prebuilt/linux-x86_64 /ndk && \
    rm -rf /tmp/android-ndk-* /tmp/ndk.zip && \
    echo "✅ NDK ready: $(du -sh /ndk | cut -f1)"

# Rust toolchain extraction stage — pre-install stable + ARM64 target (~200MB)
# Use rust:slim-bookworm (Debian) for glibc compatibility with Debian runtime
FROM rust:slim-bookworm AS rust-extractor

# Install ARM64 cross-compilation target
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && \
    rustup target add aarch64-linux-android && \
    rustc --version && cargo --version && \
    # Strip to reduce size
    find ${CARGO_HOME} -name "*.rlib" -exec strip --strip-debug {} + 2>/dev/null || true && \
    echo "✅ Rust extracted: $(du -sh ${CARGO_HOME} ${RUSTUP_HOME} | tail -1)"

# Runtime stage — use Debian for glibc compatibility with NDK clang
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    wget ca-certificates unzip curl xz-utils gcc g++ bash procps libxml2 && \
    rm -rf /var/lib/apt/lists/*

# Install adb (Android Debug Bridge) for device management
RUN apt-get update && apt-get install -y --no-install-recommends adb || \
    (wget -q -O /tmp/platform-tools.zip "https://dl.google.com/android/repository/platform-tools-latest-linux.zip" && \
     unzip -q /tmp/platform-tools.zip -d /opt && \
     rm /tmp/platform-tools.zip && \
     ln -sf /opt/platform-tools/adb /usr/local/bin/adb) || \
    echo "WARN: adb install failed — device management will be unavailable"

# Copy full NDK from extractor stage for Rust/C++ cross-compilation
ENV ANDROID_NDK=/opt/android-ndk
COPY --from=ndk-extractor /ndk ${ANDROID_NDK}

# Copy Go compiler from builder stage (~150MB) for cross-compiling user modules
ENV GOROOT=/usr/local/go
ENV PATH="${GOROOT}/bin:${PATH}"
COPY --from=backend-builder /usr/local/go ${GOROOT}

# Copy pre-installed Rust toolchain (~200MB) with ARM64 target
# Keep at default paths to preserve symlinks; update Go code to match
COPY --from=rust-extractor /usr/local/cargo /usr/local/cargo
COPY --from=rust-extractor /usr/local/rustup /usr/local/rustup
ENV CARGO_HOME=/usr/local/cargo
ENV RUSTUP_HOME=/usr/local/rustup
ENV PATH="${CARGO_HOME}/bin:${PATH}"
RUN rustc --version && cargo --version && echo "✅ Rust ready"

# Verify toolchains are usable
RUN echo "--- Toolchain Check ---" && \
    go version && \
    rustc --version && \
    cargo --version && \
    test -f ${ANDROID_NDK}/bin/aarch64-linux-android21-clang && \
    echo "✅ NDK ready: $(du -sh ${ANDROID_NDK} | cut -f1)" && \
    echo "✅ All toolchains OK" || \
    (echo "❌ Toolchain check FAILED" && exit 1)

WORKDIR /app

COPY --from=backend-builder /app/server .
COPY --from=frontend-builder /app/dist /app/dist
RUN mkdir -p /app/data /app/default-data

# Copy entrypoint script
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["./server"]
