#!/usr/bin/env python3
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    if err:
        print(f"STDERR: {err[:200]}")
    return out

# Find zipper.go
print("=== Finding zipper.go ===")
print(run("find / -name 'zipper.go' 2>/dev/null | head -10"))

# Check overlay2 for source
print("\n=== Overlay2 Source ===")
print(run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/service/ 2>/dev/null || echo 'Not found'"))

# Check if we have the modified version
print("\n=== Check if webroot logic exists ===")
print(run("grep -l 'webroot' /vol1/docker/overlay2/*/diff/backend/internal/service/zipper.go 2>/dev/null || echo 'No webroot found in any zipper.go'"))

# Check the actual running binary
print("\n=== Check running server ===")
print(run("docker exec moduforge ls -la /server /app/server 2>/dev/null"))

ssh.close()
