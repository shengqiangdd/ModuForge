import paramiko, time, sqlite3, os

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

sftp = client.open_sftp()

# Copy DB to local
local_db = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge_edit.db'
print("Copying DB from server...")
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db', local_db)
print(f"Copied: {os.path.getsize(local_db)} bytes")

# Modify locally
conn = sqlite3.connect(local_db)
c = conn.cursor()

ADMIN_ID = 'f7b4d6fa-0fe5-4f16-9390-a5fdb0905748'

tables_with_user_id = [
    'projects', 'ai_conversations', 'conversation_messages', 'build_tasks',
    'favorites', 'search_history', 'notifications', 'activities', 
    'audit_logs', 'agent_memory', 'custom_skills', 'agent_presets',
    'recycle_bin', 'todo_lists', 'todo_items', 'session_summaries',
    'project_knowledge', 'skill_evolution', 'collaboration_sessions',
    'app_logs', 'backup_schedules', 'permission_audits', 'file_comments'
]

for table in tables_with_user_id:
    try:
        # Check if table exists and has user_id column
        cols = [row[1] for row in c.execute(f"PRAGMA table_info({table})").fetchall()]
        if 'user_id' in cols:
            before = c.execute(f"SELECT COUNT(*) FROM {table} WHERE user_id != ?", (ADMIN_ID,)).fetchone()[0]
            c.execute(f"UPDATE {table} SET user_id = ? WHERE user_id != ?", (ADMIN_ID, ADMIN_ID))
            after = c.execute(f"SELECT COUNT(*) FROM {table} WHERE user_id = ?", (ADMIN_ID,)).fetchone()[0]
            print(f"  {table}: reassigned {before} rows, total {after} for admin")
    except Exception as e:
        print(f"  {table}: skipped ({e})")

# Also update projects that have NULL user_id
c.execute("UPDATE projects SET user_id = ? WHERE user_id IS NULL OR user_id = ''", (ADMIN_ID,))
c.execute("UPDATE ai_conversations SET user_id = ? WHERE user_id IS NULL OR user_id = ''", (ADMIN_ID,))
c.execute("UPDATE conversation_messages SET user_id = ? WHERE user_id IS NULL OR user_id = ''", (ADMIN_ID,))

conn.commit()

# Verify
print("\n=== Verification ===")
print(f"Projects: {c.execute('SELECT COUNT(*) FROM projects').fetchone()[0]}")
print(f"AI conversations: {c.execute('SELECT COUNT(*) FROM ai_conversations').fetchone()[0]}")
print(f"Messages: {c.execute('SELECT COUNT(*) FROM conversation_messages').fetchone()[0]}")
print(f"Build tasks: {c.execute('SELECT COUNT(*) FROM build_tasks').fetchone()[0]}")

# Show projects
print("\nProjects:")
for row in c.execute('SELECT id, name, user_id FROM projects').fetchall():
    print(f"  {row[1]} (user: {row[2][:8] if row[2] else 'None'})")

conn.close()

# Copy back to server
print("\nCopying modified DB back to server...")
sftp.put(local_db, '/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
print("Copied back!")

# Remove WAL and SHM to avoid conflicts
def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace')

print("\nRemoving WAL/SHM...")
run('rm -f /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-shm /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-wal')

# Start container
print("\nStarting container...")
run('docker start moduforge')
time.sleep(3)

# Wait for health
for i in range(10):
    time.sleep(2)
    out = run('docker exec moduforge wget -q -O /dev/null http://localhost:8086/health 2>&1 && echo "healthy" || echo "not ready"')
    if 'healthy' in out:
        print(f"Container healthy after {(i+1)*2}s")
        break

# Final API check
import json
print("\n=== Final API Check ===")
stdin, stdout, stderr = client.exec_command("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(stdout.read().decode())
token = login.get('token', '')

if token:
    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    projects = json.loads(stdout.read().decode())
    print(f"Projects: {len(projects)} found!")
    for p in projects:
        print(f"  - {p.get('name', 'unnamed')}")

    stdin, stdout, stderr = client.exec_command(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    convs_resp = stdout.read().decode()
    try:
        convs = json.loads(convs_resp)
        if isinstance(convs, list):
            print(f"AI Conversations: {len(convs)} found!")
            for cv in convs[:5]:
                print(f"  - {cv.get('title', 'untitled')[:50]}")
        else:
            print(f"Conversations: {convs_resp[:200]}")
    except:
        print(f"Conversations: {convs_resp[:200]}")

sftp.close()
client.close()
print("\nDone!")
