import paramiko, json, time

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
local_db = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge_clean.db'
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db', local_db)
print("Copied to local")

# Clean duplicates
import sqlite3
conn = sqlite3.connect(local_db)
c = conn.cursor()

# Remove duplicate "Agent Workspace" projects (keep only 1)
dupes = c.execute("SELECT id, name FROM projects WHERE name = 'Agent Workspace'").fetchall()
print(f"\nAgent Workspace duplicates: {len(dupes)}")
if len(dupes) > 1:
    # Keep the first one, delete the rest
    keep_id = dupes[0][0]
    for row in dupes[1:]:
        c.execute("DELETE FROM projects WHERE id = ? AND name = 'Agent Workspace' AND rowid != (SELECT MIN(rowid) FROM projects WHERE id = ? AND name = 'Agent Workspace')", (row[0], row[0]))
    print(f"  Kept: {keep_id}, removed {len(dupes)-1} duplicates")

# Remove duplicate "My First Module"
dupes = c.execute("SELECT id, name FROM projects WHERE name = 'My First Module'").fetchall()
print(f"\nMy First Module duplicates: {len(dupes)}")
if len(dupes) > 1:
    # Delete the one with NULL/short ID
    c.execute("DELETE FROM projects WHERE name = 'My First Module' AND id = 'd2395c29'")
    print("  Removed the duplicate with id=d2395c29")

# Remove duplicate "AI Generated Module" (keep the one with longer ID)
dupes = c.execute("SELECT id, name FROM projects WHERE name = 'AI Generated Module'").fetchall()
print(f"\nAI Generated Module duplicates: {len(dupes)}")
if len(dupes) > 1:
    # Remove the one with shorter ID
    c.execute("DELETE FROM projects WHERE name = 'AI Generated Module' AND id = '17852499'")
    print("  Removed the duplicate with id=17852499")

conn.commit()

# Verify clean state
print("\n=== Final project list ===")
projects = c.execute("SELECT id, name FROM projects ORDER BY name").fetchall()
for p in projects:
    print(f"  {p[0][:8]} | {p[1][:40]}")

print(f"\nTotal: {len(projects)} projects")
print(f"AI conversations: {c.execute('SELECT COUNT(*) FROM ai_conversations').fetchone()[0]}")
print(f"Messages: {c.execute('SELECT COUNT(*) FROM conversation_messages').fetchone()[0]}")
print(f"Build tasks: {c.execute('SELECT COUNT(*) FROM build_tasks').fetchone()[0]}")

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

# Final API test
print("\n=== Final API Test ===")
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
                print(f"  - {p.get('name', 'unnamed')[:40]}")
        else:
            print(f"Response: {out[:500]}")
    except:
        print(f"Raw: {out[:500]}")

    out, err = run(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    try:
        convs = json.loads(out)
        if isinstance(convs, dict) and 'conversations' in convs:
            print(f"\nAI Conversations: {len(convs['conversations'])} found!")
    except:
        pass

# Check frontend
print("\n=== Frontend check ===")
out, err = run("curl -s -o /dev/null -w '%{http_code}' http://localhost:8086/")
print(f"Frontend HTTP: {out.strip()}")

client.close()
print("\nDone!")
