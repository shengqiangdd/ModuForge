#!/bin/bash
# ModuForge MD Prompts & Skills API Deployment Script
# This script deploys the new MD prompts and skills API to the server

set -e

echo "=== ModuForge MD Prompts & Skills API Deployment ==="
echo "Time: $(date)"
echo ""

# Configuration
SERVER="192.168.2.9"
USER="root"
REMOTE_DIR="/opt/moduforge"
LOCAL_BACKEND_DIR="./backend"
LOCAL_FRONTEND_DIR="./frontend"

echo "1. Building backend..."
cd "$LOCAL_BACKEND_DIR"
go build -o moduforge ./cmd/moduforge
echo "   Backend built successfully"

echo ""
echo "2. Building frontend..."
cd "../$LOCAL_FRONTEND_DIR"
npm run build
echo "   Frontend built successfully"

echo ""
echo "3. Deploying to server $SERVER..."
cd ..

# Create backup
echo "   Creating backup..."
ssh "$USER@$SERVER" "cd $REMOTE_DIR && sudo cp -r backend backend.bak.$(date +%Y%m%d%H%M%S) 2>/dev/null || true"

# Deploy backend binary
echo "   Deploying backend binary..."
scp "$LOCAL_BACKEND_DIR/moduforge" "$USER@$SERVER:$REMOTE_DIR/backend/"

# Deploy frontend assets
echo "   Deploying frontend assets..."
rsync -avz --delete "$LOCAL_FRONTEND_DIR/dist/" "$USER@$SERVER:$REMOTE_DIR/frontend/dist/"

# Restart the service
echo "   Restarting ModuForge service..."
ssh "$USER@$SERVER" "cd $REMOTE_DIR && sudo docker compose restart moduforge"

echo ""
echo "4. Deployment complete!"
echo ""
echo "New API Endpoints:"
echo "  GET    /api/v1/md-prompts          - List all MD prompt files"
echo "  GET    /api/v1/md-prompts/:name    - Get specific MD prompt content"
echo "  PUT    /api/v1/md-prompts/:name    - Update MD prompt content"
echo "  POST   /api/v1/md-prompts/:name/reset - Reset MD prompt to default"
echo "  POST   /api/v1/md-prompts/reload   - Reload all MD prompts"
echo ""
echo "  GET    /api/v1/skills              - List all available skills"
echo "  GET    /api/v1/skills/:name        - Get specific skill details"
echo "  POST   /api/v1/skills/:name/execute - Execute a skill"
echo ""
echo "Access the application at: http://$SERVER:8086"
echo ""
echo "Note: The MD prompts are stored in-memory and will be reset on server restart."
echo "      For persistent changes, you need to update the MD files in the source code."
