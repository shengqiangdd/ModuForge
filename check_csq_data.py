"""Check csq user and all data in current DB"""
import docker, sqlite3, tempfile, os

c = docker.from_env()
ct = c.containers.get('moduforge')

# Find DB path
data_vol = None
for m in ct.attrs['Mounts']:
    if m['Destination'] == '/data':
        data_vol = m['Source']
        break

db_path = os.path.join(data_vol, 'moduforge.db')
print(f'DB: {db_path} ({os.path.getsize(db_path):,} bytes)')

conn = sqlite3.connect(db_path)
c2 = conn.cursor()

# All users
print('\n=== USERS ===')
for r in c2.execute('SELECT id, username, email FROM users'):
    print(f'  {r}')

# csq user projects
print('\n=== csq PROJECTS (user_id=a4c50d84) ===')
for r in c2.execute("SELECT id, user_id, name FROM projects WHERE user_id='a4c50d84-a58d-4fbc-a64d-adf93ca14446'"):
    print(f'  {r}')

# All projects grouped by user
print('\n=== ALL PROJECTS by user_id ===')
for r in c2.execute('SELECT user_id, COUNT(*) as cnt FROM projects GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} projects')

# Check if csq conversations exist
print('\n=== csq AI CONVERSATIONS ===')
for r in c2.execute("SELECT id, user_id, title FROM ai_conversations WHERE user_id='a4c50d84-a58d-4fbc-a64d-adf93ca14446'"):
    print(f'  {r}')

# All conversations by user
print('\n=== ALL AI CONVERSATIONS by user_id ===')
for r in c2.execute('SELECT user_id, COUNT(*) as cnt FROM ai_conversations GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} conversations')

# Check conversation_messages ownership
print('\n=== CONVERSATION MESSAGES by user_id (via conversation) ===')
for r in c2.execute('''
    SELECT ai.user_id, COUNT(*) as cnt 
    FROM conversation_messages cm 
    JOIN ai_conversations ai ON cm.conversation_id = ai.id 
    GROUP BY ai.user_id
'''):
    print(f'  user={r[0]}: {r[1]} messages')

# Build tasks
print('\n=== BUILD TASKS by user_id ===')
for r in c2.execute('SELECT user_id, COUNT(*) as cnt FROM build_tasks GROUP BY user_id'):
    print(f'  user={r[0]}: {r[1]} builds')

conn.close()
