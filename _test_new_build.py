import paramiko
import json
import time

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)

# Get project ID
_, o, _ = c.exec_command('curl -s http://localhost:8086/api/v1/projects')
projects = json.loads(o.read().decode())
if projects:
    pid = projects[0]['id']
    print(f'Project: {projects[0]["name"]} (id={pid})')
    
    # Trigger build
    print('\n=== Triggering build ===')
    _, o2, _ = c.exec_command(f'''curl -s -X POST http://localhost:8086/api/v1/projects/{pid}/build -H "Content-Type: application/json" -d '{{}}' 2>&1''')
    result = o2.read().decode()
    print(f'Build response: {result[:200]}')
    
    # Wait for build
    print('\n=== Waiting for build (60s) ===')
    time.sleep(60)
    
    # Check latest artifact
    print('\n=== Checking latest artifact ===')
    _, o3, _ = c.exec_command('''ls -t /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/data/storage/artifacts/*/module.zip 2>/dev/null | head -1''')
    zip_path = o3.read().decode().strip()
    print(f'Latest: {zip_path}')
    
    # Check META-INF
    _, o4, _ = c.exec_command(f'python3 -c "import zipfile; z=zipfile.ZipFile(\'{zip_path}\'); print([f for f in z.namelist() if \'META-INF\' in f])"')
    print(f'META-INF check: {o4.read().decode().strip()}')
else:
    print('No projects found')

c.close()
