import paramiko
import json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', port=22, username='admin', password='csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    out = stdout.read().decode()
    err = stderr.read().decode()
    return out + err

# 1. Container name
out = run('docker ps -a --filter name=moduforge --format "{{.Names}}"')
container = out.strip().split('\n')[0]
print(f"Container: {container}")

# 2. Container /app/data/
print("\n=== Container /app/data/ ===")
print(run(f"docker exec {container} ls -la /app/data/ 2>&1"))

# 3. Database tables
print("=== Database tables ===")
print(run(f'docker exec {container} sh -c "sqlite3 /app/data/moduforge.db \'.tables\'" 2>&1'))

# 4. Users
print("=== Users ===")
print(run(f'docker exec {container} sh -c "sqlite3 /app/data/moduforge.db \'"\'"\'SELECT id, username, role FROM users;\'"\'"\'" 2>&1'))

# 5. Projects
print("=== Projects ===")
print(run(f'docker exec {container} sh -c "sqlite3 /app/data/moduforge.db \'"\'"\'SELECT id, name FROM projects;\'"\'"\'" 2>&1'))

# 6. Project files content length
print("=== Project files content length ===")
print(run(f'docker exec {container} sh -c "sqlite3 /app/data/moduforge.db \'"\'"\'SELECT project_id, path, length(content) FROM project_files LIMIT 15;\'"\'"\'" 2>&1'))

# 7. Try login
print("=== Login ===")
print(run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\''))

# 8. Login with proper JSON
print("\n=== Login (proper) ===")
login_cmd = """curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'"""
print(run(login_cmd))

ssh.close()
