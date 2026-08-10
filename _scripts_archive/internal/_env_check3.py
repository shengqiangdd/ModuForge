import paramiko
import sys
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

# Broader search for any rhythm/token related env vars
cmds = [
    'grep -ri "rhythm" /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/ --include="*.env*" --include="*.yml" --include="*.yaml" --include="*.json" -l 2>/dev/null',
    'grep -ri "tokenrhythm" /vol1/1000/docker/qwenpaw/data/working/workspaces/ --include="*.env*" -l 2>/dev/null',
    'env | grep -i rhythm 2>/dev/null || echo "no rhythm env on host"',
    'printenv | grep -iE "RHYTHM|TOKEN_R" 2>/dev/null || echo "none"',
]

for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace').strip()
    if out:
        print(f'--- {cmd[:50]} ---')
        print(out)

ssh.close()
