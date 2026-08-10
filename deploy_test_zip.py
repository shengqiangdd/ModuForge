import paramiko
import time
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

# 1. Deploy new binary
print('1. Deploying new binary...')
run('docker stop moduforge')
time.sleep(2)

# Get UpperDir
rc, merged_dir, err = run('docker inspect moduforge --format={{.GraphDriver.Data.MergedDir}}')
upper_dir = merged_dir.replace('/merged', '/diff')
print(f'   UpperDir: {upper_dir}')

# Copy new binary to UpperDir
rc, out, err = run(f'docker cp moduforge:/tmp/server-new {upper_dir}/server')
print(f'   docker cp: rc={rc} err={err}')
run(f'chmod +x {upper_dir}/server')
run(f'chown 1000:1000 {upper_dir}/server')

# Verify
rc, out, err = run(f'ls -la {upper_dir}/server')
print(f'   {out}')

# 2. Start container
print('2. Starting container...')
run('docker start moduforge')
time.sleep(5)

# 3. Check status
print('3. Checking status...')
rc, status, err = run('docker inspect moduforge --format={{.State.Status}}')
print(f'   Status: {status}')

# 4. Check health
print('4. Checking health...')
for i in range(6):
    rc, health, err = run('docker inspect moduforge --format={{.State.Health.Status}}')
    print(f'   Health: {health}')
    if health == 'healthy':
        break
    time.sleep(5)

# 5. Test API
print('5. Testing API...')
rc, health_check, err = run('curl -s http://localhost:8086/health')
print(f'   {health_check}')

# 6. Test zip export
print('6. Testing zip export...')
# Get JWT token
rc, out, err = run('''curl -s http://localhost:8086/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' ''')
import json
try:
    token_data = json.loads(out)
    token = token_data.get('token', '')
    print(f'   Token: {token[:50]}...')
except:
    token = ''
    print(f'   Failed to get token: {out}')

if token:
    # Create test project
    rc, out, err = run(f'''curl -s http://localhost:8086/api/v1/projects -X POST -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d '{{"name":"Test Module","module_type":"universal","description":"Test"}}' ''')
    print(f'   Create project: {out[:200]}')
    
    try:
        project_data = json.loads(out)
        project_id = project_data.get('id', '')
        print(f'   Project ID: {project_id}')
        
        if project_id:
            # Add test files
            files = [
                ('module.prop', 'id=test_module\nname=Test Module\nversion=1.0'),
                ('index.html', '<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Test</h1></body></html>'),
                ('css/styles.css', 'body { font-family: Arial; }'),
                ('js/app.js', 'console.log("test");'),
                ('service.sh', '#!/system/bin/sh\n# Service'),
                ('customize.sh', '#!/system/bin/sh\n# Install'),
                ('src/main.go', 'package main\nfunc main() {}'),
                ('tmp/test.sh', '#!/bin/sh\n# Temp'),
                ('DESIGN_DOC.md', '# Design'),
                ('system/bin/test', 'binary'),
            ]
            
            for path, content in files:
                rc, out, err = run(f'''curl -s http://localhost:8086/api/v1/projects/{project_id}/files/{path} -X PUT -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d '{{"content":"{content}"}}' ''')
                print(f'   Add {path}: {out[:100]}')
            
            # Export zip
            rc, out, err = run(f'curl -s -o /tmp/test_module.zip http://localhost:8086/api/v1/projects/{project_id}/export-zip -H "Authorization: Bearer {token}"')
            print(f'   Export zip: rc={rc}')
            
            # Download and analyze
            sftp = ssh.open_sftp()
            try:
                sftp.get('/tmp/test_module.zip', r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\build_artifacts\test_module_v3.zip')
                print('   Downloaded zip')
                
                # Analyze zip
                import zipfile
                zip_path = r'C:\Users\22875\.qwenpaw\workspaces\default\ModuForge\build_artifacts\test_module_v3.zip'
                with zipfile.ZipFile(zip_path, 'r') as zf:
                    print('\n   === ZIP Contents ===')
                    for info in zf.infolist():
                        print(f'   {info.filename} ({info.file_size} bytes)')
                    
                    print('\n   === Exclusion Check ===')
                    has_webroot = any('webroot/' in info.filename for info in zf.infolist())
                    print(f'   Has webroot directory: {has_webroot}')
                    
                    excluded_files = ['tmp/', 'DESIGN_DOC.md', 'src/', '.build_cache/', 'app/backend/']
                    for exc in excluded_files:
                        found = any(exc in info.filename for info in zf.infolist())
                        status = 'FOUND (BAD!)' if found else 'excluded (GOOD)'
                        print(f'   {exc}: {status}')
            except Exception as e:
                print(f'   Download failed: {e}')
            finally:
                sftp.close()
    except Exception as e:
        print(f'   Error: {e}')

print('\nDone!')
ssh.close()
