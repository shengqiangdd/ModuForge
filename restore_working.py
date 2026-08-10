import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

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

# 2. Find the working binary (17MB+, compiled with CGO_ENABLED=1)
print('2. Finding working binary (17MB+)...')
rc, out, err = run('find /vol1/docker/overlay2 -name "server" -size +16M -type f 2>/dev/null | head -5')
print(f'   Found: {out}')

# Use the latest one (17741368 bytes from Aug 9 10:27)
# Actually, let's find the one that's around 17.8MB (17864280 bytes)
rc, out, err = run('find /vol1/docker/overlay2 -name "server" -size +17M -type f 2>/dev/null | head -5')
print(f'   Found 17M+: {out}')

# 3. Copy the working binary
print('3. Copying working binary...')
# Use the largest one we can find
rc, out, err = run('cp /vol1/docker/overlay2/p77oe1sdfai03blepphcn3fgm/diff/server /tmp/moduforge-working')
print(f'   cp: rc={rc} err={err}')

# 4. Get UpperDir
print('4. Getting UpperDir...')
rc, merged_dir, err = run('docker inspect moduforge --format={{.GraphDriver.Data.MergedDir}}')
upper_dir = merged_dir.replace('/merged', '/diff')
print(f'   UpperDir: {upper_dir}')

# 5. Copy to UpperDir
print('5. Copying working binary to UpperDir...')
rc, out, err = run(f'cp /tmp/moduforge-working {upper_dir}/server')
print(f'   cp: rc={rc} err={err}')
rc, out, err = run(f'chmod +x {upper_dir}/server')
print(f'   chmod: rc={rc} err={err}')
rc, out, err = run(f'chown 1000:1000 {upper_dir}/server')
print(f'   chown: rc={rc} err={err}')

# 6. Verify
print('6. Verifying...')
rc, out, err = run(f'ls -la {upper_dir}/server')
print(f'   {out}')

# 7. Check binary type
print('7. Checking binary type...')
rc, out, err = run(f'file {upper_dir}/server')
print(f'   {out}')

# 8. Start container
print('8. Starting container...')
run('docker start moduforge')
time.sleep(5)

# 9. Check status
print('9. Checking status...')
rc, status, err = run('docker inspect moduforge --format={{.State.Status}}')
print(f'   Status: {status}')

# 10. Check health
print('10. Checking health...')
for i in range(6):
    rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
    print(f'   Health: {health}')
    if health == 'healthy':
        break
    time.sleep(5)

# 11. Test API
print('11. Testing API...')
rc, health_check, err = run('curl -s http://localhost:8086/health')
print(f'   {health_check}')

print('\nDone!')
ssh.close()
