import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    if out: print(out)
    return out

# Full compose file from qwenpaw workspace
print("=== Full qwenpaw compose file ===")
run("sudo cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml")

# Also check the OTHER compose file in ModuForge dir
print("\n=== ModuForge dir compose file ===")
run("sudo find /vol1/1000/docker/qwenpaw -name 'docker-compose.yml' -exec echo '--- {} ---' \\; -exec cat {} \\; 2>/dev/null")

# Check if the ModuForge dir compose was ever used (check for its volume)
print("\n=== Was moduforge-source ever used? ===")
run("sudo docker volume inspect moduforge-source_moduforge_data --format '{{.CreatedAt}}' 2>/dev/null || echo 'volume not found'")

# Check volume creation dates
print("\n=== Volume creation dates ===")
for vol in ["moduforge_moduforge_data", "moduforge_data", "moduforge-source_moduforge_data", "moduforge_deploy_moduforge_data"]:
    run(f"sudo docker volume inspect {vol} --format '{vol}: created={{.CreatedAt}}' 2>/dev/null || echo '{vol}: not found'")

# The real question: where did 20260811 come from?
# Check the output.zip date more carefully
print("\n=== output.zip details ===")
run("sudo docker exec moduforge ls -la /data/storage/projects/output.zip")
run("sudo docker exec moduforge unzip -l /data/storage/projects/output.zip | head -5")

# Check if there's a build system that generates the zip with version from module.prop
print("\n=== Build system: where does version come from? ===")
run("sudo docker exec moduforge grep -rn 'versionCode' /app/backend/internal/service/ 2>/dev/null | head -10")

ssh.close()
