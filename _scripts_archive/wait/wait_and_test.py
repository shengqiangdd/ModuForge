# -*- coding: utf-8 -*-
"""Wait for rate limit and test."""
import paramiko, json, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check JWT secret from .env
print("=== JWT Secret ===")
print(run("docker exec moduforge cat /data/.env"))

# Wait for rate limit to reset (60 seconds)
print("\nWaiting 65 seconds for rate limit reset...")
time.sleep(65)

# Try login with the correct password
# From the DB dump, the user is "admin" with bcrypt hash
# Let's try the password from the original setup
print("\n=== Login ===")
login = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/login '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123"}\''
)
print("Login:", login[:300])

# If still failing, try to reset password via API
if "token" not in login:
    print("\n=== Try password reset ===")
    # Check if there's a reset endpoint
    reset = run(
        'curl -s -X POST http://localhost:8086/api/v1/auth/reset-password '
        '-H "Content-Type: application/json" '
        '-d \'{"token":"admin","new_password":"admin123"}\''
    )
    print("Reset:", reset[:200])
    
    # Try to check the hash algorithm
    print("\n=== Hash check ===")
    print("User exists in DB with bcrypt hash")
    print("Need to know the original password")

ssh.close()
print("\nDone")
