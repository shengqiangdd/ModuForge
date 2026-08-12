"""Reassign data back to csq via API or direct DB fix"""
import urllib.request, json, sqlite3, os, tempfile, paramiko

BASE = 'http://192.168.2.9:8086'
CSQ_ID = 'a4c50d84-a58d-4fbc-a64d-adf93ca14446'
ADMIN_ID = 'fec17bd3-7610-4f2a-b157-24ee1e362d23'

# Step 1: Try to get DB via API (check if there's a backup/restore endpoint)
print('=== Checking API capabilities ===')

# Login as admin
data = json.dumps({'username': 'admin', 'password': 'admin123'}).encode()
req = urllib.request.Request(f'{BASE}/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
resp = urllib.request.urlopen(req)
admin_token = json.loads(resp.read())['token']

# Check if there's a backup download endpoint
for endpoint in ['/api/v1/backup/download', '/api/v1/admin/backup', '/api/v1/admin/db']:
    try:
        req = urllib.request.Request(f'{BASE}{endpoint}', headers={'Authorization': f'Bearer {admin_token}'})
        resp = urllib.request.urlopen(req)
        print(f'  {endpoint}: {resp.status} - {resp.read()[:200]}')
    except Exception as e:
        print(f'  {endpoint}: {e}')

# Step 2: Try to create projects for csq via API
print('\n=== Creating projects for csq via API ===')

# Login as csq
data = json.dumps({'username': 'csq', 'password': 'csq0216'}).encode()
req = urllib.request.Request(f'{BASE}/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
resp = urllib.request.urlopen(req)
csq_token = json.loads(resp.read())['token']
csq_headers = {'Authorization': f'Bearer {csq_token}', 'Content-Type': 'application/json'}

# Get admin's projects details
admin_headers = {'Authorization': f'Bearer {admin_token}'}
req = urllib.request.Request(f'{BASE}/api/v1/projects', headers=admin_headers)
resp = urllib.request.urlopen(req)
admin_projects = json.loads(resp.read())

print(f'Admin has {len(admin_projects)} projects to transfer')

# Create each project under csq
for p in admin_projects:
    create_data = json.dumps({
        'name': p['name'],
        'module_type': p.get('module_type', 'universal'),
        'description': p.get('description', ''),
        'git_url': p.get('git_url', ''),
        'git_branch': p.get('git_branch', 'main')
    }).encode()
    
    req = urllib.request.Request(f'{BASE}/api/v1/projects', data=create_data, headers=csq_headers)
    try:
        resp = urllib.request.urlopen(req)
        new_proj = json.loads(resp.read())
        print(f'  Created: {p["name"]} -> new_id={new_proj.get("id", "?")}')
        
        # Check if project files can be transferred
        # Try to get project files from admin
        req2 = urllib.request.Request(f'{BASE}/api/v1/projects/{p["id"]}/files', headers=admin_headers)
        try:
            resp2 = urllib.request.urlopen(req2)
            files = json.loads(resp2.read())
            print(f'    Files: {len(files) if isinstance(files, list) else "?"}')
        except:
            print(f'    Files: could not retrieve')
            
    except Exception as e:
        print(f'  Failed to create {p["name"]}: {e}')

print('\nDone! Check csq projects now.')
