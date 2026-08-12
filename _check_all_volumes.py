import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    if out: print(out)
    return out

# Check each volume's module.prop
volumes = [
    "moduforge_moduforge_data",
    "moduforge-source_moduforge_data",
    "moduforge_deploy_moduforge_data",
    "moduforge_data",
    "moduforge_uploads",
]

for vol in volumes:
    print(f"\n=== Volume: {vol} ===")
    # List contents
    run(f"sudo ls /vol1/docker/volumes/{vol}/_data/storage/projects/ 2>/dev/null || echo '  (no storage/projects dir)'")
    # Check module.prop if exists
    run(f"sudo cat /vol1/docker/volumes/{vol}/_data/storage/projects/1785249992652501794-1864/module.prop 2>/dev/null || echo '  (no module.prop)'")
    # Check DB size
    run(f"sudo ls -la /vol1/docker/volumes/{vol}/_data/moduforge.db 2>/dev/null || echo '  (no moduforge.db)'")

# Also check the compose file location
print("\n=== docker-compose.yml ===")
run("sudo find /vol1/1000/docker/qwenpaw -name 'docker-compose.yml' -maxdepth 5 2>/dev/null | head -5")
run("sudo cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml 2>/dev/null | head -30")

ssh.close()
