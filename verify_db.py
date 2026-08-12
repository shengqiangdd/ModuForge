import paramiko, json, os

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Check if container is running
print("=== Container status ===")
out, err = run('docker ps | grep moduforge')
print(out)

# Check the DB file on the server
print("\n=== DB file on server ===")
out, err = run('ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db*')
print(out)

# Copy DB to local again to verify
print("\n=== Copy DB to verify ===")
sftp = client.open_sftp()
local_db = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\moduforge_verify.db'
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db', local_db)
print(f"Copied: {os.path.getsize(local_db)} bytes")

# Check locally
import sqlite3
conn = sqlite3.connect(local_db)
c = conn.cursor()
print(f"\nProjects: {c.execute('SELECT COUNT(*) FROM projects').fetchone()[0]}")
print(f"AI conversations: {c.execute('SELECT COUNT(*) FROM ai_conversations').fetchone()[0]}")
print(f"Messages: {c.execute('SELECT COUNT(*) FROM conversation_messages').fetchone()[0]}")

# Check user_id distribution
print("\n=== Projects user_id distribution ===")
for row in c.execute('SELECT user_id, COUNT(*) FROM projects GROUP BY user_id').fetchall():
    print(f"  user_id={row[0][:8] if row[0] else 'None'}: {row[1]} projects")

conn.close()
sftp.close()

# Check container logs for any errors
print("\n=== Container logs (last 10) ===")
out, err = run('docker logs moduforge --tail 10 2>&1')
# Try to decode as utf-8
try:
    print(out.encode('utf-8', errors='replace').decode('utf-8', errors='replace'))
except:
    print(repr(out[:500]))

client.close()
