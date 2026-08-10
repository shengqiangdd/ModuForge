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
    exit_status = stdout.channel.recv_exit_status()
    if err and exit_status != 0:
        print(f"STDERR: {err[:300]}")
    return out, exit_status

print("=== Step 1: Stop container ===")
run("docker stop moduforge")
time.sleep(2)

print("\n=== Step 2: Check stopped container ===")
out, _ = run("docker ps -a --filter name=moduforge --format '{{.Names}} {{.Status}}'")
print(out)

print("\n=== Step 3: Copy new binary to container ===")
# Use docker cp to copy the new binary
out, status = run("docker cp /tmp/build_src/internal /tmp/test_check 2>&1")
print(f"Check source: {out}")

# The new binary is at /tmp/server_new inside the container
# Let's use docker cp from host to container
out, status = run("docker cp moduforge:/tmp/server_new /tmp/server_on_host 2>&1")
print(f"Copy to host: {out}")

if status == 0:
    out, status = run("docker cp /tmp/server_on_host moduforge:/app/server 2>&1")
    print(f"Copy back: {out}")
    
    out, status = run("docker exec moduforge chmod +x /app/server 2>&1")
    print(f"Chmod: {out}")
    
    out, _ = run("docker exec moduforge ls -la /app/server 2>&1")
    print(f"Binary: {out}")
else:
    print("Binary not found in container, using alternative method")
    # Copy directly from overlay2
    out, status = run("cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal /tmp/test_src -r 2>&1")
    print(f"Copy source: {out}")

print("\n=== Step 4: Start container ===")
run("docker start moduforge")
time.sleep(5)

out, _ = run("docker ps --filter name=moduforge --format '{{.Status}}'")
print(f"Status: {out}")

out, _ = run("docker exec moduforge curl -s http://localhost:8080/health")
print(f"Health: {out}")

print("\n=== Step 5: Test export ===")
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
