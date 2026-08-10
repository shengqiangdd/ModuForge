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

# Check API routes in source code
print("=== Checking API Routes ===")
print(run("docker exec moduforge grep -r 'export' /app/backend/cmd/ 2>/dev/null || echo 'No matches'"))
print(run("docker exec moduforge grep -r 'Export' /app/backend/internal/ 2>/dev/null | head -20"))

# Try GET method instead of POST
print("\n=== Testing GET Method ===")
login_resp = run('''docker exec moduforge curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
token = json.loads(login_resp).get('token', '')
pid = "03be8376-7f72-44c4-8509-759c87708e4f"

# Try different methods
for method in ["GET", "POST", "PUT"]:
    status = run(f'docker exec moduforge curl -s -o /dev/null -w "%{{http_code}}" http://localhost:8080/api/v1/projects/{pid}/export-zip -X {method} -H "Authorization: Bearer {token}"')
    print(f"{method}: {status}")

# Check if there's a download endpoint
print("\n=== Checking Download Endpoint ===")
print(run("docker exec moduforge grep -r 'download' /app/backend/cmd/ 2>/dev/null | head -10"))

ssh.close()
