#!/usr/bin/env python3
"""Deploy security improvements - build inside container for Linux"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"
CONTAINER = "moduforge"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # Upload source files via container
    print("=== Step 1: Upload source files ===")
    
    # Read local files
    with open("ModuForge/backend/security.go", "r", encoding="utf-8") as f:
        security_go = f.read()
    with open("ModuForge/backend/main_security_inject.go", "r", encoding="utf-8") as f:
        main_inject = f.read()
    
    # Create files inside container
    for fname, content in [("security.go", security_go), ("main_security_inject.go", main_inject)]:
        # Escape content for shell
        escaped = content.replace("'", "'\\''")
        out, err = run(f"docker exec {CONTAINER} bash -c 'echo \\'{escaped}\\' > /tmp/{fname}'")
        if err and "error" in err.lower():
            print(f"  Warning uploading {fname}: {err[:100]}")
    
    print("=== Step 2: Verify files uploaded ===")
    out, err = run(f"docker exec {CONTAINER} ls -la /tmp/*.go")
    print(out)
    
    print("=== Step 3: Install Go in container ===")
    out, err = run(f"docker exec {CONTAINER} which go", timeout=10)
    if "go" not in out:
        print("Installing Go...")
        run(f"docker exec {CONTAINER} sh -c 'curl -fsSL https://go.dev/dl/go1.23.4.linux-amd64.tar.gz | tar -C /usr/local -xz'", timeout=120)
        run(f"docker exec {CONTAINER} sh -c 'export PATH=$PATH:/usr/local/go/bin && go version'")
    
    print("=== Step 4: Check project structure ===")
    out, err = run(f"docker exec {CONTAINER} ls /app/")
    print(f"/app contents: {out[:300]}")
    
    # Check if main.go uses security
    out, err = run(f"docker exec {CONTAINER} head -50 /app/main.go")
    print(f"\nmain.go head:\n{out[:500]}")
    
    client.close()

if __name__ == "__main__":
    main()
