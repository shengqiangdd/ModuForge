import paramiko, json

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    err = e.read().decode(errors='replace').strip()
    if out: print(out[:3000])
    if err: print(f"ERR: {err[:500]}")
    return out

# Check database directly
print("=== DB projects ===")
run("sudo docker exec moduforge sqlite3 /data/moduforge.db \"SELECT id, name FROM projects LIMIT 10;\"")

print("\n=== DB files for AndroBoost project ===")
run("sudo docker exec moduforge sqlite3 /data/moduforge.db \"SELECT id, name, path FROM files WHERE project_id LIKE '%1864%' LIMIT 50;\"")

# Also check the storage directory
print("\n=== Storage directory ===")
run("sudo docker exec moduforge ls -la /data/storage/projects/ 2>/dev/null || echo 'no storage dir'")

# Check if the project exists in storage
print("\n=== Project storage ===")
run("sudo docker exec moduforge find /data/storage/projects/ -maxdepth 2 -type f -name '*.sh' -o -name 'module.prop' 2>/dev/null | head -20")

ssh.close()
