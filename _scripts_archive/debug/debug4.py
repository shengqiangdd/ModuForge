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

# Install sqlite3 in container as root
print('=== INSTALL SQLITE3 ===')
run(SUDO + ' docker exec -u root moduforge sh -c "apk add --no-cache sqlite 2>&1"')

# Now check users
print('\n=== CONTAINER DB USERS ===')
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT username, password_hash FROM users;"')

# Check the actual binary location
print('\n=== BINARY LOCATION ===')
run(SUDO + ' docker exec moduforge ls -la /app/ 2>/dev/null | head -10')

# Find the actual binary
print('\n=== FIND BINARY ===')
run(SUDO + ' docker exec moduforge find / -name "moduforge" -type f 2>/dev/null')

# Check the entrypoint
print('\n=== ENTRYPOINT ===')
run(SUDO + ' docker inspect moduforge --format "{{.Config.Entrypoint}} {{.Config.Cmd}}"')

# Check if the binary reads DATABASE_PATH or something else
print('\n=== ENV VARS ===')
run(SUDO + ' docker exec moduforge sh -c "cat /proc/1/environ | tr \"\\0\" \"\\n\""')

ssh.close()
