#!/usr/bin/env python3
import paramiko
import sys

sys.stdout.reconfigure(encoding='utf-8')

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd)
    return stdout.read().decode('utf-8', errors='replace').strip()

print("=== Full ZIP Contents ===")
zip_list = run('python3 -c "import zipfile; z=zipfile.ZipFile(\'/tmp/test_export.zip\'); [print(n) for n in sorted(z.namelist())]"')
print(zip_list)

print("\n=== Analysis ===")
lines = [n for n in zip_list.split('\n') if n.strip()]
has_webroot = any('webroot/' in n for n in lines)
has_tmp = any(n.startswith('tmp/') for n in lines)
has_design = any('DESIGN_DOC' in n for n in lines)
has_backend = any('backend/' in n for n in lines)
has_upload = any(n == 'upload' for n in lines)
has_module_prop = any('module.prop' in n for n in lines)
has_service_sh = any('service.sh' in n for n in lines)
has_customize_sh = any('customize.sh' in n for n in lines)
has_system_bin = any('system/bin/' in n for n in lines)

print(f"Has webroot wrapper: {'YES' if has_webroot else 'NO'}")
print(f"Excludes tmp/: {'YES' if not has_tmp else 'NO'}")
print(f"Excludes DESIGN_DOC.md: {'YES' if not has_design else 'NO'}")
print(f"Excludes backend/: {'YES' if not has_backend else 'NO'}")
print(f"Excludes upload: {'YES' if not has_upload else 'NO'}")
print(f"Has module.prop: {'YES' if has_module_prop else 'NO'}")
print(f"Has service.sh: {'YES' if has_service_sh else 'NO'}")
print(f"Has customize.sh: {'YES' if has_customize_sh else 'NO'}")
print(f"Has system/bin/: {'YES' if has_system_bin else 'NO'}")

print("\n=== Frontend files in webroot/ ===")
frontend_files = [n for n in lines if n.startswith('webroot/')]
for f in frontend_files:
    print(f"  {f}")

print("\n=== Other files ===")
other_files = [n for n in lines if not n.startswith('webroot/') and not n.startswith('META-INF/')]
for f in other_files:
    print(f"  {f}")

ssh.close()
