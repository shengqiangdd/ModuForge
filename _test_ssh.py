import paramiko

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.2.9', 22, 'admin', 'csq0216', timeout=10)
print('Connected!')
_, o, e = c.exec_command('echo OK')
print(o.read().decode().strip())
c.close()
