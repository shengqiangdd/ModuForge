#!/usr/bin/env python3
"""Check ModuForge and Agent status"""
import paramiko
import json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=15)
    return stdout.read().decode('utf-8', errors='replace')

# Check container
print("=== Container Status ===")
print(run("docker ps --filter name=moduforge-app --format '{{.Names}} {{.Status}}'").strip())

# Check health
print("\n=== Health Check ===")
print(run("curl -s http://localhost:8086/health").strip())

# Check Agent skills
print("\n=== Agent Skills ===")
login_resp = run("curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"csq\",\"password\":\"csq0216\"}'")
token = json.loads(login_resp).get('token', '')
skills_resp = run(f"curl -s http://localhost:8086/api/v1/agent/skills -H 'Authorization: Bearer {token}'")
try:
    skills = json.loads(skills_resp)
    if 'skills' in skills:
        print(f"Total skills: {len(skills['skills'])}")
except:
    print("Skills endpoint requires auth or not available")

# Check recent errors
print("\n=== Recent Errors ===")
logs = run("docker logs moduforge-app-1 --tail 10 2>&1 | grep -i error")
clean = logs.encode('ascii', errors='replace').decode()
print(clean.strip() if clean.strip() else "No recent errors")

# Check project files count
print("\n=== AndroBoost-SmartTune Project ===")
files_resp = run(f"curl -s http://localhost:8086/api/v1/projects/1785249992652501794-1864/files -H 'Authorization: Bearer {token}'")
try:
    files = json.loads(files_resp)
    if isinstance(files, list):
        print(f"Total files: {len(files)}")
        # Show Rust files
        rust_files = [f['path'] for f in files if f.get('path', '').endswith('.rs')]
        print(f"Rust files: {len(rust_files)}")
        for rf in rust_files:
            print(f"  - {rf}")
except:
    print("Failed to get project files")

ssh.close()
