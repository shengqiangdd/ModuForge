#!/usr/bin/env python3
import urllib.request, paramiko, sys
sys.stdout.reconfigure(encoding='utf-8')

# 1. Server-side checks
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)
def run(cmd):
    _, o, e = ssh.exec_command(cmd, timeout=25)
    out = o.read().decode('utf-8', errors='replace').strip()
    err = e.read().decode('utf-8', errors='replace').strip()
    return out if out else ('ERR: ' + err if err else '')

print("=== new bundle names ===")
print(run("docker exec moduforge sh -c 'ls -la /app/dist/assets/ | grep -E \"index|svelte\"'"))
print(run("docker exec moduforge sh -c 'cat /app/dist/index.html | grep -oE \"(index|svelte)-[A-Za-z0-9_-]+\\.js\" | sort -u'"))
print("=== sw.js cache name ===")
print(run("docker exec moduforge sh -c 'grep -o \"moduforge-v[0-9]\" /app/dist/sw.js'"))
print("=== bundle contains v15 badge & auto-scroll? ===")
print(run("docker exec moduforge sh -c 'grep -c \"UI v15\" /app/dist/assets/svelte-*.js'"))
print(run("docker exec moduforge sh -c 'grep -c \"stickToBottom\" /app/dist/assets/svelte-*.js'"))
print(run("docker exec moduforge sh -c 'grep -c \"sw.js?v=15\" /app/dist/assets/index-*.js'"))
print("=== container status ===")
print(run("docker ps --filter name=moduforge --format '{{.Status}}'"))

ssh.close()

# 2. HTTP header checks
print("\n=== HTTP headers ===")
BASE = 'http://192.168.2.9:8086'
for path in ['/sw.js', '/index.html', '/']:
    try:
        req = urllib.request.Request(f'{BASE}{path}', method='GET')
        r = urllib.request.urlopen(req, timeout=10)
        print(f'--- {path} ---')
        for h in ['cache-control', 'content-type']:
            print(f'  {h}: {r.headers.get(h)}')
    except Exception as e:
        print(f'--- {path} --- ERROR: {e}')
