import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    if out: print(out)
    return out

# 1. Check container mount points
print("=== Container mounts ===")
run("sudo docker inspect moduforge --format='{{range .Mounts}}{{.Type}} {{.Source}} -> {{.Destination}} ({{.RW}}){{println}}{{end}}'")

# 2. List all docker volumes
print("\n=== All docker volumes ===")
run("sudo docker volume ls")

# 3. Check which volume the compose project uses
print("\n=== Compose project name ===")
run("sudo docker inspect moduforge --format='{{index .Config.Labels \"com.docker.compose.project\"}}'")
run("sudo docker inspect moduforge --format='{{index .Config.Labels \"com.docker.compose.project.config_files\"}}'")

# 4. Check the actual data in the mounted volume
print("\n=== module.prop in current mount ===")
run("sudo docker exec moduforge cat /data/storage/projects/1785249992652501794-1864/module.prop")

# 5. Check if there's another project dir with different version
print("\n=== Other project dirs ===")
run("sudo docker exec moduforge find /data/storage/projects -maxdepth 2 -name 'module.prop' -exec echo '--- {} ---' \\; -exec cat {} \\;")

# 6. Check docker-compose.yml mount config
print("\n=== docker-compose.yml volumes section ===")
run("sudo docker exec moduforge cat /app/docker-compose.yml 2>/dev/null | grep -A5 volumes || echo 'no compose in container'")
# Try host side
run("cat /opt/moduforge/docker-compose.yml 2>/dev/null | grep -A10 volumes || echo 'checking other paths...'")
run("find /home/admin -name 'docker-compose.yml' -maxdepth 3 2>/dev/null | head -5")

ssh.close()
