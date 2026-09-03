# Cache bust: 1788278610
# ============================================================
# ModuForge — Multi-stage Build (Go + Rust + Android NDK)
# ============================================================
# syntax=docker/dockerfile:1

# Stage 0: Frontend build
FROM node:22-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm install --include=dev
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
FROM alpine:3.20

LABEL org.opencontainers.image.title="ModuForge"
LABEL org.opencontainers.image.description="Modular software forge platform"
LABEL org.opencontainers.image.source="https://github.com/moduforge/ModuForge"

# --- Runtime deps + gcompat (NDK glibc compatibility on Alpine) ---
# android-tools provides adb (needed by the ADB module); without it every ADB
# feature fails with "adb: executable file not found in $PATH".
# openjdk17 is needed for Android APK compilation (Gradle/Android SDK).
RUN apk add --no-cache wget ca-certificates tzdata openssl curl bash xz zip unzip cmake make gcompat gcc musl-dev android-tools openjdk17 \
    && addgroup -S moduforge && adduser -S moduforge -G moduforge -u 1000

# --- Go toolchain (for cross-compiling Go source to Android binaries) ---
ENV GOPATH=/usr/local/go PATH=$PATH:/usr/local/go/bin
RUN wget -q https://go.dev/dl/go1.24.5.linux-amd64.tar.gz -O /tmp/go.tar.gz \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz \
    && /usr/local/go/bin/go version

# --- Rust toolchain ---
ENV CARGO_HOME=/usr/local/cargo RUSTUP_HOME=/usr/local/rustup
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | \
    env CARGO_HOME=/usr/local/cargo RUSTUP_HOME=/usr/local/rustup sh -s -- -y \
    --default-toolchain stable \
    && /usr/local/cargo/bin/rustup target add aarch64-linux-android \
    && /usr/local/cargo/bin/rustup component add clippy rustfmt \
    && rm -rf /usr/local/rustup/toolchains/*/share/doc \
    && chown -R moduforge:moduforge /usr/local/cargo /usr/local/rustup

# --- Android NDK r27c (trimmed: clang + sysroot) ---
# Kept: bin/ (clang), lib/ (LLVM/clang runtime), sysroot/ (headers+libs)
# Removed: liblldb.so (~165MB, debugger — not needed for compilation),
#          x86_64-w64-windows-gnu (~31MB, Windows cross-compile — Android-only builds)
RUN mkdir -p /opt/android-ndk && cd /tmp \
    && wget -q https://dl.google.com/android/repository/android-ndk-r27c-linux.zip -O ndk.zip \
    && unzip -q ndk.zip -d ndk_tmp \
    && mv ndk_tmp/android-ndk-r27c/toolchains/llvm/prebuilt/linux-x86_64/bin /opt/android-ndk/bin \
    && mv ndk_tmp/android-ndk-r27c/toolchains/llvm/prebuilt/linux-x86_64/lib /opt/android-ndk/lib \
    && mv ndk_tmp/android-ndk-r27c/toolchains/llvm/prebuilt/linux-x86_64/sysroot /opt/android-ndk/sysroot \
    && rm -rf /opt/android-ndk/lib/liblldb.so \
    && rm -rf /opt/android-ndk/lib/x86_64-w64-windows-gnu \
    && rm -rf /tmp/ndk.zip /tmp/ndk_tmp \
    && chmod -R 755 /opt/android-ndk \
    \
    # --- NDK 瘦身：项目只编译 aarch64(arm64) / arm(armeabi-v7a) 两个目标 --- \
    # 1) 删掉静态分析工具（编译不调用）clang-check / clang-tidy (~120MB) \
    && rm -f /opt/android-ndk/bin/clang-check /opt/android-ndk/bin/clang-tidy \
    # 2) 删掉非目标架构的 clang 运行时目录（i386/i686/x86_64/riscv64） \
    && find /opt/android-ndk/lib/clang -maxdepth 4 -type d \( -name 'i386*' -o -name 'i686*' -o -name 'x86_64*' -o -name 'riscv64*' -o -name 'windows*' \) -exec rm -rf {} + 2>/dev/null || true \
    # 3) 只保留链接必需的 libclang_rt.builtins-*，删 sanitizer/profile 等运行时 (~590MB) \
    && find /opt/android-ndk/lib/clang/*/lib/linux -maxdepth 1 -type f ! -name 'libclang_rt.builtins-*' -delete 2>/dev/null || true \
    # 4) 删 sysroot 里非目标架构的系统库 (riscv64/x86_64/i686, ~74MB) \
    && rm -rf /opt/android-ndk/sysroot/usr/lib/riscv64-linux-android \
             /opt/android-ndk/sysroot/usr/lib/x86_64-linux-android \
             /opt/android-ndk/sysroot/usr/lib/i686-linux-android

# --- Android SDK (for APK compilation with Gradle) ---
ENV ANDROID_HOME=/opt/android-sdk ANDROID_SDK_ROOT=/opt/android-sdk
ENV PATH="${ANDROID_HOME}/cmdline-tools/latest/bin:${ANDROID_HOME}/build-tools/34.0.0:${ANDROID_HOME}/platform-tools:${PATH}"
RUN mkdir -p ${ANDROID_HOME}/cmdline-tools && cd /tmp \
    && wget -q https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip -O sdk-tools.zip \
    && unzip -q sdk-tools.zip -d sdk-tmp \
    && mv sdk-tmp/cmdline-tools ${ANDROID_HOME}/cmdline-tools/latest \
    && rm -rf sdk-tools.zip sdk-tmp \
    && yes | sdkmanager --licenses > /dev/null 2>&1 || true \
    && sdkmanager --install \
        "build-tools;34.0.0" \
        "platforms;android-34" \
        "platform-tools" \
    > /dev/null 2>&1 \
    && rm -rf ${ANDROID_HOME}/cmdline-tools/latest/.promisor

# --- Gradle 8.7 (for Android APK builds) ---
ENV GRADLE_HOME=/opt/gradle/gradle-8.7
ENV PATH="${GRADLE_HOME}/bin:${PATH}"
RUN mkdir -p /opt/gradle && cd /tmp \
    && wget -q https://services.gradle.org/distributions/gradle-8.7-bin.zip -O gradle.zip \
    && unzip -q gradle.zip -d /opt/gradle \
    && rm -f gradle.zip \
    && echo "Gradle installed"

# --- Copy backend binary ---
COPY --from=backend-builder --chown=moduforge:moduforge /server /app/server

# --- Copy backend docs (OpenAPI spec etc.) ---
COPY --from=backend-builder --chown=moduforge:moduforge /app/docs /app/docs

# --- Copy database migrations ---
COPY --from=backend-builder --chown=moduforge:moduforge /app/migrations /app/migrations

# --- Copy frontend dist ---
COPY --from=frontend-builder --chown=moduforge:moduforge /app/dist /app/dist

# --- Entrypoint ---
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# --- Runtime defaults ---
ENV PORT=:8080 \
    DB_PATH=/data/moduforge.db \
    BUILD_DIR=/data/builds \
    MODULES_DIR=/data/modules \
    PROJECTS_DIR=/data/projects \
    GIN_MODE=release \
    DIST_DIR=/app/dist \
    PATH="/usr/local/cargo/bin:${ANDROID_HOME}/cmdline-tools/latest/bin:${ANDROID_HOME}/build-tools/34.0.0:${ANDROID_HOME}/platform-tools:${GRADLE_HOME}/bin:${PATH}" \
    CARGO_HOME="/usr/local/cargo" \
    RUSTUP_HOME="/usr/local/rustup" \
    ANDROID_HOME="/opt/android-sdk" \
    ANDROID_SDK_ROOT="/opt/android-sdk" \
    GRADLE_HOME="/opt/gradle" \
    JAVA_HOME="/usr/lib/jvm/java-17-openjdk"

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT}/health || exit 1

RUN mkdir -p /data /app/uploads && chown -R moduforge:moduforge /data /app/uploads

EXPOSE 8080

# Run as root so the entrypoint can adjust volume ownership if needed (e.g. after a host backup).
# The entrypoint script handles JWT setup before starting the server.
# Use /bin/sh to execute the entrypoint so +x permission is not required.
ENTRYPOINT ["/bin/sh", "/docker-entrypoint.sh"]
