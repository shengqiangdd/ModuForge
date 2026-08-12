import paramiko
host='192.168.2.9'
user='admin'
pw='csq0216'
ssh=paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=pw, timeout=15)

# Use triple-quoted raw string to avoid any quoting issues in remote script
script = r"""import subprocess

cmds = [
    "docker exec moduforge sh -lc 'ls -ld /data/moduforge.db /data/moduforge.db-wal /data/moduforge.db-shm || true'",
    "docker exec moduforge sh -lc 'id || true'",
    "docker exec moduforge sh -lc 'whoami || true'",
]
for c in cmds:
    p = subprocess.run(c, shell=True, capture_output=True, text=True)
    print("CMD:", c)
    print(p.stdout)
    if p.stderr: print(p.stderr)
    print("---")

# Check write test
p = subprocess.run(
    "docker exec moduforge sh -lc 'echo test|write_test>/data/.write_test&&cat /data/.write_test||FAIL'",
    shell=True, capture_output=True, text=True
)
print("WRITE_TEST:", p.stdout.strip(), p.stderr.strip())
"""

sftp=ssh.open_sftp()
with sftp.open('/tmp/fix_db.py','w') as f:
    f.write(script)
stdin,stdout,stderr=ssh.exec_command('python3 /tmp/fix_db.py')
print(stdout.read().decode('utf-8','ignore'))
print(stderr.read().decode('utf-8','ignore'))
sftp.close()
ssh.close()
