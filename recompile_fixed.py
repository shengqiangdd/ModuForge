import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=120):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    exit_status = stdout.channel.recv_exit_status()
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return exit_status, out, err

# 1. Copy fixed zipper.go to container
print('1. Copying fixed zipper.go...')
local_zipper = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go'
sftp = ssh.open_sftp()
sftp.put(local_zipper, '/tmp/zipper_fixed.go')
sftp.close()

# Copy to container
rc, out, err = run('docker cp /tmp/zipper_fixed.go moduforge:/tmp/build/internal/service/zipper.go')
print(f'   docker cp: rc={rc} err={err}')

# 2. Verify the fix is in place
print('2. Verifying fix...')
rc, out, err = run('docker exec moduforge grep -c "tmp/" /tmp/build/internal/service/zipper.go')
print(f'   Has tmp/ exclusion: {out}')

rc, out, err = run('docker exec moduforge grep -c "DESIGN_DOC.md" /tmp/build/internal/service/zipper.go')
print(f'   Has DESIGN_DOC.md exclusion: {out}')

# 3. Recompile with CGO_ENABLED=1
print('3. Recompiling with CGO_ENABLED=1...')
rc, out, err = run('docker exec moduforge sh -c "cd /tmp/build && CGO_ENABLED=1 go build -o /tmp/server-fixed ./cmd/moduforge 2>&1"', timeout=180)
print(f'   compile: rc={rc}')
if out:
    print(f'   output: {out[:1000]}')
if err:
    print(f'   error: {err[:1000]}')

# 4. Check compiled binary
print('4. Checking compiled binary...')
rc, out, err = run('docker exec moduforge ls -la /tmp/server-fixed 2>&1')
print(f'   {out}')

# 5. Deploy new binary
print('5. Deploying new binary...')
run('docker stop moduforge')
time.sleep(2)

# Get UpperDir
rc, merged_dir, err = run('docker inspect moduforge --format={{.GraphDriver.Data.MergedDir}}')
upper_dir = merged_dir.replace('/merged', '/diff')
print(f'   UpperDir: {upper_dir}')

# Copy new binary to UpperDir
rc, out, err = run(f'docker cp moduforge:/tmp/server-fixed {upper_dir}/server')
print(f'   docker cp: rc={rc} err={err}')
run(f'chmod +x {upper_dir}/server')
run(f'chown 1000:1000 {upper_dir}/server')

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
