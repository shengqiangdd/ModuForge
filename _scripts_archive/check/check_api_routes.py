#!/usr/bin/env python3
"""Check API routes in the container"""

import sys
import paramiko

# Fix Windows GBK encoding
if sys.platform == "win32":
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216')

# Check if the binary has the routes compiled in
print("Checking if binary has routes compiled in...")
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge strings /app/moduforge-server | grep -i "md-prompts" | head -5')
output = stdout.read().decode()
if output:
    print(f"✓ Found 'md-prompts' in binary: {output.strip()}")
else:
    print("✗ 'md-prompts' not found in binary")

# Check if the prompts package is embedded
print("\nChecking if prompts package is embedded...")
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge strings /app/moduforge-server | grep -i "base.md" | head -5')
output = stdout.read().decode()
if output:
    print(f"✓ Found 'base.md' in binary: {output.strip()}")
else:
    print("✗ 'base.md' not found in binary")

# Check the last few lines of the container logs for any errors
print("\nChecking container logs for errors...")
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 10 2>&1 | grep -i "error\|panic\|fail"')
output = stdout.read().decode()
if output:
    print(f"Found errors: {output}")
else:
    print("✓ No errors found in recent logs")

# Check if the server is listening on the correct port
print("\nChecking if server is listening on port 8080...")
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge netstat -tlnp 2>/dev/null || ss -tlnp 2>/dev/null || echo "netstat/ss not available"')
output = stdout.read().decode()
print(output)

ssh.close()