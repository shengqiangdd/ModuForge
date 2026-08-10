import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

stdin, stdout, stderr = ssh.exec_command('docker logs --tail 30 moduforge 2>&1')
print(stdout.read().decode())

# Also check if the binary is actually running
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ls -la /server 2>&1')
print("Binary in container:", stdout.read().decode())

ssh.close()
