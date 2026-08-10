import paramiko
import sys
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

stdin, stdout, stderr = ssh.exec_command('echo "csq0216" | sudo -S docker logs moduforge --tail 30 2>&1')
print(stdout.read().decode(errors='replace'))

ssh.close()
