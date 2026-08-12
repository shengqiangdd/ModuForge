#!/usr/bin/env python3
"""Deploy ModuForge using paramiko."""
import paramiko
import sys
import io
import time

# Fix Windows GBK encoding
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
REMOTE_DIR = "/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge"

def ssh_connect():
    """Connect via SSH."""
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=15)
    print(f"Connected to {USER}@{HOST}")
    return client

def run_cmd(client, cmd, timeout=120):
    """Run command and stream output."""
    print(f"\n>>> {cmd}")
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout)
    output = stdout.read().decode('utf-8', errors='replace')
    errors = stderr.read().decode('utf-8', errors='replace')
    if output:
        print(output)
    if errors:
        print(errors)
    return stdout.channel.recv_exit_status()

def main():
    print("=== ModuForge Deploy ===")
    print(f"Server: {HOST}")
    print(f"Directory: {REMOTE_DIR}")
    
    try:
        client = ssh_connect()
    except Exception as e:
        print(f"Connection failed: {e}")
        sys.exit(1)
    
    try:
        # Pull latest code (skip if network issue)
        print("\n[1/4] Pulling latest code...")
        rc = run_cmd(client, f"cd {REMOTE_DIR} && git pull", timeout=60)
        if rc != 0:
            print("Git pull failed (network issue), continuing with existing code...")
        
        # Stop containers
        print("\n[2/4] Stopping containers...")
        run_cmd(client, f"cd {REMOTE_DIR} && docker compose down", timeout=60)
        
        # Rebuild backend only
        print("\n[3/4] Rebuilding backend...")
        rc = run_cmd(client, 
            f"cd {REMOTE_DIR} && docker compose build --no-cache",
            timeout=300)
        if rc != 0:
            print("Build failed!")
            sys.exit(1)
        
        # Start containers
        print("\n[4/4] Starting containers...")
        run_cmd(client, f"cd {REMOTE_DIR} && docker compose up -d", timeout=60)
        
        # Health check
        print("\nWaiting for health check...")
        time.sleep(10)
        run_cmd(client, "curl -s http://localhost:8086/health", timeout=15)
        
        print("\n=== Deploy complete! ===")
    finally:
        client.close()

if __name__ == "__main__":
    main()
