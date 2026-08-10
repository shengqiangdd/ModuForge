import paramiko
import sys
sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

stdin, stdout, stderr = ssh.exec_command('docker logs moduforge 2>&1 | tail -50')
print(stdout.read().decode('utf-8', errors='replace'))

ssh.close()
