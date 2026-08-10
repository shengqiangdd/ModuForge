import paramiko
import time
import sys
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)
print('SSH connected')

# 1. Pull
print('--- git pull ---')
stdin, stdout, stderr = ssh.exec_command(
    'cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && '
    'GIT_CONFIG_GLOBAL=/tmp/gitconfig_safe git -c safe.directory="*" pull origin main'
)
print(stdout.read().decode('utf-8', errors='replace').strip())

# 2. Rebuild
print('--- docker compose up -d --build ---')
stdin, stdout, stderr = ssh.exec_command(
    'cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && '
    'docker compose up -d --build 2>&1',
    timeout=600
)
out = stdout.read().decode('utf-8', errors='replace')
print(out[-2000:] if len(out) > 2000 else out)

# 3. Wait and health check
time.sleep(10)
print('--- health check ---')
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8086/health')
print(stdout.read().decode('utf-8', errors='replace').strip())

print('--- container status ---')
stdin, stdout, stderr = ssh.exec_command('docker ps --filter name=moduforge --format "{{.Status}}"')
print(stdout.read().decode('utf-8', errors='replace').strip())

ssh.close()
print('Done')
