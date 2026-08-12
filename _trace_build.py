import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    if out: print(out)
    return out

# Check the build records in the CURRENT database
print("=== Current DB: builds table ===")
run("sudo docker exec moduforge sqlite3 /data/moduforge.db 'SELECT id, project_id, status, created_at FROM builds ORDER BY created_at DESC LIMIT 10;' 2>/dev/null || echo 'sqlite3 not available'")

# Check if sqlite3 is available, if not use strings/grep
print("\n=== Check builds via strings ===")
run("sudo docker exec moduforge strings /data/moduforge.db | grep -i '2026081' | head -10")

# Check module.prop in builds directory
print("\n=== Build output files ===")
run("sudo docker exec moduforge find /data/builds -name 'module.prop' -exec echo '--- {} ---' \\; -exec cat {} \\; 2>/dev/null | head -30 || echo 'no builds dir'")

# Check all module.prop across ALL project dirs in current volume
print("\n=== All module.prop in current volume ===")
run("sudo docker exec moduforge find /data -name 'module.prop' -exec echo '--- {} ---' \\; -exec cat {} \\; 2>/dev/null")

# Check the old volume (moduforge_data) for module.prop
print("\n=== Old volume module.prop ===")
run("sudo find /vol1/docker/volumes/moduforge_data/_data -name 'module.prop' -exec echo '--- {} ---' \\; -exec cat {} \\; 2>/dev/null | head -30")

# Check build artifacts zip
print("\n=== output.zip in current volume ===")
run("sudo docker exec moduforge unzip -p /data/storage/projects/output.zip module.prop 2>/dev/null || echo 'no output.zip or no module.prop in zip'")

# Check old volume output.zip
print("\n=== output.zip in old volume ===")
run("sudo unzip -p /vol1/docker/volumes/moduforge_data/_data/storage/projects/output.zip module.prop 2>/dev/null || echo 'no output.zip in old volume'")

# Check if there are other compose files
print("\n=== All docker-compose.yml on server ===")
run("sudo find / -name 'docker-compose.yml' -path '*/ModuForge/*' 2>/dev/null | head -10")

ssh.close()
