import paramiko
import time
import sys

# Fix Windows GBK encoding for Unicode output
sys.stdout.reconfigure(encoding='utf-8', errors='replace')
sys.stderr.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)
print('SSH connected')

# 1. Pull already done, verify
print('--- git log ---')
stdin, stdout, stderr = ssh.exec_command(
    'cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && git log --oneline -1'
)
print(stdout.read().decode('utf-8', errors='replace').strip())

# 2. Rebuild and restart
print('--- docker compose up -d --build ---')
stdin, stdout, stderr = ssh.exec_command(
    'cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && '
    'docker compose up -d --build 2>&1',
    timeout=600
)
out = stdout.read().decode('utf-8', errors='replace')
# Print last 3000 chars
print(out[-3000:] if len(out) > 3000 else out)

# 3. Health check
time.sleep(8)
print('--- health check ---')
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
print(stdout.read().decode('utf-8', errors='replace').strip())

# 4. Check container status
print('--- container status ---')
stdin, stdout, stderr = ssh.exec_command('docker ps --filter name=moduforge --format "{{.Status}} | {{.Image}}"')
print(stdout.read().decode('utf-8', errors='replace').strip())

ssh.close()
print('Done')
