#!/usr/bin/env python3
"""Deploy ModuForge - use correct NAS path."""
import paramiko
import sys
import time
import os

sys.stdout.reconfigure(encoding='utf-8')
sys.stderr.reconfigure(encoding='utf-8')

SERVER = '192.168.2.9'
USER = 'admin'
PASS = 'csq0216'
COMPOSE_DIR = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'

def run(ssh, cmd, timeout=120):
    print(f"  > {cmd[:150]}")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode()
    err = stderr.read().decode()
    if out.strip():
        print(f"  {out.strip()[:1000]}")
    if err.strip():
        print(f"  ERR: {err.strip()[:500]}")
    return out.strip(), err.strip()

def main():
    print("=" * 60)
    print("  ModuForge Deployment")
    print("=" * 60)

    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(SERVER, username=USER, password=PASS, timeout=15)
    print("[OK] Connected\n")

    # Verify location
    print("[1/5] Verifying location...")
    run(ssh, f'ls -la {COMPOSE_DIR}/docker-compose.yml')
    print()

    # Container status
    print("[2/5] Container status...")
    run(ssh, 'docker ps -a --filter name=moduforge --format "{{.Names}} {{.Status}}"')
    print()

    # Upload changed source files
    print("[3/5] Uploading source files...")
    sftp = ssh.open_sftp()

    local_backend = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend'
    local_frontend = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\frontend'

    uploaded = 0
    for root, dirs, files in os.walk(local_backend):
        for f in files:
            if f.endswith('.go') or f in ('go.mod', 'go.sum'):
                local_path = os.path.join(root, f)
                rel = os.path.relpath(local_path, local_backend).replace('\\', '/')
                remote = f'{COMPOSE_DIR}/backend/{rel}'
                try:
                    sftp.put(local_path, remote)
                    uploaded += 1
                except Exception as e:
                    print(f"  Skip: {rel} ({e})")

    for root, dirs, files in os.walk(local_frontend):
        for f in files:
            if f.endswith(('.svelte', '.ts', '.js')) and 'node_modules' not in root:
                local_path = os.path.join(root, f)
                rel = os.path.relpath(local_path, local_frontend).replace('\\', '/')
                remote = f'{COMPOSE_DIR}/frontend/{rel}'
                try:
                    sftp.put(local_path, remote)
                    uploaded += 1
                except Exception as e:
                    print(f"  Skip: {rel} ({e})")

    sftp.close()
    print(f"  Uploaded {uploaded} files")
    print()

    # Rebuild and restart
    print("[4/5] Rebuilding...")
    run(ssh, f'cd {COMPOSE_DIR} && docker compose down 2>&1', timeout=30)
    out, err = run(ssh, f'cd {COMPOSE_DIR} && docker compose build 2>&1', timeout=600)
    print()

    print("[5/5] Starting...")
    run(ssh, f'cd {COMPOSE_DIR} && docker compose up -d 2>&1', timeout=30)
    print()

    # Verify
    time.sleep(8)
    print("Verifying...")
    run(ssh, 'docker ps --filter name=moduforge --format "{{.Names}} {{.Status}}"')
    run(ssh, 'curl -s http://localhost:8086/health | head -c 200')

    ssh.close()
    print("\n[DONE] http://192.168.2.9:8086")

if __name__ == '__main__':
    main()
