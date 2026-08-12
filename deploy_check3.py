import paramiko, json, sys

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd)
    out = stdout.read()
    err = stderr.read()
    try:
        print(out.decode('utf-8', errors='replace'))
    except:
        print(repr(out))
    if err:
        try:
            print(err.decode('utf-8', errors='replace'))
        except:
            print(repr(err))

print("=== Which sqlite3? ===")
run('docker exec moduforge which sqlite3 2>&1 || echo "no sqlite3"')

print("\n=== Docker exec sh -c ===")
run('docker exec moduforge sh -c "ls /data/*.db"')

print("\n=== Try python sqlite3 ===")
run('docker exec moduforge python3 -c "import sqlite3; c=sqlite3.connect(\\\"/data/moduforge.db\\\"); print(c.execute(\\\"SELECT name FROM sqlite_master WHERE type=table\\\").fetchall())" 2>&1 || echo "no python3"')

# Use the binary directly if sqlite3 not available
print("\n=== Binary path ===")
run('docker exec moduforge ls /app/moduforge 2>&1')

print("\n=== Try using moduforge binary or go run ===")
run('docker exec moduforge sh -c "find / -name sqlite3 -type f 2>/dev/null | head -5"')

# Container logs with encoding fix
print("\n=== Container logs (last 30) ===")
run('docker logs moduforge --tail 30 2>&1')

# Check docker-compose.yml
print("\n=== docker-compose.yml ===")
run('cat /vol1/docker/docker-compose.yml 2>&1')

client.close()
