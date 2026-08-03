#!/bin/bash
# ModuForge Agent Enhancements Deployment Script
# This script deploys the new Agent enhancements to the Docker container

set -e

echo "🚀 ModuForge Agent Enhancements Deployment"
echo "=========================================="

# Check if running on host machine
if [ ! -f "/root/moduforge/docker-compose.yml" ]; then
    echo "❌ This script must be run on the host machine (192.168.2.9)"
    echo "   Please copy this script to the host and run it there."
    exit 1
fi

# Stop existing container
echo "📦 Stopping existing container..."
cd /root/moduforge
docker compose down

# Rebuild the image
echo "🔨 Rebuilding Docker image with new enhancements..."
docker compose build --no-cache

# Start the container
echo "🚀 Starting container..."
docker compose up -d

# Wait for health check
echo "⏳ Waiting for container to be healthy..."
sleep 10

# Check health
if docker compose ps | grep -q "healthy"; then
    echo "✅ Container is healthy!"
    echo ""
    echo "Enhancements deployed successfully:"
    echo "  1. Todo Manager - Task tracking and progress management"
    echo "  2. Task Delegator - Sub-agent spawning for parallel work"
    echo "  3. Context Manager - Memory and context management"
    echo "  4. Skill Registry - Dynamic skill management"
    echo "  5. Enhanced System Prompt - Better tool usage instructions"
    echo ""
    echo "Access ModuForge at: http://localhost:8086"
else
    echo "⚠️  Container may not be healthy yet. Check logs with:"
    echo "   docker compose logs -f"
fi