#!/usr/bin/env python3
import paramiko
import os

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    exit_status = stdout.channel.recv_exit_status()
    if err:
        print(f"STDERR: {err[:300]}")
    return out, exit_status

print("=== Step 1: Check available source locations ===")
out, _ = run("ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/")
print(out)

print("\n=== Step 2: Create build directory ===")
run("rm -rf /tmp/build_src")
run("mkdir -p /tmp/build_src")
run("cp -r /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/* /tmp/build_src/")
out, _ = run("ls /tmp/build_src/")
print(f"Files: {out}")

print("\n=== Step 3: Replace zipper.go ===")
sftp = ssh.open_sftp()
sftp.put(r"C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go", "/tmp/build_src/internal/service/zipper.go")
sftp.close()
out, _ = run("head -5 /tmp/build_src/internal/service/zipper.go")
print(f"First lines: {out}")

print("\n=== Step 4: Stop container ===")
run("docker stop moduforge")

print("\n=== Step 5: Copy source to container ===")
run("docker cp /tmp/build_src moduforge:/tmp/build_src")
out, _ = run("docker exec moduforge ls /tmp/build_src/")
print(f"In container: {out}")

print("\n=== Step 6: Compile in container ===")
out, status = run("docker exec moduforge sh -c 'cd /tmp/build_src && CGO_ENABLED=1 go build -o /tmp/server_new ./cmd/moduforge 2>&1'")
print(f"Compile output: {out}")
print(f"Compile status: {status}")

if status == 0:
    print("\n=== Step 7: Deploy binary ===")
    run("docker exec moduforge cp /tmp/server_new /app/server")
    run("docker exec moduforge chmod +x /app/server")
    out, _ = run("docker exec moduforge ls -la /app/server")
    print(f"Binary: {out}")
    
    print("\n=== Step 8: Start container ===")
    run("docker start moduforge")
    import time
    time.sleep(5)
    out, _ = run("docker ps --filter name=moduforge --format '{{.Status}}'")
    print(f"Status: {out}")
    out, _ = run("docker exec moduforge curl -s http://localhost:8080/health")
    print(f"Health: {out}")
    
    print("\n=== Step 9: Test export ===")
    login_resp, _ = run('''docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
    import json
    token = json.loads(login_resp).get('token', '')
    
    projects_resp, _ = run(f'docker exec moduforge curl -s http://localhost:8080/api/v1/projects -H "Authorization: Bearer {token}"')
    projects = json.loads(projects_resp)
    
    if projects:
        pid = projects[0]['id']
        print(f"Exporting project: {pid}")
        
        status, _ = run(f'docker exec moduforge curl -s -o /tmp/test_export.zip -w "%{{http_code}}" http://localhost:8080/api/v1/projects/{pid}/export-zip -X POST -H "Authorization: Bearer {token}"')
        print(f"Export status: {status}")
        
        run("docker cp moduforge:/tmp/test_export.zip /tmp/test_export.zip")
        
        print("\n=== ZIP Contents ===")
        zip_list, _ = run('python3 -c "import zipfile; z=zipfile.ZipFile(\'/tmp/test_export.zip\'); [print(n) for n in sorted(z.namelist())]"')
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
else:
    print("\n=== Compilation failed, checking error ===")
    out, _ = run("docker exec moduforge sh -c 'cd /tmp/build_src && go build -o /tmp/server_new ./cmd/moduforge 2>&1 | head -50'")
    print(out)

ssh.close()
