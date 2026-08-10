#!/usr/bin/env python3
"""Debug compilation - check source and compile output."""

import paramiko

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd, desc=""):
    print(f"=== {desc} ===")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=300)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    if out: print(out)
    if err: print(f"STDERR: {err}")
    print()
    return out

# Check if the source has our fix
print("Checking zipper.go in source...")
src_dir = "/vol1/docker/overlay2/8695b9eb7328cac868917ecb69c5d43b9bb86d3696e6f2b2947308f51c1b7233/diff/home/moduforge/backend"
run(f"head -80 {src_dir}/internal/service/zipper.go 2>/dev/null || echo 'File not found'")

# Try compiling with verbose output
print("\nCompiling with full output...")
run(f"""docker run --rm --entrypoint sh \
  -v {src_dir}:/src:ro \
  -v /tmp:/output \
  moduforge:latest -c "apk add --no-cache gcc musl-dev 2>/dev/null; echo 'gcc installed'; find / -name gcc 2>/dev/null; cd /src && CGO_ENABLED=1 go build -v -o /output/server_cgo ./cmd/moduforge 2>&1" """, "Verbose compile")

# Check output
run("ls -la /tmp/server_cgo 2>/dev/null || echo 'Not found'")

ssh.close()
print("\nDone!")
