import paramiko
host='192.168.2.9'
user='admin'
pw='csq0216'
ssh=paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=pw, timeout=15)

# chmod should work even without chown
cmds = [
    "docker exec moduforge sh -lc 'chmod 666 /data/moduforge.db /data/moduforge.db-wal /data/moduforge.db-shm && echo CHMOD_OK || echo CHMOD_FAIL'",
    "docker exec moduforge sh -lc 'ls -ld /data/moduforge.db /data/moduforge.db-wal /data/moduforge.db-shm'",
]
for c in cmds:
    _,stdout,stderr = ssh.exec_command(c)
    out = stdout.read().decode('utf-8','ignore')
    err = stderr.read().decode('utf-8','ignore')
    print('CMD:', c)
    print('OUT:', out)
    if err: print('ERR:', err)
    print('---')
ssh.close()
