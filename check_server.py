import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Get auth token
stdin, stdout, stderr = ssh.exec_command(
    "curl -s http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"admin\",\"password\":\"admin123\"}'",
    timeout=10
)
resp = json.loads(stdout.read().decode())
token = resp.get('token', '')

# Check error for one delete
cmd = f"curl -s http://localhost:8086/api/v1/projects/1785249992652501794-1864/files/assets/codemirror-DYH2DdWT.js -X DELETE -H 'Authorization: Bearer {token}'"
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
print("DELETE response:", stdout.read().decode())

# Check what files API returns
cmd = f"curl -s http://localhost:8086/api/v1/projects/1785249992652501794-1864/files -H 'Authorization: Bearer {token}'"
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
print("FILES response:", stdout.read().decode()[:2000])

# Try GET on one file
cmd = f"curl -s http://localhost:8086/api/v1/projects/1785249992652501794-1864/files/assets/codemirror-DYH2DdWT.js -H 'Authorization: Bearer {token}'"
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
print("GET file:", stdout.read().decode()[:500])

ssh.close()
