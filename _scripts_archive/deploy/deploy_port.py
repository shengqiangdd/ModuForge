import paramiko
import time
import sys

sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=300):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode(errors='replace')
    err = stderr.read().decode(errors='replace')
    if out: print(out)
    if err: print(err)
    return out

PROJ = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'
SUDO = 'echo "csq0216" | sudo -S'

# 1. Fix docker-compose.yml on server (PORT=:8080)
print('=== FIX PORT ===')
run(f'{SUDO} sed -i "s/PORT=8080/PORT=:8080/" {PROJ}/docker-compose.yml')
stdin, stdout, stderr = ssh.exec_command(f'grep PORT {PROJ}/docker-compose.yml')
print(stdout.read().decode(errors='replace'))

# 2. Stop
print('=== STOP ===')
run(f'{SUDO} docker stop moduforge 2>/dev/null; {SUDO} docker rm moduforge 2>/dev/null')

# 3. No rebuild needed - just restart with fixed config
print('=== DOCKER UP ===')
run(f'cd {PROJ} && {SUDO} docker compose up -d --force-recreate --remove-orphans')

time.sleep(5)
print('\n=== STATUS ===')
run(f'{SUDO} docker ps --format "table {{{{.Names}}}}\\t{{{{.Status}}}}\\t{{{{.Ports}}}}"')

time.sleep(5)
print('\n=== LOGS ===')
run(f'{SUDO} docker logs moduforge --tail 20 2>&1')

ssh.close()
print('\n=== DONE ===')
