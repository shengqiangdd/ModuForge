#!/usr/bin/env python3
"""Deploy ModuForge via Docker Compose.

Usage:
    python deploy_compose.py

Connects to the remote server via SSH, pulls latest code from git,
rebuilds the Docker container, waits for healthy status, and verifies
the deployment. Credentials are read from environment variables or
configurable constants.
"""
import paramiko
import time

SERVER = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
MODUFORGE_PATH = "/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge"

def run_cmd(ssh, cmd, timeout=120):
    print(f"\n>>> {cmd}")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    clean_out = out.encode('ascii', errors='replace').decode('ascii')
    clean_err = err.encode('ascii', errors='replace').decode('ascii')
    if clean_out.strip(): print(clean_out)
    if clean_err.strip(): print(clean_err)
    return out, err

def main():
    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    
    try:
        print(f"Connecting to {USER}@{SERVER}...")
        ssh.connect(SERVER, username=USER, password=PASSWORD, timeout=10)
        print("Connected!")
        
        # 1. Git pull
        print("\n=== Step 1: Git Pull ===")
        run_cmd(ssh, f"cd {MODUFORGE_PATH} && git -c safe.directory='*' pull origin main")
        
        # 2. Rebuild and restart with docker compose
        print("\n=== Step 2: Docker Compose Rebuild ===")
        run_cmd(ssh, f"cd {MODUFORGE_PATH} && docker compose up -d --build 2>&1 | tail -10")
        
        # 3. Wait for health check
        print("\n=== Step 3: Waiting for healthy status ===")
        for i in range(6):
            time.sleep(5)
            status = run_cmd(ssh, "docker ps --filter name=moduforge-app --format '{{.Status}}'")
            if 'healthy' in status[0]:
                print("Container is healthy!")
                break
        
        # 4. Final verification
        print("\n=== Step 4: Verify ===")
        run_cmd(ssh, "docker ps --filter name=moduforge-app --format '{{.Names}} {{.Status}} {{.Ports}}'")
        run_cmd(ssh, "curl -s http://localhost:8086/health")
        
    except Exception as e:
        print(f"Error: {e}")
    finally:
        ssh.close()
        print("\nDone!")

if __name__ == "__main__":
    main()
