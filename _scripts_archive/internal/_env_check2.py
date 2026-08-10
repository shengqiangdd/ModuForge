import paramiko
import sys
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

# Check all .env files on the server
cmds = [
    'find /vol1/1000/docker/qwenpaw/data/working/workspaces/default -name ".env*" -exec echo "=== {} ===" \\; -exec cat {} \\; 2>/dev/null',
    'env | grep -iE "rhythm|token_|api_key|base_url|endpoint" 2>/dev/null',
    'cat ~/.bashrc 2>/dev/null | grep -iE "export.*rhythm|export.*token|export.*api" || echo "none in bashrc"',
]

for cmd in cmds:
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode('utf-8', errors='replace')
    if out.strip():
        print(out)

ssh.close()
