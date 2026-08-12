"""Check csq user data via ModuForge API"""
import urllib.request, json

BASE = 'http://192.168.2.9:8086'

# Login as csq
data = json.dumps({'username': 'csq', 'password': 'csq0216'}).encode()
req = urllib.request.Request(f'{BASE}/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
try:
    resp = urllib.request.urlopen(req)
    login = json.loads(resp.read())
    token = login['token']
    print(f'csq login OK, user_id={login["user"]["id"]}')
except Exception as e:
    print(f'csq login FAILED: {e}')
    # Try admin
    data = json.dumps({'username': 'admin', 'password': 'admin123'}).encode()
    req = urllib.request.Request(f'{BASE}/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
    resp = urllib.request.urlopen(req)
    login = json.loads(resp.read())
    token = login['token']
    print(f'admin login OK, user_id={login["user"]["id"]}')

headers = {'Authorization': f'Bearer {token}'}

# Check projects
req = urllib.request.Request(f'{BASE}/api/v1/projects', headers=headers)
resp = urllib.request.urlopen(req)
projects = json.loads(resp.read())
print(f'\nProjects: {len(projects) if projects else 0}')
if projects:
    for p in projects:
        print(f'  - {p["name"]} (id={p["id"]}, user_id={p["user_id"]})')

# Check AI conversations
req = urllib.request.Request(f'{BASE}/api/v1/ai/conversations', headers=headers)
resp = urllib.request.urlopen(req)
convs = json.loads(resp.read())
conv_list = convs.get('conversations', convs) if isinstance(convs, dict) else convs
print(f'\nAI Conversations: {len(conv_list) if conv_list else 0}')
if conv_list:
    for cv in conv_list[:5]:
        print(f'  - {cv.get("title","?")} (user_id={cv.get("user_id","?")})')

# Check build tasks
req = urllib.request.Request(f'{BASE}/api/v1/builds', headers=headers)
resp = urllib.request.urlopen(req)
builds = json.loads(resp.read())
build_list = builds.get('builds', builds) if isinstance(builds, dict) else builds
print(f'\nBuild Tasks: {len(build_list) if build_list else 0}')
