import sqlite3
import bcrypt

db = sqlite3.connect('/vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db')
cur = db.cursor()
cur.execute('SELECT username, password_hash FROM users WHERE username = ?', ('admin',))
row = cur.fetchone()
if row:
    print('Username:', row[0])
    print('Hash:', row[1][:50] + '...')
    ok = bcrypt.checkpw(b'admin123', row[1].encode())
    print('Password admin123 matches:', ok)
    for pw in [b'admin', b'csq0216', b'password', b'123456']:
        ok = bcrypt.checkpw(pw, row[1].encode())
        if ok:
            print(f'Password {pw.decode()} matches!')
            break
    else:
        print('No common password matches - need to reset')
else:
    print('Admin user not found')
db.close()
