import sqlite3

conn = sqlite3.connect('moduforge.db.bak')
c = conn.cursor()

print("=== BACKUP DB ===")
for t in ['users', 'projects', 'messages', 'conversation_messages', 'ai_conversations', 'build_tasks']:
    try:
        count = c.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]
        print(f'{t}: {count} rows')
        if count > 0 and count < 20:
            rows = c.execute(f'SELECT * FROM {t} LIMIT 2').fetchall()
            cols = [d[0] for d in c.description]
            print(f'  Cols: {cols}')
            for r in rows:
                print(f'  {r}')
    except Exception as e:
        print(f'{t}: {e}')
conn.close()

print()
conn2 = sqlite3.connect('moduforge.db')
c2 = conn2.cursor()
print("=== CURRENT DB ===")
for t in ['users', 'projects', 'messages', 'conversation_messages', 'ai_conversations', 'build_tasks']:
    try:
        count = c2.execute(f'SELECT COUNT(*) FROM {t}').fetchone()[0]
        print(f'{t}: {count} rows')
        if count > 0 and count < 20:
            rows = c2.execute(f'SELECT * FROM {t} LIMIT 2').fetchall()
            cols = [d[0] for d in c2.description]
            print(f'  Cols: {cols}')
            for r in rows:
                print(f'  {r}')
    except Exception as e:
        print(f'{t}: {e}')
conn2.close()
