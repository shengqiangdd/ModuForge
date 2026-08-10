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

# 1. Copy fixed zipper.go
print('1. Copying fixed zipper.go...')
sftp = ssh.open_sftp()
sftp.put(r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go', '/tmp/zipper_fixed.go')
sftp.close()
rc, out, err = run('docker cp /tmp/zipper_fixed.go moduforge:/tmp/build/internal/service/zipper.go')
print(f'   docker cp: rc={rc} err={err}')

# 2. Verify
print('2. Verifying fix...')
rc, out, err = run('docker exec moduforge grep -c "tmp/" /tmp/build/internal/service/zipper.go')
print(f'   Has tmp/ exclusion: {out}')
rc, out, err = run('docker exec moduforge grep -c "DESIGN_DOC.md" /tmp/build/internal/service/zipper.go')
print(f'   Has DESIGN_DOC.md exclusion: {out}')
rc, out, err = run('docker exec moduforge grep -c "\\*.md" /tmp/build/internal/service/zipper.go')
print(f'   Has *.md exclusion: {out}')

# 3. Recompile
print('3. Recompiling...')
rc, out, err = run('docker exec moduforge sh -c "cd /tmp/build && CGO_ENABLED=1 go build -o /tmp/server-v2 ./cmd/moduforge 2>&1"', timeout=180)
print(f'   compile: rc={rc}')
if out:
    print(f'   output: {out[:500]}')

# 4. Deploy
print('4. Deploying...')
run('docker stop moduforge')
time.sleep(2)
rc, merged_dir, err = run('docker inspect moduforge --format={{.GraphDriver.Data.MergedDir}}')
upper_dir = merged_dir.replace('/merged', '/diff')
rc, out, err = run(f'docker cp moduforge:/tmp/server-v2 {upper_dir}/server')
print(f'   docker cp: rc={rc} err={err}')
run(f'chmod +x {upper_dir}/server')
run(f'chown 1000:1000 {upper_dir}/server')

# 5. Start and test
print('5. Starting container...')
run('docker start moduforge')
time.sleep(8)
rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
print(f'   Health: {health}')
rc, health_check, err = run('curl -s http://localhost:8086/health')
print(f'   API: {health_check}')

print('\nDone!')
ssh.close()
