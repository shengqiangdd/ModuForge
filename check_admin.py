"""Check what admin has (which was originally csq's data)"""
import urllib.request, json

BASE = 'http://192.168.2.9:8086'

# Login as admin
data = json.dumps({'username': 'admin', 'password': 'admin123'}).encode()
req = urllib.request.Request(f'{BASE}/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
resp = urllib.request.urlopen(req)
login = json.loads(resp.read())
token = login['token']
print(f'admin login OK, user_id={login["user"]["id"]}')

headers = {'Authorization': f'Bearer {token}'}

# Projects
req = urllib.request.Request(f'{BASE}/api/v1/projects', headers=headers)
resp = urllib.request.urlopen(req)
projects = json.loads(resp.read())
print(f'\nAdmin Projects: {len(projects) if projects else 0}')
if projects:
    for p in projects:
        print(f'  - {p["name"]} (id={p["id"][:16]}..., user_id={p["user_id"][:16]}...)')

# AI conversations
req = urllib.request.Request(f'{BASE}/api/v1/ai/conversations', headers=headers)
resp = urllib.request.urlopen(req)
convs = json.loads(resp.read())
conv_list = convs.get('conversations', convs) if isinstance(convs, dict) else convs
print(f'\nAdmin AI Conversations: {len(conv_list) if conv_list else 0}')
if conv_list:
    for cv in conv_list[:10]:
        print(f'  - {cv.get("title","?")[:50]}')
