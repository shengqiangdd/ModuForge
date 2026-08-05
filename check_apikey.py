#!/usr/bin/env python3
"""Check OpenCode Go API Key configuration"""
import paramiko
import json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=15)
    return stdout.read().decode('utf-8', errors='replace')

# Check environment variables
print("=== Environment Variables ===")
env = run("docker inspect moduforge-app-1 --format '{{range .Config.Env}}{{println .}}{{end}}'")
print(env)

# Check if OPENCODE_API_KEY is set
print("\n=== Check OPENCODE_API_KEY ===")
if 'OPENCODE_API_KEY' in env:
    print("OPENCODE_API_KEY is set")
else:
    print("OPENCODE_API_KEY is NOT set")

# Check container logs for API key errors
print("\n=== Recent Logs for API Key Errors ===")
logs = run("docker logs moduforge-app-1 --tail 20 2>&1 | grep -i 'api\\|key\\|auth\\|error'")
clean = logs.encode('ascii', errors='replace').decode()
print(clean if clean.strip() else "No API key errors found")

# Check .env file
print("\n=== .env File ===")
env_file = run("cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/.env 2>/dev/null || echo 'No .env file'")
print(env_file)

ssh.close()
