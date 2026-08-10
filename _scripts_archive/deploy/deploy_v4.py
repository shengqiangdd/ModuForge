#!/usr/bin/env python3
"""Deploy by replacing BOTH /app/moduforge-server AND /server"""
import paramiko, time, hashlib

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
LOCAL_BIN = r"C:\Users\22875\.qwenpaw\workspaces\default\moduforge\backend\moduforge.exe"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)
print("Connected")

# Upload new binary
print("\n--- Upload ---")
sftp = ssh.open_sftp()
sftp.put(LOCAL_BIN, "/tmp/moduforge-server-new")
sftp.close()
print("Uploaded")

# Replace BOTH binaries
print("\n--- Replace binaries ---")
cmds = [
    f"docker cp /tmp/moduforge-server-new {CONTAINER}:/server",
    f"docker exec {CONTAINER} chmod 755 /server",
    f"docker exec {CONTAINER} cp /tmp/moduforge-server-new /app/moduforge-server",
    f"docker exec {CONTAINER} chmod 755 /app/moduforge-server",
]
for cmd in cmds:
    print(f"  $ {cmd}")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    if err:
        print(f"    ERR: {err}")
    else:
        print(f"    OK")

# Restart container
print("\n--- Restart ---")
stdin, stdout, stderr = ssh.exec_command(f"docker restart {CONTAINER}", timeout=30)
print(stdout.read().decode().strip())
time.sleep(5)

# Health check
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
print(f"\nHealth: {stdout.read().decode().strip()}")

# Verify running binary
print("\n--- Verify ---")
stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} ls -li /server /app/moduforge-server /proc/1/exe", timeout=10)
print(stdout.read().decode().strip())

# Check strings
stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} strings /server | grep -c 'WHERE name='", timeout=15)
print(f"\nRunning /server - 'WHERE name=' count: {stdout.read().decode().strip()}")

stdin, stdout, stderr = ssh.exec_command(f"docker exec {CONTAINER} strings /server | grep -c 'not in DB'", timeout=15)
print(f"Running /server - 'not in DB' count: {stdout.read().decode().strip()}")

ssh.close()
print("\nDone")
