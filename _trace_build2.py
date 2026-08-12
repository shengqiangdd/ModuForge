import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=15)

def run(cmd, timeout=15):
    _, o, e = ssh.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors='replace').strip()
    if out: print(out)
    return out

# Install sqlite3 in container temporarily
print("=== Installing sqlite3 ===")
run("sudo docker exec moduforge apk add --no-cache sqlite 2>/dev/null || echo 'already installed or failed'")

# Now query builds
print("\n=== Builds in current DB ===")
run("sudo docker exec moduforge sqlite3 /data/moduforge.db \"SELECT id, project_id, substr(status,1,20), created_at, updated_at FROM builds ORDER BY created_at DESC LIMIT 10;\"")

# Check if there are build artifacts with version info
print("\n=== Build artifacts ===")
run("sudo docker exec moduforge find /data/builds -type f 2>/dev/null | head -20 || echo 'no /data/builds'")

# Check the ModuForge compose file (different from qwenpaw one)
print("\n=== ModuForge compose volumes ===")
run("sudo cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/docker-compose.yml 2>/dev/null | grep -A3 volumes")

# Check if there are multiple compose projects running
print("\n=== All running containers with moduforge ===")
run("sudo docker ps -a --filter 'name=modu' --format '{{.Names}} {{.Status}} {{.Image}}'")

# Check the compose project for the qwenpaw workspace compose
print("\n=== qwenpaw compose project ===")
run("sudo docker inspect moduforge --format='Project: {{index .Config.Labels \"com.docker.compose.project\"}} ConfigFiles: {{index .Config.Labels \"com.docker.compose.project.config_files\"}}'")

ssh.close()
