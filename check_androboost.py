"""Check AndroBoost-SmartTune status"""
import paramiko, sys

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Check project files
print('=== AndroBoost-SmartTune files ===')
stdin, stdout, stderr = ssh.exec_command("""
# Find the project in storage
find /vol1/docker/volumes/moduforge_data/_data/storage -name "andromon*" -o -name "androst*" -o -name "androwui*" 2>/dev/null | head -10
echo "---"
# Check if binaries exist
ls -la /vol1/docker/volumes/moduforge_data/_data/storage/projects/*/app/src/rust/target/aarch64-linux-android/release/ 2>/dev/null | head -10
echo "---"
# Check WebUI
ls -la /vol1/docker/volumes/moduforge_data/_data/storage/projects/*/app/src/go/ 2>/dev/null | head -10
""")
sys.stdout.buffer.write(stdout.read())
print()

# Check running processes
print('\n=== Container processes ===')
stdin, stdout, stderr = ssh.exec_command('docker exec moduforge ps aux 2>/dev/null | head -20')
sys.stdout.buffer.write(stdout.read())
print()

# Check WebUI endpoint
print('\n=== WebUI test ===')
stdin, stdout, stderr = ssh.exec_command('curl -s http://localhost:8081/ 2>&1 | head -5 || echo "Port 8081 not accessible"')
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
