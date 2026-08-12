#!/usr/bin/env python3
"""
Deploy ModuForge fixes: Toast + Layout + arm64 cross-compile
"""
import sys, os, io, time
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
REPO = "/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge"

# Files to upload (local_path, remote_path)
UPLOADS = [
    # Core fix: arm64 cross-compile CC environment variable
    ("backend/internal/builder/rust_compile.go", "backend/internal/builder/rust_compile.go"),
    # Frontend fixes
    ("frontend/src/lib/components/ui/Toast.svelte", "frontend/src/lib/components/ui/Toast.svelte"),
    ("frontend/src/lib/components/editor/BuildWorkspace.svelte", "frontend/src/lib/components/editor/BuildWorkspace.svelte"),
    ("frontend/src/lib/components/editor/BuildOutput.svelte", "frontend/src/lib/components/editor/BuildOutput.svelte"),
]

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    
    print(f"Connecting to {USER}@{HOST}...")
    client.connect(HOST, username=USER, password=PASSWORD, timeout=15)
    print("Connected!")
    
    sftp = client.open_sftp()
    
    # Upload files
    print("\n--- Uploading files ---")
    for local_rel, remote_rel in UPLOADS:
        local = os.path.join(os.path.dirname(__file__), local_rel)
        remote = f"{REPO}/{remote_rel}"
        if not os.path.exists(local):
            print(f"  SKIP (not found): {local_rel}")
            continue
        sftp.put(local, remote)
        print(f"  OK: {remote_rel}")
    sftp.close()
    
    # Rebuild Docker image (only rebuild Go backend layer)
    print("\n--- Rebuilding Docker image ---")
    cmd = f"cd {REPO} && docker compose down && docker compose build moduforge 2>&1"
    stdin, stdout, stderr = client.exec_command(cmd, timeout=300)
    exit_code = stdout.channel.recv_exit_status()
    output = stdout.read().decode('utf-8', errors='replace')
    
    if exit_code != 0:
        print(f"Build FAILED (exit {exit_code})")
        # Show last 30 lines
        for line in output.split('\n')[-30:]:
            if line.strip():
                print(f"  {line}")
        client.close()
        sys.exit(1)
    
    # Check for success
    if "error" in output.lower() and "successfully" not in output.lower():
        print("Build may have errors:")
        for line in output.split('\n')[-20:]:
            if line.strip():
                print(f"  {line}")
    else:
        print("Build OK")
    
    # Restart container
    print("\n--- Starting container ---")
    cmd = f"cd {REPO} && docker compose up -d 2>&1"
    stdin, stdout, stderr = client.exec_command(cmd, timeout=60)
    exit_code = stdout.channel.recv_exit_status()
    output = stdout.read().decode('utf-8', errors='replace')
    print(output.strip() if output.strip() else "Started")
    
    # Wait for health
    print("\n--- Waiting for health check ---")
    for i in range(10):
        time.sleep(2)
        stdin, stdout, stderr = client.exec_command(f"curl -s http://localhost:8086/health", timeout=5)
        health = stdout.read().decode('utf-8', errors='replace').strip()
        if health and "ok" in health.lower():
            print(f"  Healthy after {(i+1)*2}s")
            break
    else:
        print("  Warning: health check timeout, checking container...")
        stdin, stdout, stderr = client.exec_command(f"docker ps --filter name=moduforge --format '{{{{.Status}}}}'", timeout=5)
        print(f"  {stdout.read().decode().strip()}")
    
    client.close()
    print("\nDone!")

if __name__ == "__main__":
    main()
