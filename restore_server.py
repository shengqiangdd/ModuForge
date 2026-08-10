#!/usr/bin/env python3
"""Restore the working ModuForge server binary and recompile with CGO."""
import paramiko
import time

HOST = '192.168.2.9'
USER = 'admin'
PASSWORD = 'csq0216'

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PASSWORD, timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    exit_status = stdout.channel.recv_exit_status()
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return exit_status, out, err

# 1. Stop container
print('1. Stopping container...')
run('docker stop moduforge')
time.sleep(2)

# 2. Restore original binary from image layer
print('2. Restoring original binary from image layer...')
rc, out, err = run('cp /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/bin/moduforge-linux /tmp/moduforge-original')
print(f'   rc={rc} out={out} err={err}')

# 3. Get UpperDir
print('3. Getting UpperDir...')
rc, merged_dir, err = run('docker inspect moduforge --format={{.GraphDriver.Data.MergedDir}}')
upper_dir = merged_dir.replace('/merged', '/diff')
print(f'   UpperDir: {upper_dir}')

# 4. Copy to UpperDir
print('4. Copying original binary to UpperDir...')
rc, out, err = run(f'cp /tmp/moduforge-original {upper_dir}/server')
print(f'   cp: rc={rc} err={err}')
rc, out, err = run(f'chmod +x {upper_dir}/server')
print(f'   chmod: rc={rc} err={err}')
rc, out, err = run(f'chown 1000:1000 {upper_dir}/server')
print(f'   chown: rc={rc} err={err}')

# 5. Verify
print('5. Verifying...')
rc, out, err = run(f'ls -la {upper_dir}/server')
print(f'   {out}')

# 6. Start container
print('6. Starting container...')
run('docker start moduforge')
time.sleep(5)

# 7. Check status
print('7. Checking status...')
rc, status, err = run('docker inspect moduforge --format={{.State.Status}}')
print(f'   Status: {status}')

# 8. Check health
print('8. Checking health...')
for i in range(6):
    rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
    print(f'   Health: {health}')
    if health == 'healthy':
        break
    time.sleep(5)

# 9. Test API
print('9. Testing API...')
rc, health_check, err = run('curl -s http://localhost:8086/health')
print(f'   {health_check}')

print('\nDone!')
ssh.close()
