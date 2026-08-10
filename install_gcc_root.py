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

# 1. Check current container state
print('1. Checking container state...')
rc, health, err = run('docker inspect moduforge --format={{.State.Status}}')
print(f'   Status: {health}')

# 2. Try to install gcc as root
print('2. Installing gcc as root...')
# Use docker exec with --user root
rc, out, err = run('docker exec --user root moduforge apk add --no-cache gcc musl-dev 2>&1')
print(f'   install: rc={rc}')
if out:
    print(f'   stdout: {out[:500]}')
if err:
    print(f'   stderr: {err[:500]}')

# 3. Verify gcc
print('3. Verifying gcc...')
rc, out, err = run('docker exec moduforge which gcc 2>&1')
print(f'   gcc: {out}')

# 4. If gcc not found, try alternative approach
if not out or 'not found' in out.lower():
    print('4. gcc not found, trying alternative...')
    # Check if we can use the build container approach
    # Or check if there's a pre-compiled binary with our changes
    
    # Actually, let me check if the source code has our fixes
    print('   Checking source code...')
    rc, out, err = run('docker exec moduforge grep -c "tmp/" /tmp/build/internal/service/zipper.go 2>&1')
    print(f'   Has tmp/ exclusion: {out}')
    
    # Check if we can compile with a different approach
    # Maybe we need to install build-base instead of just gcc
    print('   Trying build-base...')
    rc, out, err = run('docker exec --user root moduforge apk add --no-cache build-base 2>&1')
    print(f'   install: rc={rc}')
    if out:
        print(f'   stdout: {out[:500]}')
    if err:
        print(f'   stderr: {err[:500]}')
    
    # Verify again
    rc, out, err = run('docker exec moduforge which gcc 2>&1')
    print(f'   gcc: {out}')

# 5. Compile if gcc is available
if out and 'not found' not in out.lower():
    print('5. Compiling with CGO_ENABLED=1...')
    rc, out, err = run('docker exec moduforge sh -c "cd /tmp/build && CGO_ENABLED=1 go build -o /tmp/server-new ./cmd/moduforge 2>&1"')
    print(f'   compile: rc={rc}')
    if out:
        print(f'   output: {out[:1000]}')
    if err:
        print(f'   error: {err[:1000]}')
    
    # Check binary
    rc, out, err = run('docker exec moduforge ls -la /tmp/server-new 2>&1')
    print(f'   binary: {out}')
else:
    print('5. Cannot compile - gcc not available')
    print('   Need alternative approach')

print('\nDone!')
ssh.close()
