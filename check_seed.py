import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Read SeedAdminUser function
print("=== SeedAdminUser ===")
out, err = run("sed -n '1051,1110p' /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/database/sqlite.go")
print(out)

# Read SeedMarketData function
print("\n=== SeedMarketData ===")
out, err = run("sed -n '1105,1160p' /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/internal/database/sqlite.go")
print(out)

# Read main.go to see the full init flow
print("\n=== main.go ===")
out, err = run("cat /vol1/1000/docker/qwenpaw/data/working/workspaces/default/ModuForge/backend/cmd/moduforge/main.go")
print(out)

client.close()
