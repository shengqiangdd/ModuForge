#!/usr/bin/env python3
import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return out, err

print("=== Check binary sizes ===")
out, err = run("docker exec moduforge ls -la /server /app/server /tmp/server_new 2>&1")
print(out)
if err:
    print(f"Errors: {err}")

print("\n=== Check if webroot logic is in binary ===")
out, err = run("docker exec moduforge strings /app/server | grep -i webroot | head -5")
print(f"webroot strings: {out}")

out, err = run("docker exec moduforge strings /tmp/server_new | grep -i webroot | head -5")
print(f"webroot in new binary: {out}")

print("\n=== Check if isFrontendFile function exists ===")
out, err = run("docker exec moduforge strings /tmp/server_new | grep -i frontend | head -5")
print(f"frontend strings: {out}")

print("\n=== Stop container and replace binary properly ===")
run("docker stop moduforge")
time.sleep(2)

# Replace binary
out, err = run("docker exec moduforge mv /app/server /app/server_old 2>&1")
print(f"Move old: {out} {err}")

out, err = run("docker exec moduforge mv /tmp/server_new /app/server 2>&1")
print(f"Move new: {out} {err}")

out, err = run("docker exec moduforge chmod +x /app/server 2>&1")
print(f"Chmod: {out} {err}")

out, err = run("docker exec moduforge ls -la /app/server 2>&1")
print(f"New binary: {out}")

print("\n=== Start container ===")
run("docker start moduforge")
time.sleep(5)

out, err = run("docker exec moduforge curl -s http://localhost:8080/health")
print(f"Health: {out}")

print("\n=== Test export ===")
login_resp, _ = run('''docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
import json
token = json.loads(login_resp).get('token', '')

projects_resp, _ = run(f'docker exec moduforge curl -s http://localhost:8080/api/v1/projects -H "Authorization: Bearer {token}"')
projects = json.loads(projects_resp)

if projects:
    pid = projects[0]['id']
    status, _ = run(f'docker exec moduforge curl -s -o /tmp/test_export.zip -w "%{{http_code}}" http://localhost:8080/api/v1/projects/{pid}/export-zip -X POST -H "Authorization: Bearer {token}"')
    print(f"Export status: {status}")
    
    run("docker cp moduforge:/tmp/test_export.zip /tmp/test_export.zip")
    
    print("\n=== ZIP Contents ===")
    zip_list, _ = run('python3 -c "import zipfile; z=zipfile.ZipFile(\'/tmp/test_export.zip\'); [print(n) for n in sorted(z.namelist())]"')
    print(zip_list)
    
    print("\n=== Analysis ===")
    lines = [n for n in zip_list.split('\n') if n.strip()]
    has_webroot = any('webroot/' in n for n in lines)
    print(f"Has webroot wrapper: {'YES' if has_webroot else 'NO'}")

ssh.close()
