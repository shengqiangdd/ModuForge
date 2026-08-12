"""Check if data was imported"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Check dump file content
print('=== Dump file analysis ===')
stdin, stdout, stderr = ssh.exec_command("""
wc -l /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
echo "---"
# Count INSERT statements
grep -c "INSERT" /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
echo "---"
# Check tables
grep "CREATE TABLE" /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql | head -20
echo "---"
# Check if COMMIT exists
grep "COMMIT" /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
echo "---"
# Last 5 lines
tail -5 /vol1/docker/volumes/moduforge_data/_data/moduforge.db.dump.sql
""")
print(stdout.read().decode(errors='replace'))

# Check the actual DB
print('\n=== Check DB directly ===')
cmd = """
docker run --rm \
  -v /vol1/docker/moduforge-data:/data:ro \
  alpine sh -c '
    apk add --no-cache sqlite 2>/dev/null
    echo "Tables:"
    sqlite3 /data/moduforge.db ".tables"
    echo "---"
    echo "Users:"
    sqlite3 /data/moduforge.db "SELECT id, username FROM users;"
    echo "---"
    echo "Projects:"
    sqlite3 /data/moduforge.db "SELECT id, name FROM projects;"
    echo "---"
    echo "DB size:"
    ls -la /data/moduforge.db
  '
"""
stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
print(stdout.read().decode(errors='replace'))

ssh.close()
