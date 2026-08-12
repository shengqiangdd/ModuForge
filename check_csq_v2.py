import paramiko, sqlite3, os, tempfile

# Try password auth
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())

# Try with password
try:
    c.connect('192.168.2.9', 22, 'root', password='root')
    print('Connected with password')
except:
    try:
        c.connect('192.168.2.9', 22, 'admin', password='admin')
        print('Connected as admin')
    except Exception as e:
        print(f'Auth failed: {e}')
        # Try with key and different user
        k = paramiko.Ed25519Key.from_private_key_file(r'C:\Users\22875\.ssh\id_ed25519')
        try:
            c.connect('192.168.2.9', 22, 'admin', pkey=k)
            print('Connected as admin with key')
        except Exception as e2:
            print(f'Key auth also failed: {e2}')
            exit(1)

# Copy DB locally
sftp = c.open_sftp()
local = os.path.join(tempfile.gettempdir(), 'moduforge_check.db')
sftp.get('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db', local)
sftp.close()
print(f'DB copied: {os.path.getsize(local):,} bytes')

conn = sqlite3.connect(local)
cur = conn.cursor()

print('\n=== USERS ===')
for r in cur.execute('SELECT id, username, email FROM users'):
    print(f'  {r}')

print('\n=== csq PROJECTS ===')
rows = cur.execute("SELECT id, user_id, name FROM projects WHERE user_id LIKE '%a4c50d84%'").fetchall()
if rows:
    for r in rows:
        print(f'  {r}')
else:
    print('  (NONE - csq has NO projects!)')

print('\n=== ALL PROJECTS by user_id ===')
for r in cur.execute('SELECT user_id, COUNT(*) FROM projects GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} projects')

print('\n=== csq CONVERSATIONS ===')
rows = cur.execute("SELECT id, user_id, title FROM ai_conversations WHERE user_id LIKE '%a4c50d84%'").fetchall()
if rows:
    for r in rows:
        print(f'  {r}')
else:
    print('  (NONE - csq has NO conversations!)')

print('\n=== ALL CONVERSATIONS by user_id ===')
for r in cur.execute('SELECT user_id, COUNT(*) FROM ai_conversations GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} conversations')

print('\n=== BUILD TASKS by user_id ===')
for r in cur.execute('SELECT user_id, COUNT(*) FROM build_tasks GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} builds')

conn.close()
os.remove(local)
c.close()
