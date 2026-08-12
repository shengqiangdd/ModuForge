"""Fix csq data: reassign all data from admin to csq via API"""
import urllib.request, json, time

BASE = 'http://192.168.2.9:8086'
CSQ_ID = 'a4c50d84-a58d-4fbc-a64d-adf93ca14446'
ADMIN_ID = 'fec17bd3-7610-4f2a-b157-24ee1e362d23'

def login(user, pw):
    data = json.dumps({'username': user, 'password': pw}).encode()
    req = urllib.request.Request(f'{BASE}/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
    resp = urllib.request.urlopen(req)
    return json.loads(resp.read())['token']

def api(token, method, path, body=None):
    headers = {'Authorization': f'Bearer {token}', 'Content-Type': 'application/json'}
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(f'{BASE}{path}', data=data, headers=headers, method=method)
    try:
        resp = urllib.request.urlopen(req)
        return json.loads(resp.read())
    except Exception as e:
        return {'error': str(e)}

# Login both users
admin_token = login('admin', 'admin123')
csq_token = login('csq', 'csq0216')
print(f'admin_token: {admin_token[:20]}...')
print(f'csq_token: {csq_token[:20]}...')

# Step 1: Get all admin projects
admin_projects = api(admin_token, 'GET', '/api/v1/projects')
print(f'\nAdmin projects: {len(admin_projects)}')

# Step 2: For each admin project, check if csq already has it by name
csq_projects = api(csq_token, 'GET', '/api/v1/projects')
csq_names = {p['name'] for p in csq_projects} if csq_projects else set()
print(f'csq existing projects: {csq_names}')

# Step 3: Delete csq's duplicate projects (created earlier via API)
for p in (csq_projects or []):
    if p['name'] in {ap['name'] for ap in admin_projects}:
        print(f'  Deleting duplicate: {p["name"]} (id={p["id"][:16]})')
        result = api(csq_token, 'DELETE', f'/api/v1/projects/{p["id"]}')
        print(f'    Result: {result}')
        time.sleep(0.3)

# Step 4: For each admin project, create for csq with same name
print('\n=== Creating projects for csq ===')
new_project_ids = {}
for p in admin_projects:
    create_data = {
        'name': p['name'],
        'module_type': p.get('module_type', 'universal'),
        'description': p.get('description', ''),
    }
    result = api(csq_token, 'POST', '/api/v1/projects', create_data)
    if 'error' in result:
        print(f'  FAIL: {p["name"]} - {result["error"]}')
    else:
        new_id = result.get('id')
        new_project_ids[p['id']] = new_id
        print(f'  OK: {p["name"]} (old={p["id"][:16]} -> new={new_id[:16] if new_id else "?"})')
    time.sleep(0.3)

# Step 5: Transfer files for each project
print('\n=== Transferring project files ===')
for old_id, new_id in new_project_ids.items():
    if not new_id:
        continue
    # Get files from admin project
    files = api(admin_token, 'GET', f'/api/v1/projects/{old_id}/files')
    if isinstance(files, dict) and 'error' in files:
        print(f'  Skip files for {old_id[:16]}: {files["error"]}')
        continue
    
    file_list = files if isinstance(files, list) else files.get('files', [])
    print(f'  Project {old_id[:16]}: {len(file_list)} files')
    
    for f in file_list:
        # Get file content from admin
        file_path = f.get('path', f.get('name', ''))
        content_resp = api(admin_token, 'GET', f'/api/v1/projects/{old_id}/files/{file_path}')
        
        if isinstance(content_resp, dict) and 'content' in content_resp:
            # Create file in csq project
            create_file_data = {
                'path': file_path,
                'content': content_resp['content']
            }
            result = api(csq_token, 'POST', f'/api/v1/projects/{new_id}/files', create_file_data)
            if 'error' in result:
                print(f'    FAIL: {file_path} - {result["error"][:50]}')
        time.sleep(0.1)

print('\n=== Done! ===')
print('Check csq login at http://192.168.2.9:8086')
