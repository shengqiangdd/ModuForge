import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode() + stderr.read().decode()

# Get JWT token
import json
login_out = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
login_data = json.loads(login_out)
token = login_data['token']
print(f"Token obtained: {token[:30]}...")

# Check API endpoints to see data
print("\n=== Projects list ===")
print(run(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"'))

print("\n=== Projects with files ===")
print(run(f'curl -s "http://localhost:8086/api/v1/projects?include=files" -H "Authorization: Bearer {token}"'))

# Check where the database actually is inside container
print("\n=== Container process ===")
print(run('docker exec moduforge ps aux 2>&1'))

print("\n=== Container env ===")
print(run('docker exec moduforge env | grep -i database 2>&1'))
print(run('docker exec moduforge env | grep -i db 2>&1'))
print(run('docker exec moduforge env | grep -i data 2>&1'))
print(run('docker exec moduforge env | grep -i port 2>&1'))

# Find database file inside container
print("\n=== Find db in container ===")
print(run('docker exec moduforge find / -name "moduforge.db" -type f 2>&1'))

# Check volume mounts
print("\n=== Volume mounts ===")
print(run('docker inspect moduforge --format "{{json .Mounts}}" 2>&1'))

ssh.close()
