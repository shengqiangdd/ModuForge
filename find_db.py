import paramiko
host='192.168.2.9'
user='admin'
pw='csq0216'
ssh=paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=pw, timeout=15)
cmds=[
    "docker exec moduforge sh -lc 'find /app -name moduforge.db -maxdepth 4 || true'",
    "docker exec moduforge sh -lc 'ls -ld /app/data /app/backend/data /data || true'",
    "docker exec moduforge sh -lc 'stat -c %U:%G %a /app/data /app/backend/data /data || true'",
]
for c in cmds:
    _,stdout,stderr=ssh.exec_command(c)
    out=stdout.read().decode('utf-8','ignore')
    err=stderr.read().decode('utf-8','ignore')
    print('CMD:',c)
    print(out)
    print(err)
    print('---')
ssh.close()
