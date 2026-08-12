"""Clean up duplicate projects for csq"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Find and remove duplicate projects (keep the ones with original IDs)
fix_script = r'''
import sqlite3
db = "/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db"
conn = sqlite3.connect(db)
c = conn.cursor()

csq = "a4c50d84-a58d-4fbc-a64d-adf93ca14446"

# Get all csq projects
rows = c.execute("SELECT id, name FROM projects WHERE user_id=?", (csq,)).fetchall()
print("All csq projects:")
for r in rows:
    print("  " + r[0][:16] + "... " + r[1])

# Find duplicates by name
from collections import defaultdict
by_name = defaultdict(list)
for r in rows:
    by_name[r[1]].append(r[0])

# Delete duplicates (keep the one with shortest ID or first)
deleted = 0
for name, ids in by_name.items():
    if len(ids) > 1:
        # Keep the first one, delete the rest
        for dup_id in ids[1:]:
            # Also delete related data
            c.execute("DELETE FROM project_files WHERE project_id=?", (dup_id,))
            c.execute("DELETE FROM build_tasks WHERE project_id=?", (dup_id,))
            c.execute("DELETE FROM comments WHERE project_id=?", (dup_id,))
            c.execute("DELETE FROM collaborators WHERE project_id=?", (dup_id,))
            c.execute("DELETE FROM projects WHERE id=?", (dup_id,))
            deleted += 1
            print("  Deleted duplicate: " + name + " (" + dup_id[:16] + "...")

conn.commit()

# Final count
cnt = c.execute("SELECT COUNT(*) FROM projects WHERE user_id=?", (csq,)).fetchone()[0]
print("")
print("Final csq projects: " + str(cnt))
rows = c.execute("SELECT id, name FROM projects WHERE user_id=?", (csq,)).fetchall()
for r in rows:
    print("  " + r[0][:16] + "... " + r[1])

conn.close()
print("Cleanup done!")
'''

sftp = ssh.open_sftp()
with sftp.open('/tmp/cleanup_dupes.py', 'w') as f:
    f.write(fix_script)
sftp.close()

stdin, stdout, stderr = ssh.exec_command('python3 /tmp/cleanup_dupes.py 2>&1')
print(stdout.read().decode(errors='replace'))
err = stderr.read().decode(errors='replace')
if err: print('STDERR:', err)

# Restart
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose restart 2>&1')
print(stdout.read().decode())

ssh.close()
print('Done!')
