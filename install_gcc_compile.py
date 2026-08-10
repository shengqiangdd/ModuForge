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

# 1. Restore working binary first
print('1. Restoring working binary...')
run('docker stop moduforge')
time.sleep(2)

rc, merged_dir, err = run('docker inspect moduforge --format={{.GraphDriver.Data.MergedDir}}')
upper_dir = merged_dir.replace('/merged', '/diff')

# Restore the working 17MB binary
rc, out, err = run(f'cp /vol1/docker/overlay2/p77oe1sdfai03blepphcn3fgm/diff/server {upper_dir}/server')
print(f'   cp: rc={rc} err={err}')
run(f'chmod +x {upper_dir}/server')
run(f'chown 1000:1000 {upper_dir}/server')

# Start container
run('docker start moduforge')
time.sleep(10)

# Check health
rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
print(f'   Health: {health}')

if health != 'healthy':
    print('   Container not healthy, waiting more...')
    time.sleep(10)
    rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
    print(f'   Health: {health}')

# 2. Install gcc in container
print('2. Installing gcc in container...')
rc, out, err = run('docker exec moduforge apk add --no-cache gcc musl-dev 2>&1')
print(f'   install: rc={rc}')
if out:
    print(f'   stdout: {out[:500]}')
if err:
    print(f'   stderr: {err[:500]}')

# 3. Verify gcc is available
print('3. Verifying gcc...')
rc, out, err = run('docker exec moduforge which gcc 2>&1')
print(f'   gcc: {out}')

# 4. Copy source code again (in case it was lost)
print('4. Copying source code...')
rc, out, err = run('docker exec moduforge sh -c "mkdir -p /tmp/build && cd /tmp/build && tar xf /tmp/backend-src.tar" 2>&1')
print(f'   extract: rc={rc} err={err}')

# 5. Copy fixed zipper.go
print('5. Copying fixed zipper.go...')
local_zipper = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go'
sftp = ssh.open_sftp()
sftp.put(local_zipper, '/tmp/zipper_fixed.go')
sftp.close()
rc, out, err = run('docker cp /tmp/zipper_fixed.go moduforge:/tmp/build/internal/service/zipper.go')
print(f'   docker cp: rc={rc} err={err}')

# 6. Compile with CGO_ENABLED=1
print('6. Compiling with CGO_ENABLED=1...')
rc, out, err = run('docker exec moduforge sh -c "cd /tmp/build && CGO_ENABLED=1 go build -o /tmp/server-new ./cmd/moduforge 2>&1"')
print(f'   compile: rc={rc}')
if out:
    print(f'   output: {out[:1000]}')
if err:
    print(f'   error: {err[:1000]}')

# 7. Check compiled binary
print('7. Checking compiled binary...')
rc, out, err = run('docker exec moduforge ls -la /tmp/server-new 2>&1')
print(f'   {out}')

# 8. Deploy new binary
print('8. Deploying new binary...')
run('docker stop moduforge')
time.sleep(2)

rc, out, err = run(f'docker cp moduforge:/tmp/server-new {upper_dir}/server')
print(f'   docker cp: rc={rc} err={err}')
run(f'chmod +x {upper_dir}/server')
run(f'chown 1000:1000 {upper_dir}/server')

# 9. Start container
print('9. Starting container...')
run('docker start moduforge')
time.sleep(5)

# 10. Check status
print('10. Checking status...')
rc, status, err = run('docker inspect moduforge --format={{.State.Status}}')
print(f'   Status: {status}')

# 11. Check health
print('11. Checking health...')
for i in range(6):
    rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
    print(f'   Health: {health}')
    if health == 'healthy':
        break
    time.sleep(5)

# 12. Test API
print('12. Testing API...')
rc, health_check, err = run('curl -s http://localhost:8086/health')
print(f'   {health_check}')

print('\nDone!')
ssh.close()
