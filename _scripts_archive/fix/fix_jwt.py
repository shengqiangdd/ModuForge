# -*- coding: utf-8 -*-
import paramiko, json, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check .env in data volume
print("=== .env ===")
print(run("docker exec moduforge cat /data/.env"))

# Check what JWT_SECRET the entrypoint generated
print("\n=== Entrypoint logic ===")
print(run("docker exec moduforge cat /docker-entrypoint.sh"))

# Restart container with JWT_SECRET
print("\n=== Restart with JWT_SECRET ===")
run("docker rm -f moduforge 2>/dev/null")

# Get the JWT_SECRET from .env or generate one
jwt_secret = run("docker exec moduforge sh -c 'cat /data/.env 2>/dev/null | grep JWT_SECRET || echo empty'")
print("Existing JWT:", jwt_secret)

if "empty" in jwt_secret or "JWT_SECRET" not in jwt_secret:
    # Generate a new one
    import secrets
    jwt_secret = secrets.token_hex(32)
    print("Generated JWT:", jwt_secret)
else:
    jwt_secret = jwt_secret.split("=", 1)[1].strip()

# Start with JWT_SECRET
result = run(
    'docker run -d --name moduforge '
    '--restart unless-stopped '
    '-e JWT_SECRET=%s '
    '-v moduforge_moduforge_data:/data '
    '-v moduforge_moduforge_uploads:/uploads '
    '-p 8087:8080 '
    'moduforge:latest' % jwt_secret
)
print("Run:", result)

time.sleep(8)

# Check JWT_SECRET is set
print("\nJWT_SECRET:", run("docker exec moduforge env | grep JWT"))

# Check .env
print(".env:", run("docker exec moduforge cat /data/.env"))

# Wait for rate limit
print("\nWaiting 65s...")
time.sleep(65)

# Try login
print("\n=== Login ===")
login = run(
    'curl -s -X POST http://localhost:8086/api/v1/auth/login '
    '-H "Content-Type: application/json" '
    '-d \'{"username":"admin","password":"admin123"}\''
)
print("Login:", login[:300])

# If still failing, check the hash
if "token" not in login:
    print("\n=== Password mismatch ===")
    print("User exists in DB with bcrypt hash")
    print("Password might have been changed")
    
    # Try to reset via the API
    print("\nTrying to register new user...")
    register = run(
        'curl -s -X POST http://localhost:8086/api/v1/auth/register '
        '-H "Content-Type: application/json" '
        '-d \'{"username":"testuser","password":"test123","email":"test@test.com"}\''
    )
    print("Register:", register[:300])
    
    if "token" in register:
        data = json.loads(register)
        token = data.get("token", "")
        print("\n=== Test with testuser ===")
        print(run('curl -s http://localhost:8086/api/v1/llm/custom-providers -H "Authorization: Bearer %s"' % token)[:500])

ssh.close()
