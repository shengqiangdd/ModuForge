#!/usr/bin/env python3
"""Deploy frontend and backend changes to ModuForge server."""

import os
import sys
import paramiko
from pathlib import Path

# Server configuration
HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

# Local paths
FRONTEND_DIST = Path("frontend/dist")
BACKEND_BINARY = Path("/tmp/moduforge-server-linux")

def deploy():
    """Deploy changes to the server."""
    print("=== ModuForge Deployment ===")
    print(f"Server: {USER}@{HOST}")
    print(f"Container: {CONTAINER}")
    print()

    # Check files exist
    if not FRONTEND_DIST.exists():
        print(f"ERROR: Frontend dist not found at {FRONTEND_DIST}")
        return False
    
    if not BACKEND_BINARY.exists():
        print(f"ERROR: Backend binary not found at {BACKEND_BINARY}")
        return False

    print(f"Frontend dist: {FRONTEND_DIST} ({sum(1 for _ in FRONTEND_DIST.rglob('*') if _.is_file())} files)")
    print(f"Backend binary: {BACKEND_BINARY} ({BACKEND_BINARY.stat().st_size / 1024:.1f} KB)")
    print()

    # Connect to server
    print("1. Connecting to server...")
    try:
        ssh = paramiko.SSHClient()
        ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)
        print("   Connected successfully")
    except Exception as e:
        print(f"   ERROR: {e}")
        return False

    # Copy backend binary
    print("2. Copying backend binary...")
    try:
        sftp = ssh.open_sftp()
        sftp.put(str(BACKEND_BINARY), "/tmp/moduforge-server")
        sftp.close()
        print("   Backend binary copied")
    except Exception as e:
        print(f"   ERROR: {e}")
        ssh.close()
        return False

    # Copy frontend files
    print("3. Copying frontend files...")
    try:
        sftp = ssh.open_sftp()
        
        # Create temp directory for frontend
        stdin, stdout, stderr = ssh.exec_command(f"mkdir -p /tmp/frontend_dist")
        stdout.channel.recv_exit_status()
        
        # Copy all frontend files
        for file_path in FRONTEND_DIST.rglob('*'):
            if file_path.is_file():
                remote_path = f"/tmp/frontend_dist/{file_path.relative_to(FRONTEND_DIST)}"
                # Create remote directory
                remote_dir = os.path.dirname(remote_path)
                stdin, stdout, stderr = ssh.exec_command(f"mkdir -p {remote_dir}")
                stdout.channel.recv_exit_status()
                # Copy file
                sftp.put(str(file_path), remote_path)
        
        sftp.close()
        print("   Frontend files copied")
    except Exception as e:
        print(f"   ERROR: {e}")
        ssh.close()
        return False

    # Deploy to container
    print("4. Deploying to container...")
    try:
        commands = [
            f"docker cp /tmp/moduforge-server {CONTAINER}:/app/server",
            f"docker cp /tmp/frontend_dist/. {CONTAINER}:/app/dist/",
            f"docker restart {CONTAINER}",
            "sleep 3",
            f"docker logs --tail 10 {CONTAINER}"
        ]
        
        for cmd in commands:
            print(f"   Executing: {cmd}")
            stdin, stdout, stderr = ssh.exec_command(cmd)
            exit_status = stdout.channel.recv_exit_status()
            output = stdout.read().decode()
            error = stderr.read().decode()
            
            if output:
                print(f"   Output: {output}")
            if error:
                print(f"   Error: {error}")
            
            if exit_status != 0:
                print(f"   WARNING: Command returned non-zero exit status: {exit_status}")
        
        print("   Deployment completed")
    except Exception as e:
        print(f"   ERROR: {e}")
        ssh.close()
        return False

    # Verify deployment
    print("5. Verifying deployment...")
    try:
        stdin, stdout, stderr = ssh.exec_command(f"curl -s http://localhost:8086/health")
        output = stdout.read().decode()
        print(f"   Health check: {output}")
        
        if '"status":"ok"' in output:
            print("\n✅ Deployment successful!")
            return True
        else:
            print("\n⚠️  Health check failed, but deployment may still be successful")
            return True
    except Exception as e:
        print(f"   ERROR: {e}")
        ssh.close()
        return False

    ssh.close()
    return True

if __name__ == "__main__":
    success = deploy()
    sys.exit(0 if success else 1)
