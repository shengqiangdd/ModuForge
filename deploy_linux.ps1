# Deploy Linux binary to server
# Usage: .\deploy_linux.ps1

$server = "192.168.2.9"
$port = 8086
$binaryPath = "C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\moduforge-server-linux"

Write-Host "=== ModuForge Linux Binary Deployment ===" -ForegroundColor Cyan
Write-Host "Server: ${server}:${port}" -ForegroundColor Yellow
Write-Host "Binary: $binaryPath" -ForegroundColor Yellow

# Check binary exists
if (-not (Test-Path $binaryPath)) {
    Write-Host "ERROR: Binary not found at $binaryPath" -ForegroundColor Red
    exit 1
}

$binarySize = (Get-Item $binaryPath).Length
Write-Host "Binary size: $([math]::Round($binarySize/1MB, 2)) MB" -ForegroundColor Green

# Option 1: Try SSH with key
Write-Host "`nAttempting SSH deployment..." -ForegroundColor Cyan

# Create a temporary script to run on server
$deployScript = @'
#!/bin/bash
set -e

echo "=== Deploying ModuForge Linux Binary ==="

# Backup old binary
if [ -f /app/moduforge-server ]; then
    cp /app/moduforge-server /app/moduforge-server.bak
    echo "Backed up old binary"
fi

# Stop container
docker stop moduforge 2>/dev/null || true

# Copy new binary
cp /tmp/moduforge-server-linux /app/moduforge-server
chmod +x /app/moduforge-server

# Start container
docker start moduforge

# Wait for health
sleep 3

# Check health
if curl -s http://localhost:8086/health | grep -q '"status":"ok"'; then
    echo "✓ Deployment successful!"
    echo "✓ Server is healthy"
else
    echo "✗ Health check failed, restoring backup..."
    cp /app/moduforge-server.bak /app/moduforge-server
    docker start moduforge
    echo "Restored old binary"
    exit 1
fi
'@

# Save deploy script locally
$deployScript | Out-File -FilePath "deploy_remote.sh" -Encoding UTF8

Write-Host "`nDeployment script created: deploy_remote.sh" -ForegroundColor Green
Write-Host "`nTo deploy manually:" -ForegroundColor Yellow
Write-Host "1. Copy binary to server: scp $binaryPath root@${server}:/tmp/moduforge-server-linux" -ForegroundColor White
Write-Host "2. Copy deploy script: scp deploy_remote.sh root@${server}:/tmp/" -ForegroundColor White
Write-Host "3. SSH to server and run: bash /tmp/deploy_remote.sh" -ForegroundColor White
Write-Host "`nOr provide SSH password and I'll attempt automated deployment." -ForegroundColor Cyan