import urllib.request, json

# Login
data = json.dumps({"username": "admin", "password": "admin123"}).encode()
req = urllib.request.Request('http://192.168.2.9:8086/api/v1/auth/login', data=data, headers={'Content-Type': 'application/json'})
resp = urllib.request.urlopen(req)
token = json.loads(resp.read().decode())["token"]
headers = {"Authorization": f"Bearer {token}"}

# Get AndroBoost-SmartTune files
pid = "155f1629-6e33-4407-b348-f28698f6f5cd"
req = urllib.request.Request(f'http://192.168.2.9:8086/api/v1/projects/{pid}/files', headers=headers)
resp = urllib.request.urlopen(req)
files = json.loads(resp.read().decode())

print(f"=== AndroBoost-SmartTune Files ({len(files)}) ===")
for f in files:
    name = f.get('name', 'N/A')
    path = f.get('path', 'N/A')
    size = f.get('size', 0)
    ftype = f.get('type', 'N/A')
    print(f"  [{ftype}] {path} ({size} bytes)")

# Check project details
req = urllib.request.Request(f'http://192.168.2.9:8086/api/v1/projects/{pid}', headers=headers)
resp = urllib.request.urlopen(req)
project = json.loads(resp.read().decode())
print(f"\n=== PROJECT DETAILS ===")
print(f"Name: {project.get('name')}")
print(f"Description: {project.get('description', 'N/A')[:100]}...")
print(f"Created: {project.get('created_at')}")
print(f"Updated: {project.get('updated_at')}")
