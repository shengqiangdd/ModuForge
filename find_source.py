"""Check backend routes and frontend build page"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Find source code location
print('=== Find source ===')
stdin, stdout, stderr = ssh.exec_command("""
# Check if source code exists on the server
find /vol1 -name "routes.go" -path "*/moduforge*" 2>/dev/null | head -5
echo "---"
find /vol1 -name "build*.go" -path "*/moduforge*" 2>/dev/null | head -10
echo "---"
# Check container for source
docker exec moduforge find / -name "routes.go" 2>/dev/null | head -3
echo "---"
# Check frontend build page
docker exec moduforge find / -name "Build*.svelte" -o -name "build*.svelte" 2>/dev/null | head -5
""")
sys.stdout.buffer.write(stdout.read())
print()

# Check backend routes related to builds
print('\n=== Build routes ===')
stdin, stdout, stderr = ssh.exec_command("""
# Get all registered routes
docker exec moduforge sh -c 'strings /server | grep -i "builds\\|stream\\|logs" | head -20' 2>/dev/null || echo "no strings"
""")
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
