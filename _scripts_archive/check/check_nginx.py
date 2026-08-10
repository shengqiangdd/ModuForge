# -*- coding: utf-8 -*-
import paramiko, time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=30)
    return o.read().decode().strip() or e.read().decode().strip()

# Check nginx config
print("=== Nginx config ===")
print(run("docker exec moduforge-nginx cat /etc/nginx/conf.d/default.conf 2>/dev/null || docker exec moduforge-nginx cat /etc/nginx/nginx.conf 2>/dev/null"))

# Check nginx ports
print("\n=== Nginx ports ===")
print(run("docker inspect moduforge-nginx --format '{{json .NetworkSettings.Ports}}'"))

# Check what nginx proxies to
print("\n=== Nginx upstream ===")
print(run("docker exec moduforge-nginx grep -r proxy_pass /etc/nginx/ 2>/dev/null"))

ssh.close()
