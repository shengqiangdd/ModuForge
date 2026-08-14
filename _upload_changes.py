#!/usr/bin/env python3
import paramiko, sys, hashlib, os
sys.stdout.reconfigure(encoding='utf-8')

LOCAL_ROOT = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge'
REMOTE_ROOT = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge'

FILES = [
    r'frontend\src\lib\components\editor\BuildOutput.svelte',
    r'frontend\src\lib\components\editor\BuildWorkspace.svelte',
    r'frontend\src\main.ts',
    r'frontend\public\sw.js',
    r'backend\cmd\moduforge\main.go',
]

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)
sftp = ssh.open_sftp()

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode('utf-8', errors='replace').strip()

for rel in FILES:
    local = os.path.join(LOCAL_ROOT, rel)
    remote = REMOTE_ROOT + '/' + rel.replace('\\', '/')
    with open(local, 'rb') as f:
        content = f.read()
    sftp.putfo(__import__('io').BytesIO(content), remote)
    # verify md5
    local_md5 = hashlib.md5(content).hexdigest()
    _, o, _ = ssh.exec_command(f'md5sum {remote}')
    remote_md5 = o.read().decode().split()[0]
    print(f'{"OK " if local_md5==remote_md5 else "FAIL"} {rel} ({len(content)}B)')

sftp.close()
ssh.close()
print('UPLOAD DONE')
