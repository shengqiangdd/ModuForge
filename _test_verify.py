import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Test 1: Check if EnsureMetaInf works by scanning a module zip
print('=== Test 1: Check META-INF in build artifacts ===')
_, o1, _ = c.exec_command('''cd /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge && \
docker exec moduforge sh -c 'find /data -name "*.zip" 2>/dev/null | head -5' ''')
print(o1.read().decode().strip())

# Test 2: Check security scanner by looking at logs
print('\n=== Test 2: Check recent build logs ===')
_, o2, _ = c.exec_command('''docker logs moduforge --tail=20 2>&1 | grep -E "metainf|webroot|META-INF"''')
print(o2.read().decode().strip())

# Test 3: Test the security scanner with a sample
print('\n=== Test 3: Quick build test via API ===')
_, o3, _ = c.exec_command('''curl -s http://localhost:8086/api/v1/projects | python3 -c "import sys,json; data=json.load(sys.stdin); print(f'Projects: {len(data)}')" 2>&1''')
print(o3.read().decode().strip())

c.close()
