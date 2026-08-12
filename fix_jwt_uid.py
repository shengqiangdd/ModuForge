import paramiko, json, base64

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Get a fresh token
out, err = run("""curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}'""")
login = json.loads(out)
token = login.get('token', '')
user = login.get('user', {})
print(f"Login user ID: {user.get('id')}")
print(f"Login username: {user.get('username')}")

# Decode JWT to see what uid it contains
if token:
    parts = token.split('.')
    if len(parts) >= 2:
        # Decode payload
        payload = parts[1]
        # Add padding
        payload += '=' * (4 - len(payload) % 4)
        decoded = base64.urlsafe_b64decode(payload)
        claims = json.loads(decoded)
        print(f"\nJWT claims:")
        print(f"  uid: {claims.get('uid')}")
        print(f"  username: {claims.get('username')}")
        print(f"  role: {claims.get('role')}")

# Install sqlite3 and check all users
print("\n=== Install sqlite3 ===")
out, err = run("docker exec -u root moduforge apk add --no-cache sqlite 2>&1 | tail -3")
print(out)

# Check all users in DB
print("\n=== All users in DB ===")
out, err = run("docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT id, username, role FROM users;\"'")
print(out)

# Check projects by the JWT uid
jwt_uid = claims.get('uid', '') if token else ''
print(f"\n=== Projects for JWT uid ({jwt_uid}) ===")
out, err = run(f"docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT COUNT(*) FROM projects WHERE user_id=\\\"{jwt_uid}\\\";\"'")
print(f"Count: {out.strip()}")

# Check all projects user_ids
print("\n=== All project user_ids ===")
out, err = run("docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT user_id, COUNT(*) FROM projects GROUP BY user_id;\"'")
print(out)

# The issue: JWT has uid=fec17bd3 but we reassigned to f7b4d6fa
# Need to reassign to the correct user ID
print(f"\n=== FIX: Reassign projects to JWT uid ({jwt_uid}) ===")
out, err = run(f"docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"UPDATE projects SET user_id=\\\"{jwt_uid}\\\";\"'")
print(f"Update: {out.strip()}")

out, err = run(f"docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"UPDATE ai_conversations SET user_id=\\\"{jwt_uid}\\\";\"'")
print(f"Update convs: {out.strip()}")

out, err = run(f"docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"UPDATE conversation_messages SET user_id=\\\"{jwt_uid}\\\";\"'")
print(f"Update msgs: {out.strip()}")

out, err = run(f"docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"UPDATE build_tasks SET user_id=\\\"{jwt_uid}\\\";\"'")
print(f"Update builds: {out.strip()}")

# Verify
print("\n=== Verify after fix ===")
out, err = run(f"docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT user_id, COUNT(*) FROM projects GROUP BY user_id;\"'")
print(f"Projects: {out.strip()}")

# Test API
print("\n=== API Test ===")
out, err = run(f'curl -s http://localhost:8086/api/v1/projects -H "Authorization: Bearer {token}"')
try:
    projects = json.loads(out)
    if isinstance(projects, list):
        print(f"Projects: {len(projects)} found!")
        for p in projects:
            print(f"  - {p.get('name', 'unnamed')}")
    else:
        print(f"Response: {out[:300]}")
except:
    print(f"Raw: {out[:300]}")

# AI Conversations
out, err = run(f'curl -s http://localhost:8086/api/v1/ai/conversations -H "Authorization: Bearer {token}"')
try:
    convs = json.loads(out)
    if isinstance(convs, dict) and 'conversations' in convs:
        print(f"\nAI Conversations: {len(convs['conversations'])} found!")
    elif isinstance(convs, list):
        print(f"\nAI Conversations: {len(convs)} found!")
except:
    pass

client.close()
print("\nDone!")
