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
    return out, err, exit_status

print("=== Step 1: Check container status ===")
out, _, _ = run("docker ps -a --filter name=moduforge --format '{{.Names}} {{.Status}}'")
print(f"Container: {out}")

# Wait for container to be healthy
print("\n=== Waiting for container to be healthy ===")
for i in range(10):
    out, _, _ = run("docker exec moduforge curl -s http://localhost:8080/health 2>/dev/null")
    if out:
        print(f"Health: {out}")
        break
    time.sleep(2)

print("\n=== Step 2: Replace zipper.go in overlay2 ===")
# Copy new zipper.go to the overlay2 directory
sftp = ssh.open_sftp()
sftp.put(r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go", 
         "/vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/service/zipper.go")
sftp.close()
print("Copied new zipper.go to overlay2")

# Verify the file was copied
out, _, _ = run("head -10 /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/service/zipper.go")
print(f"First lines: {out}")

print("\n=== Step 3: Recompile in container ===")
# Stop container
run("docker stop moduforge")
time.sleep(2)

# Start container
run("docker start moduforge")
time.sleep(5)

# Copy source to container
out, _, status = run("docker cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend moduforge:/tmp/build_src")
print(f"Copy source: {out}")

# Compile
out, err, status = run("docker exec moduforge sh -c 'cd /tmp/build_src && CGO_ENABLED=1 go build -o /tmp/server_new ./cmd/moduforge 2>&1'")
print(f"Compile: {out}")
if err:
    print(f"Compile errors: {err}")

# Check binary
out, _, _ = run("docker exec moduforge ls -la /tmp/server_new 2>/dev/null || echo 'Binary not found'")
print(f"Binary: {out}")

print("\n=== Step 4: Deploy binary ===")
# Stop container
run("docker stop moduforge")
time.sleep(2)

# Replace binary
out, _, status = run("docker exec moduforge mv /app/server /app/server_old 2>/dev/null")
print(f"Move old: {out}")

out, _, status = run("docker exec moduforge mv /tmp/server_new /app/server 2>/dev/null")
print(f"Move new: {out}")

out, _, status = run("docker exec moduforge chmod +x /app/server 2>/dev/null")
print(f"Chmod: {out}")

# Start container
run("docker start moduforge")
time.sleep(5)

out, _, _ = run("docker ps --filter name=moduforge --format '{{.Status}}'")
print(f"Status: {out}")

out, _, _ = run("docker exec moduforge curl -s http://localhost:8080/health")
print(f"Health: {out}")

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
        print(f"Has webroot wrapper: {'YES' if has_webroot else 'NO'}")
except Exception as e:
    print(f"Error: {e}")

ssh.close()
