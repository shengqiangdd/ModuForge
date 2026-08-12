import paramiko, json, time

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Stop
run("docker stop moduforge")
time.sleep(2)

# Copy
sftp = client.open_sftp()
local_db = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge_final.db'
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db', local_db)

import sqlite3
conn = sqlite3.connect(local_db)
c = conn.cursor()

# Get all projects grouped by name
groups = {}
for row in c.execute("SELECT rowid, id, name FROM projects ORDER BY rowid").fetchall():
    name = row[2]
    if name not in groups:
        groups[name] = []
    groups[name].append(row[0])  # rowid

# For each group with duplicates, keep only the first
total_deleted = 0
for name, rowids in groups.items():
    if len(rowids) > 1:
        # Keep the first, delete the rest
        keep = rowids[0]
        delete = rowids[1:]
        placeholders = ','.join('?' * len(delete))
        c.execute(f"DELETE FROM projects WHERE rowid IN ({placeholders})", delete)
        total_deleted += len(delete)
        print(f"  '{name[:30]}': kept rowid {keep}, deleted {len(delete)} duplicates")

print(f"\nTotal deleted: {total_deleted}")

# Verify
projects = c.execute("SELECT id, name FROM projects ORDER BY name").fetchall()
print(f"\n=== Final project list ({len(projects)} projects) ===")
for p in projects:
    print(f"  {p[0][:20]:20s} | {p[1][:40]}")

conn.commit()
conn.close()

# Copy back
sftp.put(local_db, '/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
run("rm -f /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-wal /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db-shm")
run("chmod 666 /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db")
sftp.close()

# Start
run("docker start moduforge")
print("\nWaiting for healthy...")
for i in range(15):
    time.sleep(3)
    out, err = run("docker exec moduforge wget -q -O /dev/null http://localhost:8086/health 2>&1 && echo 'healthy' || echo 'not ready'")
    if 'healthy' in out:
        print(f"Healthy after {(i+1)*3}s!")
        break

# Final test
print("\n=== Final Test ===")
out, err = run("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(out)
token = login.get('token', '')

if token:
    out, err = run(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    projects = json.loads(out)
    if isinstance(projects, list):
        print(f"Projects: {len(projects)}")
        for p in projects:
            print(f"  - {p.get('name', '?')[:40]}")

    out, err = run(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    convs = json.loads(out)
    if isinstance(convs, dict):
        print(f"AI Conversations: {len(convs.get('conversations', []))}")

    out, err = run(f'curl -s http://localhost:8086/api/v1/notifications/unread-count -H "Authorization: Bearer {token}"')
    print(f"Notifications: {out.strip()}")

print(f"\nFrontend: http://192.168.2.9:8086")
client.close()
print("Done!")
