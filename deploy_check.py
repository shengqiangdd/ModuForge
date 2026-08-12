import paramiko, json

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

# Check current volume mounts
print("=== Current volumes ===")
stdin, stdout, stderr = client.exec_command('docker inspect moduforge --format="{{range .Mounts}}{{.Type}}: {{.Source}} -> {{.Destination}}{{println}}{{end}}" 2>&1')
print(stdout.read().decode())

# Check all moduforge volumes
print("\n=== All moduforge volumes ===")
stdin, stdout, stderr = client.exec_command('docker volume ls | grep moduforge 2>&1')
print(stdout.read().decode())

# Check data in current DB
print("\n=== Current DB data ===")
stdin, stdout, stderr = client.exec_command('docker exec moduforge sh -c "ls -la /data/ 2>&1"')
print(stdout.read().decode())

# Login and check
print("\n=== Login test ===")
stdin, stdout, stderr = client.exec_command("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login_resp = stdout.read().decode()
print(login_resp[:200])

try:
    token = json.loads(login_resp).get('token', '')
    if token:
        stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}" 2>&1')
        projects = stdout.read().decode()
        print(f"\nProjects: {projects[:300]}")
except:
    print("Login failed")

# Check old volumes for data
print("\n=== Old volume data ===")
stdin, stdout, stderr = client.exec_command('docker volume inspect moduforge_moduforge_data --format="{{.Mountpoint}}" 2>&1')
old_vol = stdout.read().decode().strip()
print(f"Old volume mountpoint: {old_vol}")

if old_vol:
    stdin, stdout, stderr = client.exec_command(f'ls -la {old_vol}/ 2>&1')
    print(stdout.read().decode())

client.close()
