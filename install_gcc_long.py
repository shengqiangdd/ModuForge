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

# 1. Check if container is responsive
print('1. Checking container...')
rc, status, err = run('docker inspect moduforge --format={{.State.Status}}')
print(f'   Status: {status}')

# 2. Check for running apk processes
print('2. Checking for apk processes...')
rc, out, err = run('docker exec moduforge ps aux | grep apk 2>&1')
print(f'   Processes: {out}')

# 3. Wait for any apk processes to finish
print('3. Waiting for apk to finish...')
for i in range(5):
    rc, out, err = run('docker exec moduforge pgrep apk 2>&1')
    if not out:
        print('   No apk processes running')
        break
    print(f'   Waiting... ({i+1}/5)')
    time.sleep(5)

# 4. Try to install gcc with very long timeout
print('4. Installing gcc (long timeout)...')
rc, out, err = run('docker exec --user root moduforge apk add --no-cache gcc musl-dev 2>&1', timeout=300)
print(f'   install: rc={rc}')
if out:
    print(f'   stdout: {out[:500]}')
if err:
    print(f'   stderr: {err[:500]}')

# 5. Verify gcc
print('5. Verifying gcc...')
rc, out, err = run('docker exec moduforge which gcc 2>&1')
print(f'   gcc: {out}')

# 6. If gcc is available, compile
if out and 'not found' not in out.lower() and 'no such' not in out.lower():
    print('6. Compiling with CGO_ENABLED=1...')
    rc, out, err = run('docker exec moduforge sh -c "cd /tmp/build && CGO_ENABLED=1 go build -o /tmp/server-new ./cmd/moduforge 2>&1"', timeout=300)
    print(f'   compile: rc={rc}')
    if out:
        print(f'   output: {out[:1000]}')
    if err:
        print(f'   error: {err[:1000]}')
    
    # Check binary
    rc, out, err = run('docker exec moduforge ls -la /tmp/server-new 2>&1')
    print(f'   binary: {out}')
else:
    print('6. gcc not available')
    print('   Alternative: The working binary (17MB) does not have our zipper.go fixes')
    print('   The fixed binary (22MB) was compiled with CGO_ENABLED=0')
    print('   Need to find a way to compile with CGO_ENABLED=1')

print('\nDone!')
ssh.close()
