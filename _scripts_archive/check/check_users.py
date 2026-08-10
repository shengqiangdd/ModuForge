# -*- coding: utf-8 -*-
"""Check users and fix password."""
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check if there's a way to query users
# The DB is SQLite, let's try to read it
print("=== Check DB ===")

# Try different passwords
for pwd in ["admin123", "password", "admin", "csq0216", "123456"]:
    login = run(
        'curl -s -X POST http://localhost:8086/api/v1/auth/login '
        '-H "Content-Type: application/json" '
        '-d \'{"username":"admin","password":"%s"}\'' % pwd
    )
    if "token" in login:
        print(f"Login OK with password: {pwd}")
        print("Token:", login[:200])
        break
    else:
        print(f"Failed with {pwd}: {login[:100]}")

# Try to check the DB directly
print("\n=== DB Users ===")
# Use sqlite3 if available, or strings
print(run("docker exec moduforge sh -c 'cat /data/moduforge.db | strings | grep -i admin | head -10'"))

# Check if there's a reset endpoint
print("\n=== API Endpoints ===")
print(run("curl -s http://localhost:8086/api/v1/auth/forgot-password -X POST -H 'Content-Type: application/json' -d '{\"email\":\"admin@moduforge.local\"}' 2>/dev/null"))

ssh.close()
print("\nDone")
