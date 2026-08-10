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
    if err:
        print(f"STDERR: {err[:500]}")
    return out, exit_status

print("=== Step 1: Check container status ===")
out, _ = run("docker ps -a --filter name=moduforge --format '{{.Names}} {{.Status}}'")
print(f"Container: {out}")

print("\n=== Step 2: Start container ===")
run("docker start moduforge")
time.sleep(3)
out, _ = run("docker ps --filter name=moduforge --format '{{.Status}}'")
print(f"Status: {out}")

print("\n=== Step 3: Check logs ===")
out, _ = run("docker logs --tail 10 moduforge 2>&1")
print(out)

print("\n=== Step 4: Copy source to container ===")
run("docker cp /tmp/build_src moduforge:/tmp/build_src")
out, _ = run("docker exec moduforge ls /tmp/build_src/")
print(f"In container: {out}")

print("\n=== Step 5: Compile ===")
out, status = run("docker exec moduforge sh -c 'cd /tmp/build_src && CGO_ENABLED=1 go build -o /tmp/server_new ./cmd/moduforge 2>&1'")
print(f"Compile: {out}")
print(f"Status: {status}")

if status == 0:
    print("\n=== Step 6: Deploy ===")
    run("docker exec moduforge cp /tmp/server_new /app/server")
    run("docker exec moduforge chmod +x /app/server")
    run("docker exec moduforge ls -la /app/server")
    
    print("\n=== Step 7: Restart ===")
    run("docker restart moduforge")
    time.sleep(5)
    out, _ = run("docker exec moduforge curl -s http://localhost:8080/health")
    print(f"Health: {out}")
    
    print("\n=== Step 8: Test export ===")
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
        has_tmp = any(n.startswith('tmp/') for n in lines)
        has_design = any('DESIGN_DOC' in n for n in lines)
        has_backend = any('backend/' in n for n in lines)
        
        print(f"Has webroot wrapper: {'YES' if has_webroot else 'NO'}")
        print(f"Excludes tmp/: {'YES' if not has_tmp else 'NO'}")
        print(f"Excludes DESIGN_DOC.md: {'YES' if not has_design else 'NO'}")
        print(f"Excludes backend/: {'YES' if not has_backend else 'NO'}")
else:
    print("\n=== Compilation failed ===")

ssh.close()
