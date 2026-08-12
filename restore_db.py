import paramiko, time

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    out = stdout.read().decode('utf-8', errors='replace')
    err = stderr.read().decode('utf-8', errors='replace')
    return out, err

# Step 1: Stop container
print("Step 1: Stopping container...")
out, err = run('docker stop moduforge')
print(f"Stop: {out.strip()} {err.strip()}")

# Step 2: Replace DB
print("\nStep 2: Replacing current DB with backup...")
out, err = run('docker exec moduforge sh -c "cp /data/moduforge.db.bak /data/moduforge.db && cp /data/moduforge.db.bak /data/moduforge.db-shm /data/moduforge.db-wal 2>/dev/null; echo done"')
print(f"Copy: {out.strip()} {err.strip()}")

# Actually, we need to do this while the container is stopped, or use a different approach
# Let's use the volume mountpoint directly
print("\nStep 2b: Direct file replacement via host...")
out, err = run('cp /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db.bak /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
print(f"CP: {out.strip()} {err.strip()}")

# Remove WAL and SHM to avoid conflicts
out, err = run('rm -f /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-shm /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-wal')
print(f"RM WAL: {out.strip()} {err.strip()}")

# Step 3: Start container
print("\nStep 3: Starting container...")
out, err = run('docker start moduforge')
print(f"Start: {out.strip()} {err.strip()}")

# Wait for container to be ready
print("\nWaiting for container to be healthy...")
for i in range(10):
    time.sleep(2)
    out, err = run('docker exec moduforge wget -q -O /dev/null http://localhost:8086/health 2>&1 && echo "healthy" || echo "not ready"')
    status = out.strip()
    print(f"  {i*2}s: {status}")
    if 'healthy' in status:
        break

# Step 4: Verify data
print("\nStep 4: Verifying data...")
import json

# Login
stdin, stdout, stderr = client.exec_command("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(stdout.read().decode())
token = login.get('token', '')
print(f"Login: {login.get('user', {}).get('username', 'FAILED')}")

if token:
    # Check projects
    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    projects = stdout.read().decode()
    try:
        plist = json.loads(projects)
        if plist:
            print(f"Projects: {len(plist)} found!")
            for p in plist[:5]:
                print(f"  - {p.get('name', 'unnamed')} ({p.get('id', '')[:8]})")
        else:
            print("Projects: EMPTY")
    except:
        print(f"Projects response: {projects[:200]}")

    # Check AI conversations
    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    convs = stdout.read().decode()
    try:
        clist = json.loads(convs)
        if clist:
            print(f"AI Conversations: {len(clist)} found!")
        else:
            print("AI Conversations: EMPTY")
    except:
        print(f"Conversations response: {convs[:200]}")

client.close()
print("\nDone!")
