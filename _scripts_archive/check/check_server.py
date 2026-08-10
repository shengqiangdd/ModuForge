#!/usr/bin/env python3
"""Check server status and container health"""

import sys
import paramiko

# Fix Windows GBK encoding
if sys.platform == "win32":
    import io
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216')

# Check container status
stdin, stdout, stderr = ssh.exec_command('docker inspect moduforge --format "{{.State.Status}}"')
print('Container status:', stdout.read().decode().strip())

# Check the container's working directory
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge pwd')
print('Container working dir:', stdout.read().decode().strip())

# Check if we can write to the container
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge touch /tmp/test_write && echo "Write OK" || echo "Write FAILED"')
print('Write test:', stdout.read().decode().strip())

# Check container logs for any errors
stdin, stdout, stderr = ssh.exec_command('docker logs moduforge --tail 20')
print('\nContainer logs (last 20 lines):')
print(stdout.read().decode())

ssh.close()