#!/usr/bin/env python3
"""Deploy ModuForge to remote server via SSH + Docker Compose.

Usage:
    python deploy_compose_ssh.py [--pull] [--rebuild] [--compose-dir PATH]
"""
import argparse
import sys
import time

import paramiko

# Defaults (override via CLI)
SERVER = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
REMOTE_DIR = "/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge"


def run(ssh: paramiko.SSHClient, cmd: str, timeout: int = 120) -> str:
    print(f"  > {cmd}")
    _, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode(errors="replace")
    err = stderr.read().decode(errors="replace")
    if out.strip():
        print(out.strip()[:2000])
    if err.strip():
        print(f"ERR: {err.strip()[:2000]}")
    return out.strip()


def main() -> None:
    parser = argparse.ArgumentParser(description="Deploy ModuForge via SSH + Docker Compose")
    parser.add_argument("--pull", action="store_true", help="Git pull before building")
    parser.add_argument("--rebuild", action="store_true", help="Force --no-cache rebuild")
    parser.add_argument("--compose-dir", default=REMOTE_DIR, help="Remote compose directory")
    args = parser.parse_args()

    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    print(f"Connecting to {USER}@{SERVER}...")
    ssh.connect(SERVER, port=22, username=USER, password=PASS, timeout=15)
    print("Connected.\n")

    remote = args.compose_dir

    # Verify compose file exists
    print("[1/6] Verifying docker-compose.yml...")
    run(ssh, f"ls -l {remote}/docker-compose.yml")

    # Git pull (optional)
    if args.pull:
        print("\n[2/6] Pulling latest code...")
        # Safe fetch + reset: avoids merge conflicts on remote with local tweaks
        run(ssh, f"cd {remote} && git fetch origin main", timeout=60)
        run(ssh, f"cd {remote} && git reset --hard origin/main", timeout=30)
        run(ssh, f"cd {remote} && git clean -fd", timeout=30)
    else:
        print("\n[2/6] Skipping git pull (use --pull to enable)")

    # Down
    print("\n[3/6] Stopping existing containers...")
    run(ssh, f"cd {remote} && docker compose down", timeout=60)

    # Build
    print("\n[4/6] Building image...")
    build_flag = " --no-cache" if args.rebuild else ""
    run(ssh, f"cd {remote} && docker compose build{build_flag}", timeout=600)

    # Up
    print("\n[5/6] Starting container...")
    run(ssh, f"cd {remote} && docker compose up -d", timeout=60)

    # Verify
    print("\n[6/6] Verifying...")
    time.sleep(5)
    run(ssh, "docker ps --filter name=moduforge --format 'table {{.Names}}\\t{{.Status}}\\t{{.Ports}}'")
    run(ssh, "curl -sf http://localhost:8086/health && echo '\\nHealth OK' || echo '\\nHealth check failed'")

    ssh.close()
    print("\nDone. Access: http://192.168.2.9:8086")


if __name__ == "__main__":
    main()
