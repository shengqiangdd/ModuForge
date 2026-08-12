import paramiko, json, time, os

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# 1. Read the docker-compose.yml
print("=== docker-compose.yml ===")
out, err = run('cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml 2>/dev/null')
print(out[:3000])

# 2. Check binary for DB path strings
print("\n=== Binary DB strings ===")
out, err = run("docker exec moduforge sh -c 'strings /server 2>/dev/null | grep -i database | head -20'")
print(out)

# 3. Check /proc for open file descriptors
print("\n=== Open FDs in container ===")
out, err = run("docker exec moduforge sh -c 'ls -la /proc/1/fd/ 2>/dev/null | head -20'")
print(out)

# 4. Check /proc maps for mmap'd files
print("\n=== Memory maps ===")
out, err = run("docker exec moduforge sh -c 'cat /proc/1/maps 2>/dev/null | grep db | head -10'")
print(out)

# 5. The real test: kill the process and let Docker restart it
print("\n=== Kill process inside container to force restart ===")
out, err = run("docker exec moduforge kill -9 1 2>&1")
print(f"Kill: {out.strip()} {err.strip()}")

# Wait for Docker to restart the container
print("\nWaiting for container to restart...")
for i in range(15):
    time.sleep(2)
    out, err = run('docker ps | grep moduforge')
    if 'healthy' in out or 'Up' in out:
        print(f"  {(i+1)*2}s: Container running")
        # Check health
        out2, err2 = run('docker exec moduforge wget -q -O /dev/null http://localhost:8086/health 2>&1 && echo "healthy" || echo "not ready"')
        if 'healthy' in out2:
            print(f"  Container healthy!")
            break
    else:
        print(f"  {(i+1)*2}s: Waiting...")

# 6. Test API immediately
print("\n=== API Test after restart ===")
import json as j
stdin, stdout, stderr = client.exec_command("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = j.loads(stdout.read().decode())
token = login.get('token', '')

if token:
    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    resp = stdout.read().decode()
    try:
        projects = j.loads(resp)
        if isinstance(projects, list):
            print(f"Projects: {len(projects)} found!")
            for p in projects:
                print(f"  - {p.get('name', 'unnamed')} (user: {p.get('user_id', 'N/A')[:8]})")
        else:
            print(f"Response: {resp[:300]}")
    except:
        print(f"Raw: {resp[:300]}")

    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    convs_resp = stdout.read().decode()
    try:
        convs = j.loads(convs_resp)
        if isinstance(convs, dict) and 'conversations' in convs:
            print(f"\nAI Conversations: {len(convs['conversations'])} found!")
        elif isinstance(convs, list):
            print(f"\nAI Conversations: {len(convs)} found!")
        else:
            print(f"\nConversations: {convs_resp[:200]}")
    except:
        print(f"\nConversations: {convs_resp[:200]}")

# 7. Double check DB state after restart
print("\n=== DB state after restart ===")
out, err = run("docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT COUNT(*) FROM projects;\"'")
print(f"Projects in DB: {out.strip()}")
out, err = run("docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT user_id, COUNT(*) FROM projects GROUP BY user_id;\"'")
print(f"By user: {out.strip()}")

client.close()
print("\nDone!")
