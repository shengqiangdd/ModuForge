import paramiko
import time
import json
import zipfile
import os

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
projects = json.loads(out)
project_id = projects[0].get('id', '')
print(f'   Using project: {project_id}')

# 3. Add module.prop
print('3. Adding module.prop...')
module_prop_content = 'id=test_module\\nname=Test Module\\nversion=1.0\\nversionCode=1\\nauthor=Test'
rc, out, err = run(f'''curl -s http://localhost:8086/api/v1/projects/{project_id}/files/module.prop -X PUT -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d '{{"content":"{module_prop_content}"}}' ''')
print(f'   Add module.prop: {out[:200]}')

# 4. Add more test files
print('4. Adding test files...')
test_files = [
    ('service.sh', '#!/system/bin/sh\\n# Service script'),
    ('customize.sh', '#!/system/bin/sh\\n# Install script'),
    ('src/main.go', 'package main\\nfunc main() {}'),
    ('tmp/test.sh', '#!/bin/sh\\n# Temp file'),
    ('DESIGN_DOC.md', '# Design Document'),
    ('system/bin/test_binary', 'binary content'),
]

for path, content in test_files:
    rc, out, err = run(f'''curl -s http://localhost:8086/api/v1/projects/{project_id}/files/{path} -X PUT -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d '{{"content":"{content}"}}' ''')
    print(f'   Add {path}: {out[:100]}')

# 5. List files
print('5. Listing files...')
rc, out, err = run(f'curl -s http://localhost:8086/api/v1/projects/{project_id}/files -H "Authorization: Bearer {token}"')
files = json.loads(out)
print(f'   Files: {[f["path"] for f in files]}')

# 6. Export zip (POST method!)
print('6. Exporting zip (POST)...')
rc, out, err = run(f'curl -s -o /tmp/test_export.zip -w "%{{http_code}}" http://localhost:8086/api/v1/projects/{project_id}/export-zip -X POST -H "Authorization: Bearer {token}"')
print(f'   HTTP status: {out}')

# 7. Check exported file
print('7. Checking exported file...')
rc, out, err = run('ls -la /tmp/test_export.zip 2>&1')
print(f'   {out}')

rc, out, err = run('file /tmp/test_export.zip 2>&1')
print(f'   Type: {out}')

# 8. Download and analyze
print('8. Downloading and analyzing...')
sftp = ssh.open_sftp()
try:
    local_path = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\build_artifacts\test_module_v5.zip'
    sftp.get('/tmp/test_export.zip', local_path)
    print(f'   Downloaded to: {local_path}')
    
    # Analyze zip
    with zipfile.ZipFile(local_path, 'r') as zf:
        print('\n   === ZIP Contents ===')
        for info in zf.infolist():
            print(f'   {info.filename} ({info.file_size} bytes)')
        
        print('\n   === Exclusion Check ===')
        has_webroot = any('webroot/' in info.filename for info in zf.infolist())
        print(f'   Has webroot directory: {has_webroot}')
        
        excluded_files = ['tmp/', 'DESIGN_DOC.md', 'src/', '.build_cache/', 'app/backend/', '*.md', '*.go', '*.rs']
        for exc in excluded_files:
            found = any(exc in info.filename for info in zf.infolist())
            status = 'FOUND (BAD!)' if found else 'excluded (GOOD)'
            print(f'   {exc}: {status}')
        
        print('\n   === Expected Files ===')
        expected_files = ['module.prop', 'service.sh', 'customize.sh', 'META-INF/']
        for exp in expected_files:
            found = any(exp in info.filename for info in zf.infolist())
            status = 'present (GOOD)' if found else 'MISSING (BAD!)'
            print(f'   {exp}: {status}')
except Exception as e:
    print(f'   Error: {e}')
finally:
    sftp.close()

print('\nDone!')
ssh.close()
