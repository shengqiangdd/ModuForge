import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=15)
    out = o.read().decode(errors='replace').strip()
    if out: print(out[:2000])
    return out

# Login
login_out = run('curl -sf -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"csq","password":"csq0216"}\'')
token = json.loads(login_out)['token']

# Try different API endpoints for projects
print("\n=== Try /api/v1/projects ===")
run(f'curl -sf http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')

print("\n=== Try /api/v1/user/projects ===")
run(f'curl -sf http://localhost:8086/api/v1/user/projects -H "Authorization: Bearer {token}"')

# Check what endpoints exist
print("\n=== Try /api/v1/ ===")
run(f'curl -sf http://localhost:8086/api/v1/ -H "Authorization: Bearer {token}"')

# Try the known project ID from memory
print("\n=== Try known project ID ===")
run(f'curl -sf http://localhost:8086/api/v1/projects/1785249992652501794-1864 -H "Authorization: Bearer {token}"')
run(f'curl -sf http://localhost:8086/api/v1/projects/1785249992652501794-1864/files -H "Authorization: Bearer {token}"')

ssh.close()
