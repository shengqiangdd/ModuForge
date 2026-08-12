import paramiko, json, time

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

def run(cmd):
    stdin, stdout, stderr = client.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace'), stderr.read().decode('utf-8', errors='replace')

# Check container status
print("=== Container status ===")
out, err = run("docker ps -a | grep moduforge")
print(out)

# Check container logs
print("\n=== Container logs ===")
out, err = run("docker logs moduforge --tail 30 2>&1")
# Filter out binary noise
lines = out.split('\n')
for line in lines:
    if any(x in line for x in ['Error', 'error', 'FATAL', 'fatal', 'panic', 'listen', 'bind', 'port', 'Starting', 'Started', 'health', 'migrate', 'seed', 'database', 'sqlite']):
        print(line[:200])

# Try to check if the port is actually listening
print("\n=== Port check ===")
out, err = run("docker exec moduforge sh -c 'wget -q -O /dev/null http://localhost:8080/health 2>&1 && echo OK || echo FAIL'")
print(f"Internal health: {out.strip()}")

# Check if port 8086 is accessible from host
print("\n=== Host port check ===")
out, err = run("curl -s -o /dev/null -w '%{http_code}' http://localhost:8086/health 2>&1")
print(f"Host health: {out.strip()}")

# Check the container's environment
print("\n=== Container env ===")
out, err = run("docker inspect moduforge --format='{{range .Config.Env}}{{println .}}{{end}}' | grep -E 'PORT|DB|DATA|MODE'")
print(out)

client.close()
