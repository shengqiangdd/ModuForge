"""Try bind mount approach - copy DB to local dir, mount as bind"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Stop container
print('=== Stop ===')
stdin, stdout, stderr = ssh.exec_command('cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && sudo docker compose down 2>&1')
print(stdout.read().decode())

# Check docker-compose.yml to understand the volume setup
print('\n=== docker-compose.yml ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml 2>&1')
print(stdout.read().decode(errors='replace'))

# Check if there's a way to run the container as root
print('\n=== Check Dockerfile user ===')
stdin, stdout, stderr = ssh.exec_command('grep -i "user\\|USER" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/Dockerfile 2>&1')
print(stdout.read().decode(errors='replace'))

# Check the entrypoint
print('\n=== entrypoint ===')
stdin, stdout, stderr = ssh.exec_command('cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-entrypoint.sh 2>&1')
print(stdout.read().decode(errors='replace'))

ssh.close()
