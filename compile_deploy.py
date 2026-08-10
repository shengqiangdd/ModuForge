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

# 1. Check if container is healthy
print('1. Checking container status...')
rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
print(f'   Health: {health}')

if health != 'healthy':
    print('   Container not healthy, waiting...')
    time.sleep(10)

# 2. Check if source code exists in container
print('2. Checking for source code...')
rc, out, err = run('docker exec moduforge ls -la /app/backend 2>&1')
print(f'   /app/backend: {out}')

# 3. Check if gcc is available
print('3. Checking gcc availability...')
rc, out, err = run('docker exec moduforge which gcc 2>&1')
print(f'   gcc: {out}')

# 4. Check if source code is in the overlay2 layer
print('4. Checking overlay2 source code...')
rc, out, err = run('ls -la /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/service/zipper.go 2>&1')
print(f'   zipper.go: {out}')

# 5. Copy source code to container
print('5. Copying source code to container...')
# Create tarball of backend source
rc, out, err = run('cd /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend && tar cf /tmp/backend-src.tar . 2>&1')
print(f'   tar: rc={rc} err={err}')

# Copy tarball to container
rc, out, err = run('docker cp /tmp/backend-src.tar moduforge:/tmp/backend-src.tar')
print(f'   docker cp: rc={rc} err={err}')

# Extract in container
rc, out, err = run('docker exec moduforge sh -c "mkdir -p /tmp/build && cd /tmp/build && tar xf /tmp/backend-src.tar"')
print(f'   extract: rc={rc} err={err}')

# 6. Copy fixed zipper.go
print('6. Copying fixed zipper.go...')
# The fixed zipper.go should be on the server at /tmp/zipper.go or similar
# Let me check if we have it locally
import os
local_zipper = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go'
if os.path.exists(local_zipper):
    print(f'   Found local zipper.go: {local_zipper}')
    # Upload to server
    sftp = ssh.open_sftp()
    sftp.put(local_zipper, '/tmp/zipper_fixed.go')
    sftp.close()
    # Copy to container
    rc, out, err = run('docker cp /tmp/zipper_fixed.go moduforge:/tmp/build/internal/service/zipper.go')
    print(f'   docker cp: rc={rc} err={err}')
else:
    print(f'   Local zipper.go not found, using the one from overlay2')
    # The one from overlay2 should already have our fixes since we compiled it earlier
    # But let me verify by checking the content
    rc, out, err = run('grep -c "tmp/" /vol1/docker/overlay2/715zaietsobb3cj3kgqe9q2y9/diff/backend/internal/service/zipper.go')
    print(f'   Has tmp/ exclusion: {out}')

# 7. Compile with CGO_ENABLED=1
print('7. Compiling with CGO_ENABLED=1...')
rc, out, err = run('docker exec moduforge sh -c "cd /tmp/build && CGO_ENABLED=1 go build -o /tmp/server ./cmd/moduforge 2>&1"')
print(f'   compile: rc={rc}')
if err:
    print(f'   stderr: {err[:500]}')
if out:
    print(f'   stdout: {out[:500]}')

# 8. Check compiled binary
print('8. Checking compiled binary...')
rc, out, err = run('docker exec moduforge ls -la /tmp/server 2>&1')
print(f'   {out}')

# 9. Deploy new binary
print('9. Deploying new binary...')
# Stop container
run('docker stop moduforge')
time.sleep(2)

# Get UpperDir
rc, merged_dir, err = run('docker inspect moduforge --format={{.GraphDriver.Data.MergedDir}}')
upper_dir = merged_dir.replace('/merged', '/diff')
print(f'   UpperDir: {upper_dir}')

# Copy new binary to UpperDir
rc, out, err = run(f'docker cp moduforge:/tmp/server {upper_dir}/server')
print(f'   docker cp: rc={rc} err={err}')
rc, out, err = run(f'chmod +x {upper_dir}/server')
print(f'   chmod: rc={rc} err={err}')
rc, out, err = run(f'chown 1000:1000 {upper_dir}/server')
print(f'   chown: rc={rc} err={err}')

# 10. Start container
print('10. Starting container...')
run('docker start moduforge')
time.sleep(5)

# 11. Check status
print('11. Checking status...')
rc, status, err = run('docker inspect moduforge --format={{.State.Status}}')
print(f'   Status: {status}')

# 12. Check health
print('12. Checking health...')
for i in range(6):
    rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
    print(f'   Health: {health}')
    if health == 'healthy':
        break
    time.sleep(5)

# 13. Test API
print('13. Testing API...')
rc, health_check, err = run('curl -s http://localhost:8086/health')
print(f'   {health_check}')

print('\nDone!')
ssh.close()
