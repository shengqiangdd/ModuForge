import paramiko, json

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# 1. Read docker-compose.yml
print("=== docker-compose.yml ===")
out, err = run("cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml")
print(out)

# 2. Read Dockerfile
print("\n=== Dockerfile ===")
out, err = run("cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/Dockerfile 2>/dev/null || echo 'NOT FOUND'")
print(out[:2000])

# 3. Read entrypoint script
print("\n=== Entrypoint script ===")
out, err = run("docker exec moduforge cat /docker-entrypoint.sh 2>/dev/null || echo 'NOT FOUND'")
print(out[:2000])

# 4. Check if there's a .env file
print("\n=== .env file ===")
out, err = run("cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/.env 2>/dev/null || echo 'NOT FOUND'")
print(out)

# 5. Check what happens during container init - look at the binary's init behavior
print("\n=== Binary strings: init/seed/migrate ===")
out, err = run("docker exec moduforge sh -c 'strings /server 2>/dev/null | grep -iE \"init.*db|seed|migrate|create.*table|auto.?migrat|first.?run\" | head -20'")
print(out)

# 6. Check if there's a MODUFORGE_DEV flag behavior
print("\n=== MODUFORGE_DEV references ===")
out, err = run("docker exec moduforge sh -c 'strings /server 2>/dev/null | grep -i MODUFORGE_DEV | head -10'")
print(out)

# 7. Check the actual database initialization in source code
print("\n=== Source code: database init ===")
out, err = run("find /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend -name '*.go' -exec grep -l 'AutoMigrate\\|CreateTable\\|initDB\\|InitDB\\|seedData\\|SeedData' {} \\; 2>/dev/null")
print(f"Files with init: {out}")

# 8. Check the main.go or cmd entry point
print("\n=== main.go / cmd ===")
out, err = run("find /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend -name 'main.go' -o -name 'cmd.go' 2>/dev/null | head -5")
print(f"Entry files: {out}")

client.close()
