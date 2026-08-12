"""Check dump file and DB"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Check dump file
print('=== Dump file ===')
stdin, stdout, stderr = ssh.exec_command("""
wc -l /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
grep -c INSERT /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
grep "CREATE TABLE" /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql | head -10
grep COMMIT /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
""")
out = stdout.read().decode(errors='replace')
print(out)

# Check DB
print('=== DB check ===')
cmd = """
docker run --rm \
  -v /vol1/docker/moduforge-data:/data:ro \
  alpine sh -c '
    apk add --no-cache sqlite 2>/dev/null
    echo "Tables:"
    sqlite3 /data/moduforge.db ".tables"
    echo "Users:"
    sqlite3 /data/moduforge.db "SELECT id, username FROM users;"
    echo "Projects:"
    sqlite3 /data/moduforge.db "SELECT id, name FROM projects;"
    echo "Size:"
    ls -la /data/moduforge.db
  '
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
print(stdout.read().decode(errors='replace'))

ssh.close()
