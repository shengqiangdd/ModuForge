import paramiko, json

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Check current container volume mounts
print("=== Current container mounts ===")
out, err = run('docker inspect moduforge --format="{{json .Mounts}}"')
mounts = json.loads(out)
for m in mounts:
    print(f"  {m['Type']}: {m['Source']} -> {m['Destination']} (name: {m.get('Name', 'N/A')})")

# Check which volume the compose file maps to
print("\n=== Compose volume name ===")
out, err = run("docker inspect moduforge --format='{{index .Config.Labels \"com.docker.compose.project\"}}' 2>&1")
print(f"Project: {out.strip()}")
out, err = run("docker inspect moduforge --format='{{index .Config.Labels \"com.docker.compose.volumes\"}}' 2>&1")
print(f"Volumes label: {out.strip()}")

# Check if moduforge_data volume has data
print("\n=== moduforge_data volume ===")
out, err = run("ls -la /vol1/docker/volumes/moduforge_data/_data/ 2>/dev/null")
print(out)

# Check if moduforge_moduforge_data volume has data
print("\n=== moduforge_moduforge_data volume ===")
out, err = run("ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/ 2>/dev/null")
print(out)

# The compose file uses 'moduforge_data' not 'moduforge_moduforge_data'!
# We need to copy the DB to the RIGHT volume
print("\n=== Copy DB to correct volume ===")
out, err = run("cp /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db /vol1/docker/volumes/moduforge_data/_data/moduforge.db 2>&1")
print(f"Copy: {out.strip()}")

# Check
out, err = run("ls -la /vol1/docker/volumes/moduforge_data/_data/moduforge.db* 2>/dev/null")
print(f"Files: {out.strip()}")

# Fix permissions
out, err = run("chown 1000:1000 /vol1/docker/volumes/moduforge_data/_data/moduforge.db 2>&1")
print(f"Chown: {out.strip()}")

# Restart container
print("\n=== Restart container ===")
out, err = run("docker restart moduforge 2>&1")
print(f"Restart: {out.strip()}")

import time
time.sleep(10)

# Wait for healthy
for i in range(10):
    time.sleep(3)
    out, err = run("docker exec moduforge wget -q -O /dev/null http://localhost:8086/health 2>&1 && echo 'healthy' || echo 'not ready'")
    if 'healthy' in out:
        print(f"Healthy after {(i+1)*3}s!")
        break

# Test API
print("\n=== API Test ===")
out, err = run("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(out)
token = login.get('token', '')
print(f"Login: {login.get('user', {}).get('username', 'FAILED')}")

if token:
    out, err = run(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    try:
        projects = json.loads(out)
        if isinstance(projects, list):
            print(f"\nProjects: {len(projects)} found!")
            for p in projects:
                print(f"  - {p.get('name', 'unnamed')}")
        else:
            print(f"Response: {out[:300]}")
    except:
        print(f"Raw: {out[:300]}")

client.close()
print("\nDone!")
