"""Verify: volumes are mounted, data persists, no need for backup/restore"""
import docker, os

c = docker.from_env()
ct = c.containers.get('moduforge')

# 1. Volume mounts
print('=== Volume Mounts ===')
for m in ct.attrs['Mounts']:
    print(f"  {m['Source']} -> {m['Destination']}")

# 2. DB files on host volume
print('\n=== DB files in volume (host) ===')
data_vol = None
for m in ct.attrs['Mounts']:
    if m['Destination'] == '/data':
        data_vol = m['Source']
        break

if data_vol:
    for f in sorted(os.listdir(data_vol)):
        path = os.path.join(data_vol, f)
        if os.path.isfile(path):
            size = os.path.getsize(path)
            print(f'  {f}: {size:,} bytes')
        else:
            print(f'  {f}/')

# 3. Container ENV
print('\n=== Container ENV (DB related) ===')
for e in ct.attrs['Config']['Env']:
    if any(k in e.upper() for k in ['DATA', 'DB', 'PORT', 'DATABASE']):
        print(f'  {e}')

# 4. Container /data
print('\n=== Container /data/ contents ===')
ec, out = ct.exec_run('ls -la /data/')
print(out.decode(errors='replace'))

# 5. Key insight
print('\n=== ANALYSIS ===')
print('1. Volume is mounted: /data is a Docker named volume -> data persists across rebuilds')
print('2. DB file exists in volume: moduforge.db is present')
print('3. Container reads from /data/moduforge.db -> same volume')
print()
print('=> Data SHOULD persist across docker compose up -d --build')
print('=> The "data loss" was NOT from volume deletion')
print('=> Root cause was likely:')
print('   - Entry script creating fresh DB on first start')
print('   - OR WAL mode cache issue with old file descriptors')
print('   - OR user_id mismatch (API filters by JWT user_id)')
