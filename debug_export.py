import paramiko
import time
import json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=60):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    exit_status = stdout.channel.recv_exit_status()
    out = stdout.read().decode().strip()
    err = stderr.read().decode().strip()
    return exit_status, out, err

# 1. Get JWT token
print('1. Getting JWT token...')
rc, out, err = run('''curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
token_data = json.loads(out)
token = token_data.get('token', '')
print(f'   Token: {token[:50]}...')

# 2. List projects
print('2. Listing projects...')
rc, out, err = run(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
print(f'   Projects: {out[:500]}')

# 3. Get project ID
try:
    projects = json.loads(out)
    if projects and len(projects) > 0:
        project_id = projects[0].get('id', '')
        print(f'   Using project: {project_id}')
        
        # 4. List files in project
        print('3. Listing files...')
        rc, out, err = run(f'curl -s http://localhost:8086/api/v1/projects/{project_id}/files -H "Authorization: Bearer {token}"')
        print(f'   Files: {out[:500]}')
        
        # 5. Try export with verbose output
        print('4. Testing export...')
        rc, out, err = run(f'curl -v -o /tmp/test_export.zip http://localhost:8086/api/v1/projects/{project_id}/export-zip -H "Authorization: Bearer {token}" 2>&1')
        print(f'   Export output: {out[:1000]}')
        
        # 6. Check if file was created
        print('5. Checking exported file...')
        rc, out, err = run('ls -la /tmp/test_export.zip 2>&1')
        print(f'   File: {out}')
        
        # 7. Check file content
        rc, out, err = run('file /tmp/test_export.zip 2>&1')
        print(f'   Type: {out}')
        
        # 8. Try to read first few bytes
        rc, out, err = run('xxd /tmp/test_export.zip | head -5 2>&1')
        print(f'   Hex: {out}')
except Exception as e:
    print(f'   Error: {e}')

print('\nDone!')
ssh.close()
