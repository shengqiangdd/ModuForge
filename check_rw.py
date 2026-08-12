import paramiko, textwrap
host='192.168.2.9'
user='admin'
pw='csq0216'
ssh=paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(host, username=user, password=pw, timeout=15)
sftp=ssh.open_sftp()
script=textwrap.dedent('''
import subprocess
for cmd in [
    "docker exec moduforge sh -lc 'ls -ld /data /data/moduforge.db /data/moduforge.db-wal'",
    "docker exec moduforge sh -lc 'stat -c %U:%G %a /data /data/moduforge.db /data/moduforge.db-wal || true'",
    "docker exec moduforge sh -lc 'touch /data/.write_test && rm -f /data/.write_test && echo WRITABLE || echo NOT_WRITABLE'",
]:
    p=subprocess.run(cmd,shell=True,capture_output=True,text=True)
    print("CMD:",cmd)
    print(p.stdout)
    if p.stderr: print(p.stderr)
    print("---")
''')
with sftp.open('/tmp/check_rw.py','w') as f:
    f.write(script)
stdin,stdout,stderr=ssh.exec_command('python3 /tmp/check_rw.py')
print(stdout.read().decode('utf-8','ignore'))
print(stderr.read().decode('utf-8','ignore'))
sftp.close()
ssh.close()
