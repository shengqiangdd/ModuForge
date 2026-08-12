"""Read BuildWorkspace.svelte - the main build component"""
import paramiko, sys

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

base = '/vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/frontend/src'

# Read BuildWorkspace.svelte
print('=== BuildWorkspace.svelte ===')
stdin, stdout, stderr = ssh.exec_command(f'cat {base}/lib/components/editor/BuildWorkspace.svelte')
sys.stdout.buffer.write(stdout.read())
print()

ssh.close()
