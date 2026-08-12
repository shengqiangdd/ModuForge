import paramiko, json, time, os

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Step 1: Completely stop and REMOVE the container
print("=== Step 1: Stop and remove container ===")
out, err = run("docker stop moduforge 2>&1")
print(f"Stop: {out.strip()}")
out, err = run("docker rm moduforge 2>&1")
print(f"Remove: {out.strip()}")

time.sleep(2)

# Step 2: Verify no processes are using the DB
print("\n=== Step 2: Verify no processes ===")
out, err = run("docker ps | grep moduforge")
print(f"Running: '{out.strip()}' (should be empty)")

# Step 3: Delete ALL DB files from the volume
print("\n=== Step 3: Delete all DB files ===")
out, err = run("rm -f /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db*")
print(f"Remove: {out.strip()}")

# Verify deletion
out, err = run("ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/")
print(f"Remaining: {out.strip()}")

# Step 4: Copy the modified DB (from local)
print("\n=== Step 4: Copy modified DB to server ===")
sftp = client.open_sftp()
local_db = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge_edit.db'
sftp.put(local_db, '/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
sftp.close()

# Verify
out, err = run("ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db")
print(f"Copied: {out.strip()}")

# Step 5: Fix permissions
print("\n=== Step 5: Fix permissions ===")
out, err = run("chown 1000:1000 /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db 2>&1 || echo 'chown skipped'")
print(f"Chown: {out.strip()}")

# Step 6: Recreate container with docker compose
print("\n=== Step 6: Recreate container ===")
out, err = run("cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && docker compose up -d 2>&1")
print(out)

# Step 7: Wait for healthy
print("\n=== Step 7: Wait for healthy ===")
for i in range(20):
    time.sleep(3)
    out, err = run("docker exec moduforge wget -q -O /dev/null http://localhost:8086/health 2>&1 && echo 'healthy' || echo 'not ready'")
    if 'healthy' in out:
        print(f"Healthy after {(i+1)*3}s!")
        break
    print(f"  {(i+1)*3}s: waiting...")

# Step 8: Verify no deleted FDs
print("\n=== Step 8: Verify FDs ===")
out, err = run("docker exec moduforge sh -c 'ls -la /proc/1/fd/ 2>/dev/null | grep -E \"wal|shm|db\"'")
print(f"DB FDs: {out.strip()}")

# Step 9: Test API
print("\n=== Step 9: API Test ===")
stdin, stdout, stderr = client.exec_command("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(stdout.read().decode())
token = login.get('token', '')
print(f"Login: {login.get('user', {}).get('username', 'FAILED')}")

if token:
    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    resp = stdout.read().decode()
    try:
        projects = json.loads(resp)
        if isinstance(projects, list):
            print(f"\nProjects: {len(projects)} found!")
            for p in projects:
                print(f"  - {p.get('name', 'unnamed')}")
        else:
            print(f"Response: {resp[:300]}")
    except:
        print(f"Raw: {resp[:300]}")

    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    convs_resp = stdout.read().decode()
    try:
        convs = json.loads(convs_resp)
        if isinstance(convs, dict) and 'conversations' in convs:
            print(f"\nAI Conversations: {len(convs['conversations'])} found!")
        elif isinstance(convs, list):
            print(f"\nAI Conversations: {len(convs)} found!")
        else:
            print(f"\nConversations: {convs_resp[:200]}")
    except:
        print(f"\nConversations: {convs_resp[:200]}")

client.close()
print("\nDone!")
