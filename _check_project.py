import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=15)
    out = o.read().decode(errors='replace').strip()
    if out: print(out)
    return out

# 1. Get JWT token
print("=== Login ===")
login_out = run('curl -sf -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"csq","password":"csq0216"}\'')
token = json.loads(login_out)['token']

# 2. List projects to find AndroBoost
print("\n=== Projects ===")
proj_out = run(f'curl -sf http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
projects = json.loads(proj_out)
if isinstance(projects, dict) and 'projects' in projects:
    projects = projects['projects']
for p in projects:
    print(f"  ID: {p.get('id', p.get('project_id', '?'))}  Name: {p.get('name', '?')}")

# 3. Find AndroBoost project
ab_project = None
for p in projects:
    name = p.get('name', '')
    if 'andro' in name.lower() or 'smart' in name.lower() or 'boost' in name.lower():
        ab_project = p
        break
if not ab_project:
    # Try first project
    ab_project = projects[0] if projects else None

if not ab_project:
    print("No projects found!")
    ssh.close()
    exit(1)

pid = ab_project.get('id', ab_project.get('project_id'))
print(f"\n=== Using project: {ab_project.get('name')} (ID: {pid}) ===")

# 4. List files
print("\n=== Files ===")
files_out = run(f'curl -sf http://localhost:8086/api/v1/projects/{pid}/files -H "Authorization: Bearer {token}"')
files = json.loads(files_out)
if isinstance(files, dict) and 'files' in files:
    files = files['files']
for f in files:
    fname = f.get('name', f.get('path', '?'))
    fid = f.get('id', '?')
    print(f"  {fname} (id={fid})")

ssh.close()
