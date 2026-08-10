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

# Check all DB files in the container
print('=== DB FILES IN CONTAINER ===')
run(SUDO + ' docker exec moduforge find / -name "*.db" -o -name "*.sqlite" 2>/dev/null')

# Check if there's a default DB_PATH in the code
print('\n=== CHECK DEFAULT DB PATH ===')
run('grep -n "DB_PATH\|dbPath\|db_path\|moduforge.db" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/config/config.go 2>/dev/null')

# Check what the binary actually uses
print('\n=== CHECK MAIN.GO ===')
run('grep -n "DB_PATH\|dbPath\|db_path\|moduforge.db" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/cmd/moduforge/main.go 2>/dev/null')

# Check if the container has a different DB at a default path
print('\n=== CHECK DEFAULT PATH ===')
run(SUDO + ' docker exec moduforge ls -la /app/moduforge.db 2>/dev/null || echo "No /app/moduforge.db"')
run(SUDO + ' docker exec moduforge ls -la /moduforge.db 2>/dev/null || echo "No /moduforge.db"')

# Check the actual startup log for DB path
print('\n=== STARTUP LOG ===')
run(SUDO + ' docker logs moduforge 2>&1 | head -20')

ssh.close()
