#!/usr/bin/env python3
import paramiko, sys
sys.stdout.reconfigure(encoding='utf-8')
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=1500):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode('utf-8', errors='replace').strip()
    err = e.read().decode('utf-8', errors='replace').strip()
    return out if out else ('ERR: ' + err if err else '')

WORKDIR = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'

print('=== docker compose build (long) ===')
print(run(f'cd {WORKDIR} && docker compose build moduforge 2>&1 | tail -30'))

print('\n=== docker compose up -d ===')
print(run(f'cd {WORKDIR} && docker compose up -d 2>&1 | tail -10'))

print('\n=== status ===')
print(run("docker ps --filter name=moduforge --format '{{.Status}}'"))
print(run("docker inspect moduforge --format 'Created: {{.Created}}'"))
print(run("docker logs --tail 8 moduforge 2>&1 | tail -8"))

ssh.close()
print('BUILD+DEPLOY DONE')
