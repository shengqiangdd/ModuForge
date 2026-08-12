import paramiko
import sys
import time

def deploy():
    c = paramiko.SSHClient()
    c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)
    print('Connected!')

    base = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'

    # Step 1: docker compose down
    print('\n=== Step 1: docker compose down ===')
    _, o, e = c.exec_command(f'cd {base} && docker compose down 2>&1', timeout=60)
    out = o.read().decode()
    print(out)

    # Step 2: docker compose up -d --build
    print('\n=== Step 2: docker compose up -d --build ===')
    _, o2, e2 = c.exec_command(f'cd {base} && docker compose up -d --build 2>&1', timeout=300)
    # Read output line by line
    for line in iter(o2.readline, ""):
        print(line, end='')
    err = e2.read().decode()
    if err:
        print('STDERR:', err)

    # Step 3: Check status
    print('\n=== Step 3: Check status ===')
    time.sleep(5)
    _, o3, _ = c.exec_command(f'cd {base} && docker compose ps 2>&1', timeout=30)
    print(o3.read().decode())

    c.close()
    print('\n=== Deploy complete! ===')

if __name__ == '__main__':
    deploy()
