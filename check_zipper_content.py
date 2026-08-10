import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    exit_status = stdout.channel.recv_exit_status()
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return exit_status, out, err

# 1. Check what's in the container's zipper.go
print('1. Checking container zipper.go...')
rc, out, err = run('docker exec moduforge head -100 /tmp/build/internal/service/zipper.go')
print(f'   Content:\n{out[:2000]}')

# 2. Check the local fixed zipper.go
print('\n2. Checking local fixed zipper.go...')
local_zipper = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go'
with open(local_zipper, 'r') as f:
    content = f.read()
    print(f'   Has tmp/: {"tmp/" in content}')
    print(f'   Has DESIGN_DOC.md: {"DESIGN_DOC.md" in content}')
    print(f'   First 500 chars:\n{content[:500]}')

# 3. Check if the file was copied correctly
print('\n3. Checking file on server...')
rc, out, err = run('ls -la /tmp/zipper_fixed.go')
print(f'   Server file: {out}')

# 4. Copy again with explicit content
print('\n4. Re-copying fixed zipper.go...')
# Upload again
sftp = ssh.open_sftp()
sftp.put(local_zipper, '/tmp/zipper_fixed.go')
sftp.close()

# Copy to container
rc, out, err = run('docker cp /tmp/zipper_fixed.go moduforge:/tmp/build/internal/service/zipper.go')
print(f'   docker cp: rc={rc} err={err}')

# Verify
rc, out, err = run('docker exec moduforge grep -c "tmp/" /tmp/build/internal/service/zipper.go')
print(f'   Has tmp/ exclusion: {out}')

rc, out, err = run('docker exec moduforge grep -c "DESIGN_DOC.md" /tmp/build/internal/service/zipper.go')
print(f'   Has DESIGN_DOC.md exclusion: {out}')

print('\nDone!')
ssh.close()
