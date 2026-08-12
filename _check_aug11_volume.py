import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    if out: print(out)
    return out

# Check the Aug 11 volume
print("=== moduforge-source_moduforge_data ===")
run("sudo ls -la /vol1/docker/volumes/moduforge-source_moduforge_data/_data/ 2>/dev/null | head -20")

print("\n=== module.prop in source volume ===")
run("sudo cat /vol1/docker/volumes/moduforge-source_moduforge_data/_data/storage/projects/1785249992652501794-1864/module.prop 2>/dev/null || echo 'NOT FOUND'")

print("\n=== service.sh in source volume ===")
run("sudo grep -n 'system/bin' /vol1/docker/volumes/moduforge-source_moduforge_data/_data/storage/projects/1785249992652501794-1864/service.sh 2>/dev/null || echo 'NOT FOUND'")

print("\n=== customize.sh binary refs in source volume ===")
run("sudo grep -n 'system/bin' /vol1/docker/volumes/moduforge-source_moduforge_data/_data/storage/projects/1785249992652501794-1864/customize.sh 2>/dev/null || echo 'NOT FOUND'")

print("\n=== system/bin in source volume ===")
run("sudo ls -la /vol1/docker/volumes/moduforge-source_moduforge_data/_data/storage/projects/1785249992652501794-1864/system/bin/ 2>/dev/null | grep -E '(andromon|linucb|androwui)' || echo 'NOT FOUND'")

print("\n=== DB in source volume ===")
run("sudo ls -la /vol1/docker/volumes/moduforge-source_moduforge_data/_data/moduforge.db 2>/dev/null")

# Also check if the Aug 11 compose used a different project name
print("\n=== Was moduforge-source used from qwenpaw? ===")
run("sudo find /vol1/1000/docker/qwenpaw -name 'docker-compose*' -exec grep -l 'moduforge-source' {} \\; 2>/dev/null || echo 'no compose references moduforge-source'")

# Check if there's a .env or project name override
print("\n=== .env files ===")
run("sudo find /vol1/1000/docker/qwenpaw -name '.env' -path '*/ModuForge/*' -exec echo '--- {} ---' \\; -exec cat {} \\; 2>/dev/null || echo 'no .env'")

ssh.close()
