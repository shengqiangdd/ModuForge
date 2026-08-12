import paramiko, json, time

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Check container status
print("=== Container status ===")
out, err = run("docker ps -a | grep moduforge")
print(out)

# Check if container is healthy
print("\n=== Health check ===")
out, err = run("docker exec moduforge wget -q -O /dev/null http://localhost:8080/health 2>&1 && echo OK || echo FAIL")
print(f"Internal: {out.strip()}")

out, err = run("curl -s http://localhost:8086/health 2>&1")
print(f"External: {out.strip()}")

# Query DB directly from inside container
print("\n=== Query DB from container ===")
out, err = run("docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT COUNT(*) FROM projects;\"'")
print(f"Projects count: {out.strip()}")

out, err = run("docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT id, name, user_id, deleted_at FROM projects LIMIT 3;\"'")
print(f"Projects:\n{out}")

# Check the admin user
print("\n=== Admin user ===")
out, err = run("docker exec moduforge sh -c 'sqlite3 /data/moduforge.db \"SELECT id, username, role FROM users WHERE username=\\\"admin\\\";\"'")
print(f"Admin: {out.strip()}")

# Check what the API handler does - try with different user
print("\n=== Test with curl verbose ===")
out, err = run("""curl -sv http://localhost:8086/api/v1/projects -H "Authorization: Bearer $(curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')" 2>&1 | tail -20""")
print(out)

# Check container logs again
print("\n=== Container logs (last 10) ===")
out, err = run("docker logs moduforge --tail 10 2>&1")
# Filter for relevant lines
for line in out.split('\n'):
    if line.strip() and not line.startswith('{') and not line.startswith('total'):
        print(line[:150])

# Check if there's a WAL file with data
print("\n=== WAL file check ===")
out, err = run("ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db*")
print(out)

client.close()
