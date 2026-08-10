import paramiko
import sys
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

# Check env vars in the container
stdin, stdout, stderr = ssh.exec_command(
    'docker exec moduforge env | grep -iE "rhythm|token|api_key|endpoint|base_url" 2>/dev/null'
)
print('=== Container env ===')
print(stdout.read().decode('utf-8', errors='replace'))

# Check .env file in the persistent volume
stdin, stdout, stderr = ssh.exec_command(
    'docker exec moduforge cat /data/.env 2>/dev/null'
)
print('=== /data/.env ===')
print(stdout.read().decode('utf-8', errors='replace'))

# Check docker-compose env
stdin, stdout, stderr = ssh.exec_command(
    'cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && cat .env 2>/dev/null'
)
print('=== Local .env ===')
print(stdout.read().decode('utf-8', errors='replace'))

# Check all env vars
stdin, stdout, stderr = ssh.exec_command(
    'docker exec moduforge env 2>/dev/null'
)
print('=== All container env ===')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
