import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Check the build_module.go for EnsureMetaInf
print('=== Check EnsureMetaInf in build_module.go ===')
_, o1, _ = c.exec_command('''docker exec moduforge sh -c 'grep -n "EnsureMetaInf\|EnsureMETA\|META-INF" /app/build_module.go 2>/dev/null | head -20' ''')
print(o1.read().decode().strip())

# Check the security scanner for false positive fixes
print('\n=== Check security_scanner.go for exclude patterns ===')
_, o2, _ = c.exec_command('''docker exec moduforge sh -c 'grep -n "exclude\|Exclude\|pattern\|\.es\|\.shell" /app/security_scanner.go 2>/dev/null | head -20' ''')
print(o2.read().decode().strip())

# Check webroot packaging
print('\n=== Check webui packaging in build_module.go ===')
_, o3, _ = c.exec_command('''docker exec moduforge sh -c 'grep -n "webroot\|webui" /app/build_module.go 2>/dev/null | head -20' ''')
print(o3.read().decode().strip())

c.close()
