#!/usr/bin/env python3
import paramiko
import time
import sys

sys.stdout.reconfigure(encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace').strip()
    err = stderr.read().decode('utf-8', errors='replace').strip()
    exit_status = stdout.channel.recv_exit_status()
    return out, err, exit_status

print("=== Step 1: Stop container ===")
run("docker stop moduforge")
time.sleep(3)

print("\n=== Step 2: Copy new binary to replace /server ===")
# Copy the new binary from /app/server to a temp location on host
out, err, status = run("docker cp moduforge:/app/server /tmp/new_server")
print(f"Copy to host: {out} {err}")

if status == 0:
    # Now copy it back to /server
    out, err, status = run("docker cp /tmp/new_server moduforge:/server")
    print(f"Copy to /server: {out} {err}")
    
    # Verify
    out, err, status = run("docker exec moduforge ls -la /server /app/server 2>&1")
    print(f"Verify: {out}")
else:
    print("Failed to copy binary")

print("\n=== Step 3: Start container ===")
run("docker start moduforge")
time.sleep(5)

out, _, _ = run("docker ps --filter name=moduforge --format '{{.Status}}'")
print(f"Status: {out}")

out, _, _ = run("docker exec moduforge curl -s http://localhost:8080/health")
print(f"Health: {out}")

print("\n=== Step 4: Verify running binary ===")
out, _, _ = run("docker exec moduforge strings /server | grep -c webroot")
print(f"webroot count in /server: {out}")

out, _, _ = run("docker exec moduforge strings /server | grep -i 'isFrontendFile' | head -3")
print(f"isFrontendFile: {out}")

print("\n=== Step 5: Test export ===")
login_resp, _, _ = run('''docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
import json
try:
    token = json.loads(login_resp).get('token', '')
    
    projects_resp, _, _ = run(f'docker exec moduforge curl -s http://localhost:8080/api/v1/projects -H "Authorization: Bearer {token}"')
    projects = json.loads(projects_resp)
    
    if projects:
        pid = projects[0]['id']
        status, _, _ = run(f'docker exec moduforge curl -s -o /tmp/test_export.zip -w "%{{http_code}}" http://localhost:8080/api/v1/projects/{pid}/export-zip -X POST -H "Authorization: Bearer {token}"')
        print(f"Export status: {status}")
        
        run("docker cp moduforge:/tmp/test_export.zip /tmp/test_export.zip")
        
        print("\n=== ZIP Contents ===")
        zip_list, _, _ = run('python3 -c "import zipfile; z=zipfile.ZipFile(\'/tmp/test_export.zip\'); [print(n) for n in sorted(z.namelist())]"')
        print(zip_list)
        
        print("\n=== Analysis ===")
        lines = [n for n in zip_list.split('\n') if n.strip()]
        has_webroot = any('webroot/' in n for n in lines)
        has_tmp = any(n.startswith('tmp/') for n in lines)
        has_design = any('DESIGN_DOC' in n for n in lines)
        has_backend = any('backend/' in n for n in lines)
        
        print(f"Has webroot wrapper: {'YES' if has_webroot else 'NO'}")
        print(f"Excludes tmp/: {'YES' if not has_tmp else 'NO'}")
        print(f"Excludes DESIGN_DOC.md: {'YES' if not has_design else 'NO'}")
        print(f"Excludes backend/: {'YES' if not has_backend else 'NO'}")
except Exception as e:
    print(f"Error: {e}")

ssh.close()
