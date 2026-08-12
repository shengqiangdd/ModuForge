#!/bin/bash
# ModuForge Deployment Script - runs on server
# Usage: deploy_remote.sh

set -e

echo "========================================="
echo "  ModuForge Deployment"
echo "========================================="

# Step 1: Stop old build container
echo "[1/5] Stopping old build container..."
docker rm -f moduforge-build 2>/dev/null || true

# Step 2: Build Go backend
echo "[2/5] Building Go backend..."
docker run --name moduforge-build \
    -v /app/working/workspaces/default/ModuForge/backend:/src \
    -w /src \
    golang:1.25-alpine \
    sh -c "apk add --no-cache gcc musl-dev && CGO_ENABLED=1 go build -ldflags='-s -w' -trimpath -o /tmp/server ./cmd/moduforge/"

# Step 3: Extract binary
echo "[3/5] Extracting binary..."
docker cp moduforge-build:/tmp/server /tmp/server
docker cp moduforge:/app/dist /tmp/moduforge_dist

# Step 4: Create Dockerfile
echo "[4/5] Creating Dockerfile..."
cat > /tmp/Dockerfile.deploy << 'HEREDOC'
FROM alpine:3.20

RUN apk add --no-cache wget ca-certificates tzdata openssl curl bash xz zip unzip cmake make gcc musl-dev && \
    addgroup -S moduforge && adduser -S moduforge -G moduforge

COPY server /server
RUN chmod +x /server

COPY dist/ /app/dist/

COPY entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENV PORT=:8080 \
    DB_PATH=/data/moduforge.db \
    BUILD_DIR=/data/builds \
    MODULES_DIR=/data/modules \
    PROJECTS_DIR=/data/projects \
    GIN_MODE=release \
    DIST_DIR=/app/dist

RUN mkdir -p /data /app/uploads && chown -R moduforge:moduforge /data /app /app/uploads

EXPOSE 8080
USER moduforge
ENTRYPOINT ["/docker-entrypoint.sh"]
HEREDOC

cat > /tmp/entrypoint.sh << 'HEREDOC'
#!/bin/sh
set -e

log() {
  echo "[entrypoint] $(date '+%Y-%m-%d %H:%M:%S') $*"
}

if [ -n "$JWT_SECRET" ]; then
  log "Using JWT_SECRET from environment variable"
elif [ -f /data/.env ] && grep -q "^JWT_SECRET=." /data/.env 2>/dev/null; then
  export JWT_SECRET=$(grep "^JWT_SECRET=" /data/.env | head -1 | cut -d= -f2-)
  log "Loaded JWT_SECRET from /data/.env"
else
  JWT_SECRET=$(openssl rand -hex 32)
  export JWT_SECRET
  echo "JWT_SECRET=${JWT_SECRET}" >> /data/.env
  log "Generated random JWT_SECRET and saved to /data/.env"
fi

log "Starting ModuForge on port ${PORT:-:8080}..."
exec /server "$@"
HEREDOC

# Build image
echo "[5/5] Building Docker image..."
docker build -t moduforge:latest -f /tmp/Dockerfile.deploy /tmp

# Restart container
echo "Restarting container..."
docker stop moduforge 2>/dev/null || true
docker rm moduforge 2>/dev/null || true

docker run -d \
    --name moduforge \
    --restart unless-stopped \
    -p 8086:8080 \
    -e PORT=:8080 \
    -e DATABASE_PATH=/data/moduforge.db \
    -e BUILD_DIR=/data/builds \
    -e MODULES_DIR=/data/modules \
    -e PROJECTS_DIR=/data/projects \
    -e GIN_MODE=release \
    -e TZ=Asia/Shanghai \
    -v moduforge_data:/data \
    -v moduforge_uploads:/app/uploads \
    moduforge:latest

# Cleanup
docker rm -f moduforge-build 2>/dev/null || true
rm -rf /tmp/server /tmp/dist /tmp/moduforge_dist /tmp/Dockerfile.deploy /tmp/entrypoint.sh

echo ""
echo "========================================="
echo "  Deployment Complete!"
echo "  Access: http://192.168.2.9:8086"
echo "========================================="
