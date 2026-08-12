import paramiko, json, time

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

ADMIN_UID = 'fec17bd3-7610-4f2a-b157-24ee1e362d23'

# Step 1: Stop container
print("=== Step 1: Stop container ===")
run("docker stop moduforge")
time.sleep(2)

# Step 2: Copy DB to local, fix, copy back
print("\n=== Step 2: Copy DB locally and fix ===")
sftp = client.open_sftp()
local_db = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge_fix.db'

# Copy from server
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db', local_db)
print("Copied to local")

# Fix locally with Python sqlite3
import sqlite3, os
conn = sqlite3.connect(local_db)
c = conn.cursor()

# Reassign all data to the correct admin user
tables = ['projects', 'ai_conversations', 'conversation_messages', 'build_tasks',
          'favorites', 'search_history', 'notifications', 'activities',
          'audit_logs', 'agent_memory', 'custom_skills', 'agent_presets',
          'recycle_bin', 'todo_lists', 'todo_items', 'session_summaries',
          'project_knowledge', 'skill_evolution', 'collaboration_sessions',
          'backup_schedules', 'file_comments']

for table in tables:
    try:
        cols = [row[1] for row in c.execute(f"PRAGMA table_info({table})").fetchall()]
        if 'user_id' in cols:
            before = c.execute(f"SELECT COUNT(*) FROM {table} WHERE user_id != ?", (ADMIN_UID,)).fetchone()[0]
            c.execute(f"UPDATE {table} SET user_id = ? WHERE user_id != ?", (ADMIN_UID, ADMIN_UID))
            c.execute(f"UPDATE {table} SET user_id = ? WHERE user_id IS NULL OR user_id = ''", (ADMIN_UID,))
            after = c.execute(f"SELECT COUNT(*) FROM {table} WHERE user_id = ?", (ADMIN_UID,)).fetchone()[0]
            print(f"  {table}: reassigned {before} rows, total {after} for admin")
    except Exception as e:
        print(f"  {table}: skipped ({e})")

# Also fix user99999 which has NULL id
c.execute("UPDATE projects SET user_id = ? WHERE user_id IS NULL", (ADMIN_UID,))
c.execute("UPDATE projects SET user_id = ? WHERE user_id = 'demo'", (ADMIN_UID,))

conn.commit()

# Verify
print("\n=== Verification ===")
print(f"Projects: {c.execute('SELECT COUNT(*) FROM projects').fetchone()[0]}")
print(f"AI conversations: {c.execute('SELECT COUNT(*) FROM ai_conversations').fetchone()[0]}")
print(f"Messages: {c.execute('SELECT COUNT(*) FROM conversation_messages').fetchone()[0]}")
print(f"Build tasks: {c.execute('SELECT COUNT(*) FROM build_tasks').fetchone()[0]}")

# Check user_id distribution
print("\nProjects by user_id:")
for row in c.execute('SELECT user_id, COUNT(*) FROM projects GROUP BY user_id').fetchall():
    print(f"  {row[0]}: {row[1]}")

conn.close()

# Copy back to server
sftp.put(local_db, '/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
print("\nCopied back to server")

# Remove WAL/SHM to avoid conflicts
run("rm -f /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-wal /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-shm")

# Fix permissions
run("chmod 666 /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db")

sftp.close()

# Step 3: Start container
print("\n=== Step 3: Start container ===")
run("docker start moduforge")

# Wait for healthy
print("\nWaiting for healthy...")
for i in range(15):
    time.sleep(3)
    out, err = run("docker exec moduforge wget -q -O /dev/null http://localhost:8086/health 2>&1 && echo 'healthy' || echo 'not ready'")
    if 'healthy' in out:
        print(f"Healthy after {(i+1)*3}s!")
        break

# Step 4: Test API
print("\n=== Step 4: API Test ===")
out, err = run("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(out)
token = login.get('token', '')
print(f"Login: {login.get('user', {}).get('username', 'FAILED')} (uid: {login.get('user', {}).get('id', 'N/A')[:8]})")

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

    out, err = run(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    try:
        convs = json.loads(out)
        if isinstance(convs, dict) and 'conversations' in convs:
            print(f"\nAI Conversations: {len(convs['conversations'])} found!")
        elif isinstance(convs, list):
            print(f"\nAI Conversations: {len(convs)} found!")
    except:
        pass

client.close()
print("\nDone!")
