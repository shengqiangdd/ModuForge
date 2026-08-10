#!/usr/bin/env python3
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    if err:
        print(f"STDERR: {err}")
    return out

# Check container status
print("=== Container Status ===")
print(run("docker ps --filter name=moduforge --format '{{.Names}} {{.Status}}'"))

# Check health
print("\n=== Health Check ===")
print(run("docker exec moduforge curl -s http://localhost:8080/health"))

# Test login
print("\n=== Login Test ===")
login_resp = run('''docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
print(f"Login response: {login_resp[:200]}")

try:
    token = json.loads(login_resp).get('token', '')
    print(f"Token: {token[:40]}...")
    
    # List projects
    print("\n=== Projects ===")
    projects_resp = run(f'docker exec moduforge curl -s http://localhost:8080/api/v1/projects -H "Authorization: Bearer {token}"')
    print(f"Projects response: {projects_resp[:300]}")
    
    projects = json.loads(projects_resp)
    if projects:
        pid = projects[0]['id']
        print(f"\n=== Exporting Project: {pid} ===")
        
        # Export zip
        status = run(f'docker exec moduforge curl -s -o /tmp/test_export.zip -w "%{{http_code}}" http://localhost:8080/api/v1/projects/{pid}/export-zip -H "Authorization: Bearer {token}"')
        print(f"Export status: {status}")
        
        # Copy to host
        run("docker cp moduforge:/tmp/test_export.zip /tmp/test_export.zip")
        
        # Check zip contents
        print("\n=== Zip Contents ===")
        zip_list = run('python3 -c "import zipfile; z=zipfile.ZipFile(\'/tmp/test_export.zip\'); [print(n) for n in sorted(z.namelist())]"')
        print(zip_list)
        
        # Check if webroot exists
        lines = zip_list.split('\n')
        has_webroot = any('webroot' in n for n in lines)
        has_tmp = any(n.startswith('tmp/') for n in lines)
        has_design = any('DESIGN_DOC' in n for n in lines)
        
        print(f"\n✓ Has webroot wrapper: {has_webroot}")
        print(f"{'✗' if has_tmp else '✓'} Excludes tmp/: {'NO' if has_tmp else 'YES'}")
        print(f"{'✗' if has_design else '✓'} Excludes DESIGN_DOC.md: {'NO' if has_design else 'YES'}")
    else:
        print("No projects found")
except Exception as e:
    print(f"Error: {e}")

ssh.close()
