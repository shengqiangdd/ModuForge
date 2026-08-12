"""Debug: test actual write capability inside container"""
import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

# Test write inside container
print('=== Test write in container ===')
cmds = [
    # Who is running?
    'docker exec moduforge id 2>&1 || echo "container not running"',
    # Check file perms
    'docker exec moduforge ls -la /data/moduforge.db 2>&1',
    # Test direct write
    'docker exec moduforge sh -c "echo test > /data/test_write.txt && cat /data/test_write.txt && rm /data/test_write.txt" 2>&1',
    # Test SQLite write
    'docker exec moduforge sh -c "python3 -c \\\"import sqlite3; c=sqlite3.connect(\\\\\\\"/data/moduforge.db\\\\\\\"); c.execute(\\\\\\\"SELECT 1\\\\\\\"); print(\\\\\\\"read OK\\\\\\\")\\\" 2>&1" 2>&1',
    # Check if there's a readonly mount
    'docker inspect moduforge --format="{{range .Mounts}}{{.Type}} {{.Source}} -> {{.Destination}} (RW={{.RW}}){{println}}{{end}}" 2>&1',
    # Check Go binary's DB path
    'docker exec moduforge env | grep -i "db\\|data\\|path" 2>&1',
    # Try to write DB directly
    'docker exec moduforge sh -c "cp /data/moduforge.db /data/moduforge.db.test && ls -la /data/moduforge.db.test && rm /data/moduforge.db.test" 2>&1',
]

for cmd in cmds:
    print(f'\n>>> {cmd[:80]}')
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=10)
    out = stdout.read().decode(errors='replace')
    err = stderr.read().decode(errors='replace')
    if out: print(out)
    if err: print(f'ERR: {err}')

ssh.close()
