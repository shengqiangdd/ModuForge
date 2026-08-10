#!/usr/bin/env python3
"""Deploy using docker commit approach"""
import paramiko, time

HOST = "192.168.2.9"
USER = "admin"
PASS = "csq0216"
LOCAL_BIN = r"C:\Users\22875\.qwenpaw\workspaces\default\moduforge\backend\moduforge.exe"
CONTAINER = "moduforge"

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASS, timeout=10)
print("Connected")

# Step 1: Stop container
print("\n--- Stop container ---")
stdin, stdout, stderr = ssh.exec_command(f"docker stop {CONTAINER}", timeout=30)
print(stdout.read().decode().strip())

# Step 2: Check if binary exists in stopped container
print("\n--- Check binary before ---")
stdin, stdout, stderr = ssh.exec_command(f"docker run --rm -v /var/lib/docker/overlay2:/overlay alpine find /overlay -name 'moduforge-server' -path '*/merged/app/*' 2>/dev/null | head -3", timeout=30)
print(stdout.read().decode().strip())

# Step 3: Upload to /tmp
print("\n--- Upload ---")
sftp = ssh.open_sftp()
sftp.put(LOCAL_BIN, "/tmp/moduforge-server-new")
sftp.close()
print("Uploaded")

# Step 4: Start container briefly to ensure filesystem is accessible, then stop
print("\n--- Start briefly ---")
stdin, stdout, stderr = ssh.exec_command(f"docker start {CONTAINER}", timeout=10)
print(stdout.read().decode().strip())
time.sleep(2)

# Step 5: Use docker exec to copy - the container is running now
print("\n--- Copy via running container ---")
stdin, stdout, stderr = ssh.exec_command(
    f"docker exec {CONTAINER} sh -c 'cat /app/moduforge-server | head -c 10 | od -A x -t x1z | head -1'",
    timeout=10
)
old_hex = stdout.read().decode().strip()
print(f"Old binary header: {old_hex}")

# Now overwrite via dd through stdin
print("\n--- Overwrite binary via cat ---")
# Upload new binary to container's filesystem
cmds = [
    # Write new binary
    f"docker exec -i {CONTAINER} sh -c 'cat > /app/moduforge-server' < {LOCAL_BIN}",
    # Make executable
    f"docker exec {CONTAINER} chmod 755 /app/moduforge-server",
    # Verify new binary header
    f"docker exec {CONTAINER} sh -c 'head -c 10 /app/moduforge-server | od -A x -t x1z | head -1'",
]
for cmd in cmds:
    print(f"  $ {cmd[:80]}...")
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    if out:
        print(f"    {out[:200]}")
    if err:
        print(f"    ERR: {err[:200]}")

# Step 6: Restart
print("\n--- Restart container ---")
stdin, stdout, stderr = ssh.exec_command(f"docker restart {CONTAINER}", timeout=30)
print(stdout.read().decode().strip())

time.sleep(5)

# Health check
stdin, stdout, stderr = ssh.exec_command("curl -s http://localhost:8086/health", timeout=10)
print(f"\nHealth: {stdout.read().decode().strip()}")

# Verify binary content
print("\n--- Verify binary ---")
stdin, stdout, stderr = ssh.exec_command(
    f"docker exec {CONTAINER} strings /app/moduforge-server | grep -c 'WHERE name='",
    timeout=15
)
count = stdout.read().decode().strip()
print(f"'WHERE name=' count: {count}")

stdin, stdout, stderr = ssh.exec_command(
    f"docker exec {CONTAINER} strings /app/moduforge-server | grep -c 'not in DB'",
    timeout=15
)
count2 = stdout.read().decode().strip()
print(f"'not in DB' count: {count2}")

ssh.close()
print("\nDone")
