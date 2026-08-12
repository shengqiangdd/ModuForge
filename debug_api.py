import paramiko, json

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Check container env vars
print("=== Container ENV ===")
out, err = run('docker inspect moduforge --format="{{range .Config.Env}}{{println .}}{{end}}"')
for line in out.strip().split('\n'):
    if 'DATABASE' in line or 'DB' in line or 'DATA' in line or 'PORT' in line:
        print(f"  {line}")

# Check container mount details
print("\n=== Container Mounts ===")
out, err = run('docker inspect moduforge --format="{{json .Mounts}}"')
mounts = json.loads(out)
for m in mounts:
    print(f"  {m['Type']}: {m['Source']} -> {m['Destination']}")

# Check what DB file the container sees
print("\n=== Container /data contents ===")
out, err = run('docker exec moduforge ls -la /data/')
print(out)

# Try to run sqlite3 in the container using the binary
print("\n=== Try using the binary to query ===")
out, err = run('docker exec moduforge find / -name "moduforge" -type f 2>/dev/null | head -3')
print(f"Binary: {out.strip()}")

# Check the API response in detail
print("\n=== API Projects Detail ===")
stdin, stdout, stderr = client.exec_command("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(stdout.read().decode())
token = login.get('token', '')

if token:
    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    resp = stdout.read().decode()
    print(f"Response: {resp[:500]}")
    
    # Try admin endpoint
    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/admin/projects -H "Authorization: Bearer {token}" 2>&1')
    resp2 = stdout.read().decode()
    print(f"\nAdmin projects: {resp2[:500]}")

# Check if there's a different DB path
print("\n=== Check for other DB files ===")
out, err = run('docker exec moduforge find / -name "*.db" -type f 2>/dev/null | head -10')
print(out)

client.close()
