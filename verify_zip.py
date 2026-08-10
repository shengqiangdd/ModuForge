#!/usr/bin/env python3
import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode().strip()

# Get JWT token
resp = json.loads(run('''curl -s http://localhost:8080/api/v1/auth/login -X POST -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}' '''))
token = resp.get('token', '')
print(f"Token: {token[:40]}...")

# List projects
projects = json.loads(run(f'curl -s http://localhost:8080/api/v1/projects -H "Authorization: Bearer {token}"'))
print(f"\nProjects ({len(projects)}):")
for p in projects:
    print(f"  - {p['id']}: {p['name']}")

if projects:
    pid = projects[0]['id']
    print(f"\nExporting project: {pid}")
    
    # Export zip
    status = run(f'curl -s -o /tmp/test_export.zip -w "%{{http_code}}" http://localhost:8080/api/v1/projects/{pid}/export-zip -H "Authorization: Bearer {token}"')
    print(f"Export status: {status}")
    
    # Check zip contents
    print("\nZip contents:")
    zip_list = run('python3 -c "import zipfile; z=zipfile.ZipFile(\'/tmp/test_export.zip\'); [print(n) for n in sorted(z.namelist())]"')
    print(zip_list)
    
    # Check if webroot exists
    has_webroot = any('webroot' in n for n in zip_list.split('\n'))
    has_tmp = any('tmp/' in n or n.startswith('tmp/') for n in zip_list.split('\n'))
    has_design = any('DESIGN_DOC' in n for n in zip_list.split('\n'))
    has_upload = any('upload' in n and 'upload' == n.strip() for n in zip_list.split('\n'))
    
    print(f"\n✓ Has webroot wrapper: {has_webroot}")
    print(f"{'✗' if has_tmp else '✓'} Excludes tmp/: {'NO' if has_tmp else 'YES'}")
    print(f"{'✗' if has_design else '✓'} Excludes DESIGN_DOC.md: {'NO' if has_design else 'YES'}")
    print(f"{'✗' if has_upload else '✓'} Excludes upload: {'NO' if has_upload else 'YES'}")

ssh.close()
