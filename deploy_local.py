#!/usr/bin/env python3
"""
Deploy existing local build artifacts to server.
Use this when you have already built locally and just need to deploy.
"""
import os
import shutil
import tempfile
import tarfile
from pathlib import Path
import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
PROJECT_DIR = Path(__file__).parent  # ModuForge directory

def prepare_and_deploy():
    """Package local build and deploy."""
    print("=" * 60)
    print("ModuForge Local Deployment")
    print("=" * 60)
    
    # Check sources
    frontend_dist = PROJECT_DIR / "frontend" / "dist"
    backend_binary = Path("/tmp/moduforge-server-linux")
    
    if not frontend_dist.exists():
        print(f"ERROR: Frontend dist not found: {frontend_dist}")
        return False
    
    if not backend_binary.exists():
        print(f"ERROR: Backend binary not found: {backend_binary}")
        return False
    
    # Create package
    print("\n=== Creating Package ===")
    pkg_dir = Path(tempfile.mkdtemp(prefix="moduforge_"))
    dist_dir = pkg_dir / "dist"
    dist_dir.mkdir()
    
    # Copy frontend (fix path separators)
    print("Copying frontend...")
    for item in frontend_dist.rglob("*"):
        if item.is_file():
            # Skip unnecessary files
            if item.name.endswith('.map') or item.name in ('manifest.json',):
                continue
            if item.name == 'MaterialSymbolsOutlined.ttf':
                continue  # Large font, keep woff2 only
            
            rel_path = str(item.relative_to(frontend_dist)).replace("\\", "/")
            dest = dist_dir / rel_path
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(item, dest)
    
    # Copy backend
    print("Copying backend...")
    shutil.copy2(backend_binary, pkg_dir / "server")
    (pkg_dir / "server").chmod(0o755)
    
    # Create tar
    tar_path = Path(tempfile.mktemp(suffix=".tar.gz"))
    with tarfile.open(tar_path, "w:gz") as tar:
        tar.add(pkg_dir, arcname=".")
    
    size_mb = tar_path.stat().st_size / (1024 * 1024)
    print(f"Package: {tar_path.name} ({size_mb:.1f} MB)")
    
    # Deploy
    print("\n=== Deploying ===")
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    sftp = ssh.open_sftp()
    sftp.put(str(tar_path), "/tmp/moduforge-deploy.tar.gz")
    sftp.close()
    
    commands = [
        f"docker stop {CONTAINER} 2>/dev/null || true",
        "rm -rf /tmp/moduforge-deploy && mkdir -p /tmp/moduforge-deploy",
        "tar -xzf /tmp/moduforge-deploy.tar.gz -C /tmp/moduforge-deploy",
        f"docker start {CONTAINER}",
        f"docker cp /tmp/moduforge-deploy/dist/. {CONTAINER}:/app/dist/",
        f"docker cp /tmp/moduforge-deploy/server {CONTAINER}:/app/server",
        f"docker exec {CONTAINER} chmod +x /app/server",
        f"docker restart {CONTAINER}",
        "sleep 3",
    ]
    
    for cmd in commands:
        print(f"  > {cmd}")
        stdin, stdout, stderr = ssh.exec_command(cmd)
        stdout.channel.recv_exit_status()
    
    # Verify
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"\nHealth: {health}")
    
    ssh.close()
    shutil.rmtree(pkg_dir, ignore_errors=True)
    tar_path.unlink(missing_ok=True)
    
    return '"status":"ok"' in health

if __name__ == "__main__":
    import sys
    success = prepare_and_deploy()
    sys.exit(0 if success else 1)
