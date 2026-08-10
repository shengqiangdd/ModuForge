#!/usr/bin/env python3
"""
ModuForge Full Deployment Script
Handles: build, package, deploy via Python (no Windows SSH issues)
"""
import os
import sys
import shutil
import subprocess
import tarfile
import tempfile
from pathlib import Path
import paramiko

# Configuration
HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
PROJECT_DIR = Path(__file__).parent  # ModuForge directory

# Files/dirs to exclude from deployment
EXCLUDE_PATTERNS = {
    'node_modules', '.git', '__pycache__', '*.pyc', '.pytest_cache',
    'target', '.vscode', '.idea', 'test-results', 'tmp',
    'docs', '.github', 'coverage', 'dist', 'build',
    'package-lock.json', 'package-lock.json', '*.map',
    '.env', '.env.local', '*.log'
}

# Files to exclude from frontend dist
FRONTEND_EXCLUDE = {
    '*.map', 'manifest.json', 'sw.js', 'icon-*.svg',
    'MaterialSymbolsOutlined.ttf'  # Large font, keep only woff2
}

def clean_dir(path: Path):
    """Remove directory if exists."""
    if path.exists():
        shutil.rmtree(path)
    path.mkdir(parents=True, exist_ok=True)

def build_frontend():
    """Build frontend and return dist path."""
    print("\n=== Building Frontend ===")
    frontend_dir = PROJECT_DIR / "frontend"
    
    # Clean previous build
    dist_dir = frontend_dir / "dist"
    clean_dir(dist_dir)
    
    # Install deps if needed
    if not (frontend_dir / "node_modules").exists():
        print("Installing frontend dependencies...")
        subprocess.run(["npm", "install"], cwd=frontend_dir, check=True)
    
    # Build
    print("Building frontend...")
    result = subprocess.run(
        ["npm", "run", "build"],
        cwd=frontend_dir,
        capture_output=True,
        text=True
    )
    
    if result.returncode != 0:
        print(f"Build failed: {result.stderr}")
        return None
    
    print(f"Frontend built: {dist_dir}")
    return dist_dir

def build_backend():
    """Build backend binary for Linux."""
    print("\n=== Building Backend ===")
    backend_dir = PROJECT_DIR / "backend"
    output = Path("/tmp/moduforge-server-linux")
    
    # Build in Docker for correct Linux binary
    print("Building backend in Docker...")
    docker_cmd = f"""docker run --rm \
        -v {backend_dir}:/src \
        -v /tmp:/out \
        golang:1.21-alpine \
        sh -c "cd /src && go build -ldflags='-s -w' -o /out/moduforge-server-linux ./cmd/moduforge"
    """
    
    result = subprocess.run(docker_cmd, shell=True, capture_output=True, text=True)
    
    if result.returncode != 0:
        print(f"Docker build failed: {result.stderr}")
        # Try local build if Docker fails
        print("Trying local build...")
        subprocess.run(
            ["go", "build", "-ldflags=-s -w", "-o", str(output), "./cmd/moduforge"],
            cwd=backend_dir,
            check=True
        )
    
    if not output.exists():
        print("Backend build failed")
        return None
    
    size_mb = output.stat().st_size / (1024 * 1024)
    print(f"Backend binary: {output} ({size_mb:.1f} MB)")
    return output

def prepare_deployment(frontend_dist: Path, backend_binary: Path):
    """Create deployment package with only necessary files."""
    print("\n=== Preparing Deployment Package ===")
    
    pkg_dir = Path(tempfile.mkdtemp(prefix="moduforge_"))
    dist_dir = pkg_dir / "dist"
    dist_dir.mkdir()
    
    # Copy frontend files (excluding unnecessary ones)
    print("Copying frontend files...")
    for item in frontend_dist.rglob("*"):
        if item.is_file():
            # Check exclusion patterns
            skip = False
            for pattern in FRONTEND_EXCLUDE:
                if pattern.startswith("*."):
                    if item.name.endswith(pattern[1:]):
                        skip = True
                        break
                elif item.name == pattern:
                    skip = True
                    break
            
            if not skip:
                # Use forward slashes for Linux
                rel_path = str(item.relative_to(frontend_dist)).replace("\\", "/")
                dest = dist_dir / rel_path
                dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(item, dest)
                print(f"  + {rel_path}")
    
    # Copy backend binary
    backend_dest = pkg_dir / "server"
    shutil.copy2(backend_binary, backend_dest)
    backend_dest.chmod(0o755)
    
    # Create tar archive
    tar_path = Path(tempfile.mktemp(suffix=".tar.gz"))
    with tarfile.open(tar_path, "w:gz") as tar:
        tar.add(pkg_dir, arcname=".")
    
    size_mb = tar_path.stat().st_size / (1024 * 1024)
    print(f"\nPackage created: {tar_path} ({size_mb:.1f} MB)")
    
    # Cleanup temp dir
    shutil.rmtree(pkg_dir)
    
    return tar_path

def deploy_to_server(tar_path: Path):
    """Deploy package to server via Python paramiko."""
    print("\n=== Deploying to Server ===")
    
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    print(f"Connected to {HOST}")
    
    # Upload tar
    print("Uploading package...")
    sftp = ssh.open_sftp()
    remote_tar = f"/tmp/moduforge-deploy.tar.gz"
    sftp.put(str(tar_path), remote_tar)
    sftp.close()
    
    # Deploy commands
    commands = [
        # Stop container
        f"docker stop {CONTAINER} 2>/dev/null || true",
        
        # Extract to temp location
        "rm -rf /tmp/moduforge-deploy && mkdir -p /tmp/moduforge-deploy",
        f"tar -xzf {remote_tar} -C /tmp/moduforge-deploy",
        
        # Copy to container volume (preserving data)
        f"docker start {CONTAINER}",
        f"docker cp /tmp/moduforge-deploy/dist/. {CONTAINER}:/app/dist/",
        f"docker cp /tmp/moduforge-deploy/server {CONTAINER}:/app/server",
        
        # Fix permissions
        f"docker exec {CONTAINER} chmod +x /app/server",
        
        # Restart
        f"docker restart {CONTAINER}",
        "sleep 3",
        
        # Verify
        f"docker logs --tail 5 {CONTAINER}",
        "curl -s http://localhost:8086/health",
    ]
    
    for cmd in commands:
        print(f"  > {cmd}")
        stdin, stdout, stderr = ssh.exec_command(cmd)
        exit_status = stdout.channel.recv_exit_status()
        output = stdout.read().decode().strip()
        error = stderr.read().decode().strip()
        
        if output:
            print(f"    {output}")
        if error and exit_status != 0:
            print(f"    ERROR: {error}")
    
    # Final health check
    print("\n=== Final Health Check ===")
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"Health: {health}")
    
    ssh.close()
    
    return '"status":"ok"' in health

def main():
    print("=" * 60)
    print("ModuForge Full Deployment")
    print("=" * 60)
    
    # Build
    frontend_dist = build_frontend()
    if not frontend_dist:
        print("Frontend build failed!")
        return False
    
    backend_binary = build_backend()
    if not backend_binary:
        print("Backend build failed!")
        return False
    
    # Package
    tar_path = prepare_deployment(frontend_dist, backend_binary)
    
    # Deploy
    success = deploy_to_server(tar_path)
    
    # Cleanup
    tar_path.unlink(missing_ok=True)
    
    print("\n" + "=" * 60)
    if success:
        print("✅ Deployment successful!")
        print(f"Frontend: http://{HOST}:8086")
        print(f"API: http://{HOST}:8086/api/v1")
    else:
        print("❌ Deployment failed!")
    
    return success

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
