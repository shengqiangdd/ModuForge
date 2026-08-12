import sqlite3

conn = sqlite3.connect('moduforge.db.bak')
c = conn.cursor()

# Check all users
print("=== ALL USERS ===")
users = c.execute('SELECT id, username, email FROM users').fetchall()
for u in users:
    print(f"  {u}")

# Check which user owns which projects
print("\n=== PROJECTS BY USER ===")
projects = c.execute('SELECT user_id, name, id FROM projects').fetchall()
for p in projects:
    print(f"  user={p[0]}, project={p[1]}, id={p[2][:8] if p[2] else 'None'}")

# Check conversation owners
print("\n=== AI CONVERSATIONS BY USER ===")
convs = c.execute('SELECT user_id, title, mode FROM ai_conversations').fetchall()
for cv in convs:
    print(f"  user={cv[0]}, title={cv[1][:40] if cv[1] else ''}, mode={cv[2]}")

conn.close()
