#!/usr/bin/env python3
import paramiko
import os

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, check=True):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    exit_status = stdout.channel.recv_exit_status()
    if err:
        print(f"STDERR: {err[:500]}")
    if check and exit_status != 0:
        print(f"WARNING: Exit status {exit_status}")
    return out

print("=== Step 1: Copy zipper.go to server ===")
sftp = ssh.open_sftp()
sftp.put(r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go", "/tmp/zipper_new.go")
sftp.close()
print("Copied to server")

print("\n=== Step 2: Stop container ===")
run("docker stop moduforge")

print("\n=== Step 3: Copy source code ===")
run("rm -rf /tmp/moduforge_build", check=False)
run("mkdir -p /tmp/moduforge_build")
run("docker cp moduforge:/src /tmp/moduforge_build/src", check=False)

# Also copy from overlay2
run("cp -r /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/* /tmp/moduforge_build/src/", check=False)

print("\n=== Step 4: Replace zipper.go ===")
run("cp /tmp/zipper_new.go /tmp/moduforge_build/src/internal/service/zipper.go")
run("head -20 /tmp/moduforge_build/src/internal/service/zipper.go")

print("\n=== Step 5: Start container ===")
run("docker start moduforge")
run("sleep 3")
run("docker ps --filter name=moduforge --format '{{.Status}}'")

print("\n=== Step 6: Copy source to container ===")
run("docker cp /tmp/moduforge_build/src moduforge:/src_new")

print("\n=== Step 7: Compile ===")
compile_result = run("docker exec moduforge sh -c 'cd /src_new && CGO_ENABLED=1 go build -o /tmp/server_new ./cmd/moduforge'", check=False)
print(f"Compile result: {compile_result}")

# Check if binary exists
binary_check = run("docker exec moduforge ls -la /tmp/server_new 2>/dev/null || echo 'Binary not found'")
print(f"Binary check: {binary_check}")

if "Binary not found" in binary_check:
    print("\n=== Compilation failed, trying alternative ===")
    # Try using the overlay2 source directly
    run("docker cp /tmp/moduforge_build/src/internal/service/zipper.go moduforge:/src/internal/service/zipper.go")
    compile_result = run("docker exec moduforge sh -c 'cd /src && CGO_ENABLED=1 go build -o /tmp/server_new ./cmd/moduforge'", check=False)
    print(f"Compile result 2: {compile_result}")

print("\n=== Step 8: Deploy binary ===")
run("docker exec moduforge cp /tmp/server_new /app/server_new")
run("docker exec moduforge chmod +x /app/server_new")
run("docker exec moduforge ls -la /app/server_new")

print("\n=== Step 9: Stop and swap binary ===")
run("docker stop moduforge")
run("docker exec moduforge mv /app/server /app/server_old 2>/dev/null", check=False)
run("docker exec moduforge mv /app/server_new /app/server", check=False)

print("\n=== Step 10: Start container ===")
run("docker start moduforge")
run("sleep 5")
run("docker ps --filter name=moduforge --format '{{.Status}}'")
run("docker exec moduforge curl -s http://localhost:8080/health")

print("\n=== Step 11: Test export ===")
login_resp = run('''docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
import json
try:
    token = json.loads(login_resp).get('token', '')
    
    # List projects
    projects_resp = run(f'docker exec moduforge curl -s http://localhost:8080/api/v1/projects -H "Authorization: Bearer {token}"')
    projects = json.loads(projects_resp)
    
    if projects:
        pid = projects[0]['id']
        print(f"\nExporting project: {pid}")
        
        # Export zip
        status = run(f'docker exec moduforge curl -s -o /tmp/test_export.zip -w "%{{http_code}}" http://localhost:8080/api/v1/projects/{pid}/export-zip -X POST -H "Authorization: Bearer {token}"')
        print(f"Export status: {status}")
        
        # Copy to host
        run("docker cp moduforge:/tmp/test_export.zip /tmp/test_export.zip")
        
        # Check zip contents
        print("\n=== ZIP Contents ===")
        zip_list = run('python3 -c "import zipfile; z=zipfile.ZipFile(\'/tmp/test_export.zip\'); [print(n) for n in sorted(z.namelist())]"')
        print(zip_list)
        
        # Analysis
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
