import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Test 1: Check if security scanner has the safe module vars exclusion
print('=== Test 1: Check security_scanner.go for safe module vars ===')
_, o1, _ = c.exec_command('docker exec moduforge grep -n "safeModuleVars\|MODPATH\|MODID\|KSU" /app/server 2>/dev/null | head -5')
print(o1.read().decode().strip() or 'Binary file - checking differently')

# Test 2: Check if zipper.go has .es* and .shell* exclusion
print('\n=== Test 2: Check zipper.go for .es* and .shell* ===')
_, o2, _ = c.exec_command('docker exec moduforge strings /app/server | grep -E "\.es\*|\.shell\*" | head -5')
print(o2.read().decode().strip())

# Test 3: Test build a sample module
print('\n=== Test 3: Quick API test ===')
_, o3, _ = c.exec_command('curl -s http://localhost:8086/api/v1/health 2>&1 | head -5')
print(o3.read().decode().strip())

c.close()
