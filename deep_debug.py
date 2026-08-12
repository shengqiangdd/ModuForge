import paramiko, json, time, os

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# 1. Find docker-compose.yml
print("=== Find docker-compose.yml ===")
out, err = run('find /vol1 -name "docker-compose*" -path "*moduforge*" 2>/dev/null')
print(out)
out, err = run('find /vol1 -name "docker-compose*" -path "*ModuForge*" 2>/dev/null')
print(out)

# Also check where the container was created from
print("\n=== Container label com.docker.compose.project ===")
out, err = run('docker inspect moduforge --format="{{index .Config.Labels \"com.docker.compose.project.working_dir\"}}" 2>&1')
print(f"Working dir: {out.strip()}")
out, err = run('docker inspect moduforge --format="{{index .Config.Labels \"com.docker.compose.project.config_files\"}}" 2>&1')
print(f"Config files: {out.strip()}")
out, err = run('docker inspect moduforge --format="{{index .Config.Labels \"com.docker.compose.project\"}}" 2>&1')
print(f"Project: {out.strip()}")

# 2. Check for WAL/SHM files RIGHT NOW
print("\n=== WAL/SHM files ===")
out, err = run('ls -la /vol1/docker/volumes/moduforge_moduforge_data/_data/moduforge.db*')
print(out)

# 3. Check ALL moduforge volumes for DB files
print("\n=== Check all moduforge volumes ===")
for vol in ['moduforge_data', 'moduforge_deploy_moduforge_data', 'moduforge_moduforge_data']:
    out, err = run(f'ls -la /vol1/docker/volumes/{vol}/_data/moduforge.db* 2>/dev/null')
    if out.strip():
        print(f"\n{vol}:")
        print(out)

# 4. Check if there are any bind mounts
print("\n=== Check for bind mounts in docker inspect ===")
out, err = run('docker inspect moduforge --format="{{json .HostConfig.Binds}}"')
print(f"Binds: {out.strip()}")

# 5. Read the .env file inside the data dir
print("\n=== .env in data dir ===")
out, err = run('cat /vol1/docker/volumes/moduforge_moduforge_data/_data/.env 2>/dev/null')
print(out)

# 6. Check the actual DB content from inside container
print("\n=== Install sqlite3 as root ===")
out, err = run('docker exec -u root moduforge apk add --no-cache sqlite 2>&1 | tail -5')
print(out)

if 'OK' in out or 'already installed' in out:
    print("\n=== Query DB from inside container ===")
    out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT COUNT(*) as cnt FROM projects;\\""')
    print(f"Projects count: {out.strip()}")
    out, err = run('docker exec moduforge sh -c "sqlite3 /data/moduforge.db \\"SELECT id, name, user_id FROM projects LIMIT 5;\\""')
    print(f"Projects: {out}")

# 7. Check if binary has its own DB path
print("\n=== Check binary for embedded paths ===")
out, err = run('docker exec moduforge sh -c "strings /app/moduforge 2>/dev/null | grep -i \\\\.db | head -10"')
print(out)

# 8. Check the container process
print("\n=== Container process ===")
out, err = run('docker exec moduforge ps aux 2>&1 || docker top moduforge 2>&1')
print(out)

client.close()
