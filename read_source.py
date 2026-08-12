"""Read source files to understand build routes and log streaming"""
import paramiko, sys

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Read routes.go to find build-related routes
print('=== routes.go (build routes) ===')
stdin, stdout, stderr = ssh.exec_command("""
cat /vol1/docker/overlay2/d9ad66b1f27d35bdc3061c13b1b1b94ad57d62dbd3b9ff2e275f8a4e2423daec/diff/home/moduforge/backend/internal/handler/routes.go | grep -i "build\\|stream\\|log"
""")
sys.stdout.buffer.write(stdout.read())
print()

# Read build handler
print('\n=== build.go (handler) ===')
stdin, stdout, stderr = ssh.exec_command("""
cat /vol1/docker/overlay2/d9ad66b1f27d35bdc3061c13b1b1b94ad57d62dbd3b9ff2e275f8a4e2423daec/diff/home/moduforge/backend/internal/handler/build.go | head -100
""")
sys.stdout.buffer.write(stdout.read())
print()

# Check frontend build page
print('\n=== Frontend build route ===')
stdin, stdout, stderr = ssh.exec_command("""
find /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/frontend/src -name "+page.svelte" -path "*build*" 2>/dev/null
echo "---"
find /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/frontend/src -name "+page*" -path "*editor*" 2>/dev/null | head -5
""")
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
