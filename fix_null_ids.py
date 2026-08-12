import paramiko, json, time, uuid

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Stop container
print("=== Stop container ===")
run("docker stop moduforge")
time.sleep(2)

# Copy DB locally
sftp = client.open_sftp()
local_db = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge_fix2.db'
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db', local_db)
print("Copied to local")

# Fix NULL IDs
import sqlite3
conn = sqlite3.connect(local_db)
c = conn.cursor()

# Check projects with NULL or empty IDs
print("\n=== Projects with NULL/empty IDs ===")
null_projects = c.execute("SELECT rowid, id, name, user_id FROM projects WHERE id IS NULL OR id = ''").fetchall()
print(f"Found {len(null_projects)} projects with NULL/empty IDs")

for row in null_projects:
    rowid, old_id, name, user_id = row
    new_id = str(uuid.uuid4())[:8]
    print(f"  rowid={rowid}, name='{name[:30]}', assigning id={new_id}")
    c.execute("UPDATE projects SET id = ? WHERE rowid = ?", (new_id, rowid))

# Also check for duplicate IDs
print("\n=== Check for duplicate IDs ===")
dupes = c.execute("SELECT id, COUNT(*) FROM projects WHERE id IS NOT NULL GROUP BY id HAVING COUNT(*) > 1").fetchall()
print(f"Found {len(dupes)} duplicate IDs")
for row in dupes:
    print(f"  Duplicate ID: {row[0]} ({row[1]} projects)")

# Verify all projects have IDs now
print("\n=== All projects ===")
for row in c.execute("SELECT id, name, user_id FROM projects").fetchall():
    print(f"  {row[0][:8]} | {row[1][:40]} | user: {row[2][:8]}")

conn.commit()
conn.close()

# Copy back
sftp.put(local_db, '/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
run("rm -f /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-wal /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-shm")
run("chmod 666 /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db")
sftp.close()

# Start container
print("\n=== Start container ===")
run("docker start moduforge")

print("\nWaiting for healthy...")
for i in range(15):
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
                print(f"  - {p.get('name', 'unnamed')[:40]} (id: {p.get('id', 'N/A')[:8]})")
        else:
            print(f"Response: {out[:500]}")
    except:
        print(f"Raw: {out[:500]}")

    out, err = run(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    try:
        convs = json.loads(out)
        if isinstance(convs, dict) and 'conversations' in convs:
            print(f"\nAI Conversations: {len(convs['conversations'])} found!")
            for cv in convs['conversations'][:5]:
                title = cv.get('title', 'untitled')
                print(f"  - {title[:50]}")
    except:
        pass

    # Notifications
    out, err = run(f'curl -s http://localhost:8086/api/v1/notifications/unread-count -H "Authorization: Bearer {token}"')
    print(f"\nNotifications unread: {out.strip()}")

client.close()
print("\nDone!")
