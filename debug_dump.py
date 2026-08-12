"""Debug: check what's in the dump and DB"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Check dump content
print('=== Dump tables ===')
stdin, stdout, stderr = ssh.exec_command("""
grep "CREATE TABLE" /tmp/moduforge_dump.sql
echo "---"
grep "INSERT" /tmp/moduforge_dump.sql | head -5
echo "---"
# Check if users table exists
grep -c "users" /tmp/moduforge_dump.sql
""")
print(stdout.read().decode(errors='replace'))

# Check DB tables
print('\n=== DB tables ===')
stdin, stdout, stderr = ssh.exec_command("""
python3 -c "
import sqlite3
conn = sqlite3.connect('/vol1/docker/moduforge-data/moduforge.db')
c = conn.cursor()
tables = [r[0] for r in c.execute(\"SELECT name FROM sqlite_master WHERE type='table'\").fetchall()]
print(f'Tables ({len(tables)}):')
for t in tables:
    try:
        cnt = c.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]
        print(f'  {t}: {cnt} rows')
    except Exception as e:
        print(f'  {t}: ERROR {e}')
conn.close()
"
""")
print(stdout.read().decode(errors='replace'))

ssh.close()
