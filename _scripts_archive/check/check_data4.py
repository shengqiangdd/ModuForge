import paramiko
import json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode() + stderr.read().decode()

# Login
login_out = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
token = json.loads(login_out)['token']

# Check AndroBoost-SmartTune project files via API
print("=== AndroBoost-SmartTune files ===")
project_id = "155f1629-6e33-4407-b348-f28698f6f5cd"
files_out = run(f'curl -s http://localhost:8086/api/v1/projects/{project_id}/files -H "Authorization: Bearer {token}"')
files = json.loads(files_out)
if isinstance(files, list):
    print(f"Total files: {len(files)}")
    for f in files[:20]:
        path = f.get('path', f.get('name', 'unknown'))
        content = f.get('content', '')
        size = len(content) if content else 0
        print(f"  {path} ({size} bytes)")
    if len(files) > 20:
        print(f"  ... and {len(files) - 20} more")
else:
    print(files_out)

# Check file content of a key file
print("\n=== Check specific file content ===")
for fname in ["src/rust/src/main.rs", "src/go/main.go", "src/cpp/main.cpp"]:
    file_out = run(f'curl -s "http://localhost:8086/api/v1/projects/{project_id}/files/{fname}" -H "Authorization: Bearer {token}"')
    try:
        data = json.loads(file_out)
        content = data.get('content', '')
        print(f"{fname}: {len(content)} bytes, first 100 chars: {content[:100]}")
    except:
        print(f"{fname}: {file_out[:200]}")

ssh.close()
