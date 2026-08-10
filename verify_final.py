#!/usr/bin/env python3
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return out

# Login
login_resp = run('''docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
token = json.loads(login_resp).get('token', '')
pid = "03be8376-7f72-44c4-8509-759c87708e4f"

# Export with POST
print("=== Exporting ZIP ===")
result = run(f'docker exec moduforge curl -s -o /tmp/exported.zip -w "HTTP:%{{http_code}} Size:%{{size_download}}" http://localhost:8080/api/v1/projects/{pid}/export-zip -X POST -H "Authorization: Bearer {token}"')
print(f"Result: {result}")

# Copy to host
print("\n=== Copying to Host ===")
run("docker cp moduforge:/tmp/exported.zip /tmp/exported.zip")
run("docker exec moduforge ls -la /tmp/exported.zip")

# Check zip contents
print("\n=== ZIP Contents ===")
zip_list = run('python3 -c "import zipfile; z=zipfile.ZipFile(\'/tmp/exported.zip\'); [print(n) for n in sorted(z.namelist())]"')
print(zip_list)

# Analysis
print("\n=== Analysis ===")
lines = [n for n in zip_list.split('\n') if n.strip()]
has_webroot = any('webroot' in n for n in lines)
has_tmp = any(n.startswith('tmp/') for n in lines)
has_design = any('DESIGN_DOC' in n for n in lines)
has_upload = any(n == 'upload' for n in lines)
has_backend = any('backend/' in n for n in lines)

print(f"Has webroot wrapper: {'YES' if has_webroot else 'NO'}")
print(f"Excludes tmp/: {'YES' if not has_tmp else 'NO'}")
print(f"Excludes DESIGN_DOC.md: {'YES' if not has_design else 'NO'}")
print(f"Excludes upload: {'YES' if not has_upload else 'NO'}")
print(f"Excludes backend/: {'YES' if not has_backend else 'NO'}")

ssh.close()
