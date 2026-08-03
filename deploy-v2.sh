#!/bin/bash

# Deploy Memory V2 and Skill Manager updates

HOST="192.168.2.9"
USER="admin"
CONTAINER="moduforge-app-1"

echo "=== Deploying Memory V2 and Skill Manager ==="

# Build backend
echo "[1/3] Building backend..."
cd /app/working/workspaces/default/ModuForge/backend
/usr/local/go/bin/go build -o /tmp/moduforge-server ./cmd/moduforge 2>&1
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi
echo "Build successful"

# Copy binary to host
echo "[2/3] Copying to host..."
sshpass -p 'csq0216' scp -o StrictHostKeyChecking=no /tmp/moduforge-server ${USER}@${HOST}:/tmp/moduforge-server 2>&1
if [ $? -ne 0 ]; then
    echo "Copy failed!"
    exit 1
fi

# Deploy to container
echo "[3/3] Deploying to container..."
sshpass -p 'csq0216' ssh -o StrictHostKeyChecking=no ${USER}@${HOST} << EOF
docker cp /tmp/moduforge-server ${CONTAINER}:/app/backend
docker restart ${CONTAINER}
sleep 3
docker logs --tail 20 ${CONTAINER}
EOF

if [ $? -ne 0 ]; then
    echo "Deploy failed!"
    exit 1
fi

echo ""
echo "=== Deployment Complete ==="
echo "Memory V2 and Skill Manager features are now available"
echo ""
echo "New features:"
echo "1. Memory V2: Semantic search, tiered storage (short/long/archive), consolidation"
echo "2. Skill Manager: Version control, dependencies, rollback, clone, export/import"
echo "3. Enhanced system prompt with usage examples"