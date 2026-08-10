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
PROJ = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'

# 1. Find old database files
print('=== OLD DATABASE ===')
run(f'ls -la {PROJ}/data/moduforge.db*')

# 2. Get volume mountpoint
print('\n=== VOLUME MOUNTPOINT ===')
out = run(f'{SUDO} docker volume inspect moduforge_moduforge_data --format {{{{.Mountpoint}}}}')
MOUNTPOINT = out.strip()
print(f'Mountpoint: {MOUNTPOINT}')

# 3. Copy old DB to new volume
print('\n=== COPY DATABASE ===')
# Copy all db files (db, db-shm, db-wal)
run(f'{SUDO} cp {PROJ}/data/moduforge.db {MOUNTPOINT}/moduforge.db')
run(f'{SUDO} cp {PROJ}/data/moduforge.db-shm {MOUNTPOINT}/moduforge.db-shm 2>/dev/null; true')
run(f'{SUDO} cp {PROJ}/data/moduforge.db-wal {MOUNTPOINT}/moduforge.db-wal 2>/dev/null; true')
run(f'{SUDO} chmod 644 {MOUNTPOINT}/moduforge.db*')

# 4. Also copy storage directory if it exists (projects, agents, etc.)
print('\n=== COPY STORAGE ===')
run(f'{SUDO} ls -la {PROJ}/data/storage/ 2>/dev/null')
run(f'{SUDO} cp -r {PROJ}/data/storage/* {MOUNTPOINT}/ 2>/dev/null; true')
run(f'{SUDO} cp -r {PROJ}/data/sessions/* {MOUNTPOINT}/sessions/ 2>/dev/null; true')
run(f'{SUDO} mkdir -p {MOUNTPOINT}/sessions')
run(f'{SUDO} cp -r {PROJ}/data/sessions/* {MOUNTPOINT}/sessions/ 2>/dev/null; true')

# 5. Verify
print('\n=== VERIFY ===')
run(f'ls -la {MOUNTPOINT}/')

# 6. Restart container to pick up the data
print('\n=== RESTART ===')
run(f'{SUDO} docker restart moduforge')

import time
time.sleep(5)

# 7. Check logs
print('\n=== LOGS ===')
run(f'{SUDO} docker logs moduforge --tail 15 2>&1')

ssh.close()
print('\n=== DATA RECOVERY COMPLETE ===')
