import paramiko
import sys
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Check actual Dockerfile on server
stdin, stdout, stderr = ssh.exec_command('cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/Dockerfile')
print('=== ACTUAL DOCKERFILE ON SERVER ===')
print(stdout.read().decode(errors='replace'))

# Check git status
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && git diff backend/Dockerfile 2>/dev/null')
print('=== GIT DIFF ===')
print(stdout.read().decode(errors='replace'))

ssh.close()
