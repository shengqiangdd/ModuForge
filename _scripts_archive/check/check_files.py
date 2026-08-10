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

# Check volume for actual files
print('=== VOLUME FILES ===')
run(SUDO + ' docker exec moduforge ls -la /data/projects/ 2>/dev/null || echo "No /data/projects"')

# Check if there are project directories
print('\n=== PROJECT DIRS ===')
run(SUDO + ' docker exec moduforge find /data -name "*.rs" -o -name "*.go" -o -name "*.cpp" -o -name "*.h" 2>/dev/null | head -20')

# Check the storage directory
print('\n=== STORAGE ===')
run(SUDO + ' docker exec moduforge ls -la /data/storage/ 2>/dev/null')

# Check uploads
print('\n=== UPLOADS ===')
run(SUDO + ' docker exec moduforge ls -la /app/uploads/ 2>/dev/null')

# The actual project files might be in a different location
print('\n=== FIND ANDROBOOST ===')
run(SUDO + ' docker exec moduforge find / -name "*AndroBoost*" -o -name "*androboost*" 2>/dev/null | head -10')

# Check the local source code on the host
print('\n=== LOCAL SOURCE ===')
PROJ = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'
run(f'ls -la {PROJ}/data/storage/projects/ 2>/dev/null | head -10')

# Check if the projects are in the git repo
print('\n=== GIT PROJECTS ===')
run(f'ls -la {PROJ}/projects/ 2>/dev/null | head -10')

ssh.close()
