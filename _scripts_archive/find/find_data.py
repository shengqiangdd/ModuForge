import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
    return stdout.read().decode(errors='replace'), stderr.read().decode(errors='replace')

SUDO = 'echo "csq0216" | sudo -S'

# 1. List all docker volumes
print('=== ALL DOCKER VOLUMES ===')
out, _ = run(f'{SUDO} docker volume ls')
print(out)

# 2. Check for old moduforge volume data
print('\n=== CHECK OLD VOLUME DATA ===')
out, _ = run(f'{SUDO} docker volume inspect moduforge_moduforge_data 2>/dev/null || echo "NOT FOUND"')
print(out)

# 3. List all containers (including stopped)
print('\n=== ALL CONTAINERS ===')
out, _ = run(f'{SUDO} docker ps -a --format "table {{{{.Names}}}}\\t{{{{.Status}}}}\\t{{{{.Image}}}}"')
print(out)

# 4. Check if old data volume exists as anonymous
print('\n=== VOLUME MOUNTS ===')
out, _ = run(f'{SUDO} docker inspect moduforge --format {{{{range .Mounts}}}}{{{{.Name}}}} -> {{{{.Destination}}}} ({{{{.Source}}}})\\n{{{{end}}}}')
print(out)

# 5. Look for old database file on host
print('\n=== HOST FILESYSTEM SEARCH ===')
out, _ = run(f'{SUDO} find /var/lib/docker/volumes -name "moduforge.db" 2>/dev/null | head -5')
print(out if out.strip() else 'No old DB files found in Docker volumes')

# Also check the old path directly
out, _ = run(f'{SUDO} ls -la /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/data/ 2>/dev/null || echo "No local data dir"')
print(f'\n=== LOCAL DATA DIR ===\n{out}')

ssh.close()
