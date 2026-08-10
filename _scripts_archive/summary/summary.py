import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8', errors='replace')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd, timeout=30):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode(errors='replace')
    err = stderr.read().decode(errors='replace')
    if out: print(out)
    if err: print(err)
    return out

SUDO = 'echo "csq0216" | sudo -S'

# Summary of recovered data
print('=== RECOVERED DATA SUMMARY ===')
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT username, role FROM users;"')
print()
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT id, name FROM projects WHERE name IS NOT NULL AND name != \'\';"')
print()
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT name FROM sqlite_master WHERE type=\'table\' ORDER BY name;"')
print()
run(SUDO + ' docker exec moduforge sqlite3 /data/moduforge.db "SELECT COUNT(*) as total_users FROM users; SELECT COUNT(*) as total_projects FROM projects; SELECT COUNT(*) as total_files FROM project_files;"')

# Check health
print('\n=== HEALTH CHECK ===')
run(SUDO + ' docker inspect moduforge --format "{{.State.Health.Status}}"')

ssh.close()
