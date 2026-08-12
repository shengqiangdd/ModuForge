import paramiko, json

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def api(method, path, token=None, body=None):
    headers = f'-H "Content-Type: application/json"'
    auth = f'-H "Authorization: Bearer {token}"' if token else ''
    body_str = json.dumps(body) if body else ''
    cmd = f'curl -s -X {method} http://localhost:8086{path} {headers} {auth} {("-d \'" + body_str + "\'") if body_str else ""} 2>&1'
    stdin, stdout, stderr = client.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='replace')

# Login
login = json.loads(api('POST', '/api/v1/auth/login', body={'username':'admin','password':'admin123'}))
token = login.get('token', '')
print(f"Login: {login}")

# Check projects
projects = api('GET', '/api/v1/projects', token)
print(f"\nProjects raw: {projects[:500]}")

# Check users via login page
print(f"\nLogin response keys: {list(login.keys())}")

# Check if the DB has the right data using the binary
# Find the binary
stdin, stdout, stderr = client.exec_command('docker exec moduforge find / -name "moduforge" -type f 2>/dev/null | head -5')
bins = stdout.read().decode('utf-8', errors='replace')
print(f"\nBinary paths: {bins}")

# Check container image
stdin, stdout, stderr = client.exec_command('docker inspect moduforge --format="{{.Config.Image}}"')
print(f"Image: {stdout.read().decode()}")

# Check if there's a docker-compose.yml somewhere
stdin, stdout, stderr = client.exec_command('find /vol1 -name "docker-compose.yml" 2>/dev/null | head -5')
print(f"\nCompose files: {stdout.read().decode()}")

# Check the volume that's actually mounted
stdin, stdout, stderr = client.exec_command('docker inspect moduforge --format="{{json .Mounts}}"')
mounts = stdout.read().decode()
print(f"\nMounts: {mounts[:500]}")

client.close()
