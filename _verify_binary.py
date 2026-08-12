import paramiko
import io, zipfile

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Check if the binary has EnsureMetaInf string
print('=== Check binary for EnsureMetaInf ===')
_, o, _ = c.exec_command('docker exec moduforge strings /server 2>/dev/null | grep -i metainf')
metainf = o.read().decode().strip()
print(f'metainf strings: {metainf}')

# Check if the binary has the update-binary content
print('\n=== Check for update-binary template ===')
_, o2, _ = c.exec_command('docker exec moduforge strings /server 2>/dev/null | grep "update-binary"')
ub = o2.read().decode().strip()
print(f'update-binary: {ub}')

# Check the actual build path in the binary
print('\n=== Check build path ===')
_, o3, _ = c.exec_command('docker exec moduforge strings /server 2>/dev/null | grep -i "webroot\\|metainf" | head -5')
print(o3.read().decode().strip())

# Trigger a new build via the Agent API (with auth)
print('\n=== Try build via Agent chat ===')
_, o4, _ = c.exec_command('''docker exec moduforge curl -s http://localhost:8080/api/v1/health 2>&1''')
print(f'Health: {o4.read().decode().strip()[:200]}')

c.close()
