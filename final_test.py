import paramiko, json

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Login
print("=== Login ===")
out, err = run("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(out)
token = login.get('token', '')
print(f"User: {login.get('user', {}).get('username', 'FAILED')}")

if token:
    # Projects
    print("\n=== Projects ===")
    out, err = run(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    try:
        projects = json.loads(out)
        if isinstance(projects, list):
            print(f"Count: {len(projects)}")
            for p in projects:
                print(f"  - {p.get('name', 'unnamed')}")
        else:
            print(f"Response: {out[:300]}")
    except:
        print(f"Raw: {out[:300]}")

    # AI Conversations
    print("\n=== AI Conversations ===")
    out, err = run(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
    try:
        convs = json.loads(out)
        if isinstance(convs, dict) and 'conversations' in convs:
            print(f"Count: {len(convs['conversations'])}")
            for cv in convs['conversations'][:5]:
                print(f"  - {cv.get('title', 'untitled')[:50]}")
        elif isinstance(convs, list):
            print(f"Count: {len(convs)}")
        else:
            print(f"Response: {out[:300]}")
    except:
        print(f"Raw: {out[:300]}")

    # Build tasks
    print("\n=== Build Tasks ===")
    out, err = run(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
    try:
        projects = json.loads(out)
        if isinstance(projects, list) and len(projects) > 0:
            pid = projects[0].get('id', '')
            if pid:
                out2, err2 = run(f'curl -s http://localhost:8086/api/v1/projects/{pid}/builds -H "Authorization: Bearer {token}"')
                try:
                    builds = json.loads(out2)
                    if isinstance(builds, list):
                        print(f"Build count: {len(builds)}")
                    else:
                        print(f"Builds: {out2[:200]}")
                except:
                    print(f"Builds raw: {out2[:200]}")
    except:
        pass

    # Notifications
    print("\n=== Notifications ===")
    out, err = run(f'curl -s http://localhost:8086/api/v1/notifications/unread-count -H "Authorization: Bearer {token}"')
    print(f"Unread: {out.strip()}")

client.close()
print("\nDone!")
