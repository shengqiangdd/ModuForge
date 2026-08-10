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

# Check container env
print('=== CONTAINER ENV ===')
run(SUDO + ' docker exec moduforge env | grep -i "database\|db_path\|DB"')

# Check all db files
print('\n=== ALL DB FILES ===')
run(SUDO + ' docker exec moduforge find / -name "*.db" -ls 2>/dev/null')

# Check working directory
print('\n=== WORKING DIR ===')
run(SUDO + ' docker exec moduforge pwd')

# Check if there's a db at the default relative path
print('\n=== DEFAULT PATH ===')
run(SUDO + ' docker exec moduforge ls -la data/moduforge.db 2>/dev/null || echo "No data/moduforge.db"')

# Check the startup log for DB path
print('\n=== STARTUP LOG (full) ===')
run(SUDO + ' docker logs moduforge 2>&1 | head -30')

ssh.close()
