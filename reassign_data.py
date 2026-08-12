import paramiko, time

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Stop container to safely modify DB
print("Stopping container...")
run('docker stop moduforge')
time.sleep(2)

# Reassign all projects to current admin user
print("\nReassigning projects...")
sql_projects = "UPDATE projects SET user_id = 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748' WHERE user_id != 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748';"
out, err = run(f'docker exec moduforge sh -c "sqlite3 /data/moduforge.db \'{sql_projects}\'" 2>&1')
print(f"Projects: {out.strip()} {err.strip()}")

# Reassign all AI conversations to current admin user
print("\nReassigning AI conversations...")
sql_convs = "UPDATE ai_conversations SET user_id = 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748' WHERE user_id != 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748';"
out, err = run(f'docker exec moduforge sh -c "sqlite3 /data/moduforge.db \'{sql_convs}\'" 2>&1')
print(f"Conversations: {out.strip()} {err.strip()}")

# Reassign conversation_messages
print("\nReassigning conversation_messages...")
sql_msgs = "UPDATE conversation_messages SET user_id = 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748' WHERE user_id != 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748';"
out, err = run(f'docker exec moduforge sh -c "sqlite3 /data/moduforge.db \'{sql_msgs}\'" 2>&1')
print(f"Messages: {out.strip()} {err.strip()}")

# Reassign build_tasks
print("\nReassigning build_tasks...")
sql_builds = "UPDATE build_tasks SET user_id = 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748' WHERE user_id != 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748';"
out, err = run(f'docker exec moduforge sh -c "sqlite3 /data/moduforge.db \'{sql_builds}\'" 2>&1')
print(f"Build tasks: {out.strip()} {err.strip()}")

# Reassign other tables
for table in ['favorites', 'search_history', 'notifications', 'activities', 'audit_logs', 'agent_memory', 'custom_skills', 'agent_presets', 'recycle_bin', 'todo_lists', 'todo_items']:
    sql = f"UPDATE {table} SET user_id = 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748' WHERE user_id != 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748';"
    out, err = run(f'docker exec moduforge sh -c "sqlite3 /data/moduforge.db \'{sql}\'" 2>&1')
    if out.strip():
        print(f"{table}: {out.strip()}")

# Verify
print("\n=== Verification ===")
out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT COUNT(*) FROM projects;\\""')
print(f"Projects count: {out.strip()}")

out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT COUNT(*) FROM ai_conversations;\\""')
print(f"AI conversations count: {out.strip()}")

out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT COUNT(*) FROM conversation_messages;\\""')
print(f"Messages count: {out.strip()}")

# Restart container
print("\nStarting container...")
run('docker start moduforge')
time.sleep(3)

# Wait for health
for i in range(10):
    time.sleep(2)
    out, err = run('docker exec moduforge wget -q -O /dev/null http://localhost:8086/health 2>&1 && echo "healthy" || echo "not ready"')
    if 'healthy' in out:
        print(f"Container healthy after {(i+1)*2}s")
        break

# Final verification via API
import json
print("\n=== API Verification ===")
stdin, stdout, stderr = client.exec_command("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(stdout.read().decode())
token = login.get('token', '')
print(f"Login: {login.get('user', {}).get('username', 'FAILED')}")

if token:
    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    projects = json.loads(stdout.read().decode())
    print(f"\nProjects: {len(projects)} found!")
    for p in projects:
        print(f"  - {p.get('name', 'unnamed')} (id: {p.get('id', '')[:8]})")

    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    convs = json.loads(stdout.read().decode())
    print(f"\nAI Conversations: {len(convs)} found!")
    for cv in convs[:5]:
        print(f"  - {cv.get('title', 'untitled')[:50]}")

client.close()
print("\nDone!")
