import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    exit_status = stdout.channel.recv_exit_status()
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return exit_status, out, err

# 1. Check container status
print('1. Checking container status...')
rc, status, err = run('docker inspect moduforge --format={{.State.Status}}')
print(f'   Status: {status}')

# 2. Check if gcc is already installed
print('2. Checking if gcc exists...')
rc, out, err = run('docker exec moduforge ls /usr/bin/gcc 2>&1')
print(f'   gcc check: {out}')

# 3. If not, try to install with timeout
if 'No such file' in out or rc != 0:
    print('3. Installing gcc (with timeout)...')
    rc, out, err = run('docker exec --user root moduforge sh -c "apk add --no-cache gcc musl-dev 2>&1"', timeout=120)
    print(f'   install: rc={rc}')
    if out:
        print(f'   stdout: {out[:300]}')
    if err:
        print(f'   stderr: {err[:300]}')
    
    # Verify
    rc, out, err = run('docker exec moduforge which gcc 2>&1')
    print(f'   gcc: {out}')

# 4. Alternative: Check if we can use a different approach
print('4. Alternative approach...')
# Check if there's a way to get a working binary with our fixes
# The issue is we need CGO_ENABLED=1 AND our zipper.go fixes

# Let me check if the container has the source code with our fixes
rc, out, err = run('docker exec moduforge grep -c "tmp/" /tmp/build/internal/service/zipper.go 2>&1')
print(f'   Source has tmp/ exclusion: {out}')

# Check if we can compile with a different method
print('5. Checking compilation options...')
rc, out, err = run('docker exec moduforge go version 2>&1')
print(f'   Go version: {out}')

print('\nDone!')
ssh.close()
