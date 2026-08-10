import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode(errors='replace')
    err = stderr.read().decode(errors='replace')
    if out: print(out)
    if err: print(err)
    return out

SUDO = 'echo "csq0216" | sudo -S'

# Check container's actual DB
print('=== CONTAINER DB USERS ===')
script = r'''
import sqlite3
db = sqlite3.connect("/data/moduforge.db")
cur = db.cursor()
cur.execute("SELECT username, password_hash FROM users")
for row in cur.fetchall():
    print(row[0], row[1][:40] + "...")
db.close()
'''
run(f'cat > /tmp/check.py << \'EOF\'\n{script}\nEOF')

# Install python3 in container first
run(SUDO + ' docker exec moduforge sh -c "apk add --no-cache python3 2>&1 || true"')
run(SUDO + ' docker cp /tmp/check.py moduforge:/tmp/check.py')
run(SUDO + ' docker exec moduforge python3 /tmp/check.py')

# Also check if the Go binary is using a different path
print('\n=== BINARY CHECK ===')
run(SUDO + ' docker exec moduforge ls -la /app/moduforge 2>/dev/null || echo "No /app/moduforge"')

# Check the actual env var in the running process
print('\n=== RUNNING PROCESS ENV ===')
run(SUDO + ' docker exec moduforge sh -c "cat /proc/1/environ | tr \"\\0\" \"\\n\" | grep -i database"')

# Check if there's a config file
print('\n=== CONFIG FILE ===')
run(SUDO + ' docker exec moduforge ls -la /app/config* 2>/dev/null || echo "No config"')
run(SUDO + ' docker exec moduforge ls -la /app/*.yaml /app/*.yml /app/*.json /app/*.toml 2>/dev/null || echo "No config files"')

ssh.close()
