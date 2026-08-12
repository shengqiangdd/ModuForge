import paramiko

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=15)

cmds = [
    'docker inspect moduforge:latest --format "{{.RepoDigests}}"',
    'docker history moduforge:latest --no-trunc --format "{{.CreatedBy}}" | head -n 20',
    'docker run --rm --entrypoint sh moduforge:latest -lc "ls -l /server; ls -l /app/server || true; file /server || true; file /app/server || true"',
]

for c in cmds:
    print('\n>', c)
    _, o, e = ssh.exec_command(c, timeout=60)
    out = o.read().decode(errors='replace')
    err = e.read().decode(errors='replace')
    if out.strip():
        print(out.strip()[:4000])
    if err.strip():
        print('ERR:', err.strip()[:2000])

ssh.close()
