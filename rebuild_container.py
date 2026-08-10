#!/usr/bin/env python3
"""
Rebuild container dist from scratch - avoid Windows path issues entirely.
Uses tar on server side to sync files properly.
"""
import paramiko
import time
import tarfile
import tempfile
from pathlib import Path

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"
PROJECT_DIR = Path(__file__).parent

def rebuild():
    print("=" * 60)
    print("Rebuilding Container Dist")
    print("=" * 60)
    
    # Step 1: Create clean tar locally
    print("\n1. Creating clean tar archive...")
    frontend_dist = PROJECT_DIR / "frontend" / "dist"
    
    tar_path = Path(tempfile.mktemp(suffix=".tar"))
    with tarfile.open(tar_path, "w") as tar:
        for item in frontend_dist.rglob("*"):
            if item.is_file():
                # Skip unnecessary files
                if item.name in ('manifest.json', 'sw.js', 'icon-192.svg', 
                                 'icon-512.svg', 'MaterialSymbolsOutlined.ttf'):
                    continue
                if item.name.endswith('.map'):
                    continue
                
                # Use forward slashes
                arcname = "dist/" + str(item.relative_to(frontend_dist)).replace("\\", "/")
                tar.add(item, arcname=arcname)
    
    print(f"  Archive created: {tar_path.name}")
    
    # Step 2: Upload and extract on server
    print("\n2. Uploading to server...")
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
    
    sftp = ssh.open_sftp()
    remote_tar = "/tmp/moduforge-dist.tar"
    sftp.put(str(tar_path), remote_tar)
    sftp.close()
    
    # Step 3: Stop container and replace dist
    print("\n3. Replacing dist in container...")
    commands = [
        f"docker stop {CONTAINER}",
        "sleep 2",
        f"docker rm {CONTAINER}",
        # Recreate container
        f"docker run -d --name {CONTAINER} -p 8086:8086 moduforge:latest",
        "sleep 3",
        # Extract dist
        f"docker cp {remote_tar} {CONTAINER}:/tmp/dist.tar",
        f"docker exec {CONTAINER} rm -rf /app/dist",
        f"docker exec {CONTAINER} mkdir -p /app/dist",
        f"docker exec {CONTAINER} tar -xf /tmp/dist.tar -C /app/",
        f"docker exec {CONTAINER} rm /tmp/dist.tar",
    ]
    
    for cmd in commands:
        print(f"  > {cmd}")
        stdin, stdout, stderr = ssh.exec_command(cmd)
        stdout.channel.recv_exit_status()
    
    # Step 4: Verify
    print("\n4. Verifying...")
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} find /app/dist -type f | sort")
    print(stdout.read().decode())
    
    stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} du -sh /app/dist")
    print(f"Size: {stdout.read().decode().strip()}")
    
    stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health")
    health = stdout.read().decode()
    print(f"Health: {health}")
    
    ssh.close()
    tar_path.unlink(missing_ok=True)
    
    return '"status":"ok"' in health

if __name__ == '__main__':
    import sys
    success = rebuild()
    sys.exit(0 if success else 1)
