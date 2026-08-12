"""Read frontend build components"""
import paramiko, sys

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

base = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/frontend/src'

# Find the main build page
print('=== Build page files ===')
stdin, stdout, stderr = ssh.exec_command(f"""
find {base} -name "*.svelte" -o -name "*.ts" | xargs grep -l "build.*log\\|StreamLogs\\|EventSource\\|SSE" 2>/dev/null | head -10
""")
sys.stdout.buffer.write(stdout.read())
print()

# Read BuildHistory component
print('\n=== BuildHistory.svelte ===')
stdin, stdout, stderr = ssh.exec_command(f'cat {base}/lib/components/editor/BuildHistory.svelte')
sys.stdout.buffer.write(stdout.read())
print()

# Read BuildConfig component
print('\n=== BuildConfig.svelte ===')
stdin, stdout, stderr = ssh.exec_command(f'cat {base}/lib/components/editor/BuildConfig.svelte')
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
