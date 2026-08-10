import paramiko
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=60):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    exit_status = stdout.channel.recv_exit_status()
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return exit_status, out, err

# 1. Kill any apk processes
print('1. Killing apk processes...')
rc, out, err = run('docker exec --user root moduforge pkill -9 apk 2>&1')
print(f'   pkill: rc={rc} err={err}')
time.sleep(2)

# 2. Remove apk lock
print('2. Removing apk lock...')
rc, out, err = run('docker exec --user root moduforge rm -f /var/lib/apk/lock /var/lib/apk/lockfile /var/cache/apk/archives/lock 2>&1')
print(f'   rm lock: rc={rc} err={err}')

# 3. Try installing gcc again
print('3. Installing gcc...')
rc, out, err = run('docker exec --user root moduforge apk add --no-cache gcc musl-dev 2>&1', timeout=120)
print(f'   install: rc={rc}')
if out:
    print(f'   stdout: {out[:500]}')
if err:
    print(f'   stderr: {err[:500]}')

# 4. Verify gcc
print('4. Verifying gcc...')
rc, out, err = run('docker exec moduforge which gcc 2>&1')
print(f'   gcc: {out}')

# 5. If gcc is available, compile
if out and 'not found' not in out.lower():
    print('5. Compiling with CGO_ENABLED=1...')
    # First, make sure source code has our fixes
    rc, out, err = run('docker exec moduforge grep -c "tmp/" /tmp/build/internal/service/zipper.go 2>&1')
    print(f'   Source has tmp/ exclusion: {out}')
    
    if out == '0':
        print('   Source does not have fixes, copying...')
        local_zipper = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\backend\internal\service\zipper.go'
        sftp = ssh.open_sftp()
        sftp.put(local_zipper, '/tmp/zipper_fixed.go')
        sftp.close()
        rc, out, err = run('docker cp /tmp/zipper_fixed.go moduforge:/tmp/build/internal/service/zipper.go')
        print(f'   docker cp: rc={rc} err={err}')
    
    # Compile
    rc, out, err = run('docker exec moduforge sh -c "cd /tmp/build && CGO_ENABLED=1 go build -o /tmp/server-new ./cmd/moduforge 2>&1"', timeout=180)
    print(f'   compile: rc={rc}')
    if out:
        print(f'   output: {out[:1000]}')
    if err:
        print(f'   error: {err[:1000]}')
    
    # Check binary
    rc, out, err = run('docker exec moduforge ls -la /tmp/server-new 2>&1')
    print(f'   binary: {out}')
else:
    print('5. gcc not available, trying alternative...')
    # Check if we can use build-base instead
    rc, out, err = run('docker exec --user root moduforge apk add --no-cache build-base 2>&1', timeout=120)
    print(f'   build-base: rc={rc}')
    if out:
        print(f'   stdout: {out[:500]}')
    if err:
        print(f'   stderr: {err[:500]}')

print('\nDone!')
ssh.close()
