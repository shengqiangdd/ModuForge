#!/usr/bin/env python3
"""
ModuForge 部署脚本 - 带 MD 提示词系统
在本地构建，然后通过 SSH 复制到 Docker 容器
"""

import paramiko
import sys
import os
import subprocess
import time

# Windows 编码修复
if sys.platform == 'win32':
    os.system('chcp 65001 >nul 2>&1')
    sys.stdout.reconfigure(encoding='utf-8')
    sys.stderr.reconfigure(encoding='utf-8')

# 配置
SERVER_HOST = "192.168.2.9"
SERVER_PORT = 22
SERVER_USER = "admin"
SERVER_PASS = "csq0216"
CONTAINER_NAME = "moduforge-app-1"

# 本地项目路径
LOCAL_PROJECT = os.path.dirname(os.path.abspath(__file__))
LOCAL_BACKEND = os.path.join(LOCAL_PROJECT, "backend")

def run_local(cmd, cwd=None):
    """执行本地命令"""
    print(f"  >> {cmd}")
    result = subprocess.run(cmd, shell=True, cwd=cwd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"  [ERROR] {result.stderr}")
    return result.returncode, result.stdout, result.stderr

def create_ssh_client(host, port, user, password):
    """创建 SSH 客户端连接"""
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(host, port=port, username=user, password=password, timeout=15)
    return client

def execute_remote(client, command, timeout=60):
    """执行远程命令"""
    print(f"  >> {command}")
    stdin, stdout, stderr = client.exec_command(command, timeout=timeout)
    exit_code = stdout.channel.recv_exit_status()
    out = stdout.read().decode('utf-8', errors='ignore')
    err = stderr.read().decode('utf-8', errors='ignore')
    return exit_code, out, err

def sftp_put(client, local_path, remote_path):
    """通过 SFTP 上传文件"""
    sftp = client.open_sftp()
    try:
        sftp.put(local_path, remote_path)
        return True
    except Exception as e:
        print(f"  [ERROR] SFTP 上传失败: {e}")
        return False
    finally:
        sftp.close()

def deploy():
    """主部署流程"""
    print("=" * 60)
    print("ModuForge 部署脚本 (带 MD 提示词系统)")
    print("=" * 60)

    # Step 1: 构建 Linux 二进制
    print("\n[1/7] 构建 Linux 二进制...")
    local_binary = os.path.join(LOCAL_BACKEND, "cmd", "moduforge", "moduforge")
    
    # 交叉编译为 Linux AMD64
    env = os.environ.copy()
    env['CGO_ENABLED'] = '1'
    env['GOOS'] = 'linux'
    env['GOARCH'] = 'amd64'
    
    code, out, err = run_local(
        f"go build -o {local_binary} ./cmd/moduforge/",
        cwd=LOCAL_BACKEND
    )
    
    if code != 0:
        print(f"  [ERROR] 构建失败: {err}")
        return False
    
    # 检查文件大小
    if os.path.exists(local_binary):
        size_mb = os.path.getsize(local_binary) / (1024 * 1024)
        print(f"  [OK] 构建成功: {local_binary} ({size_mb:.1f} MB)")
    else:
        print("  [ERROR] 二进制文件不存在")
        return False

    # Step 2: 检查 MD 文件
    print("\n[2/7] 检查 MD 提示词文件...")
    prompts_dir = os.path.join(LOCAL_BACKEND, "internal", "agent", "prompts")
    md_files = [f for f in os.listdir(prompts_dir) if f.endswith('.md')]
    print(f"  [OK] 找到 {len(md_files)} 个 MD 文件: {', '.join(md_files)}")

    # Step 3: 连接服务器
    print("\n[3/7] 连接服务器...")
    try:
        client = create_ssh_client(SERVER_HOST, SERVER_PORT, SERVER_USER, SERVER_PASS)
        print("  [OK] 连接成功")
    except Exception as e:
        print(f"  [ERROR] 连接失败: {e}")
        return False

    # Step 4: 上传后端二进制
    print("\n[4/7] 上传后端二进制...")
    remote_tmp = "/tmp/moduforge_new"
    if not sftp_put(client, local_binary, remote_tmp):
        client.close()
        return False
    
    # 设置执行权限并复制到容器
    code, out, err = execute_remote(client, f"chmod +x {remote_tmp} && docker cp {remote_tmp} {CONTAINER_NAME}:/app/server && rm {remote_tmp}")
    if code != 0:
        print(f"  [ERROR] 复制到容器失败: {err}")
        client.close()
        return False
    print("  [OK] 后端上传成功")

    # Step 5: 上传 MD 提示词文件
    print("\n[5/7] 上传 MD 提示词文件...")
    remote_prompts_dir = "/tmp/prompts_upload"
    execute_remote(client, f"mkdir -p {remote_prompts_dir}")
    
    # 上传每个 MD 文件
    for md_file in md_files:
        local_file = os.path.join(prompts_dir, md_file)
        remote_file = f"{remote_prompts_dir}/{md_file}"
        if not sftp_put(client, local_file, remote_file):
            print(f"  [WARN] 上传 {md_file} 失败")
            continue
        print(f"  [OK] 上传 {md_file}")
    
    # 复制到容器
    code, out, err = execute_remote(client, f"docker cp {remote_prompts_dir}/. {CONTAINER_NAME}:/app/prompts/ && rm -rf {remote_prompts_dir}")
    if code != 0:
        print(f"  [WARN] 提示词文件复制可能有问题: {err}")
    else:
        print("  [OK] MD 提示词文件上传成功")

    # Step 6: 重启容器
    print("\n[6/7] 重启容器...")
    code, out, err = execute_remote(client, f"docker restart {CONTAINER_NAME}")
    if code != 0:
        print(f"  [ERROR] 重启失败: {err}")
        client.close()
        return False
    print("  [OK] 重启成功")

    # 等待容器启动
    print("\n等待容器启动...")
    time.sleep(8)

    # Step 7: 验证
    print("\n[7/7] 验证服务...")
    code, out, err = execute_remote(client, f"docker ps --filter name={CONTAINER_NAME} --format '{{{{.Status}}}}'")
    print(f"  容器状态: {out.strip()}")
    
    # 测试健康检查
    code, out, err = execute_remote(client, f"curl -s http://localhost:8086/health 2>&1")
    if "ok" in out.lower() or "healthy" in out.lower():
        print(f"  [OK] 健康检查通过: {out.strip()}")
    else:
        print(f"  [WARN] 健康检查: {out.strip()}")

    client.close()

    print("\n" + "=" * 60)
    print("[OK] 部署完成!")
    print("=" * 60)
    print("\n提示词系统已更新:")
    print("  - base.md: 基础角色定义")
    print("  - act.md: Act 模式提示词")
    print("  - plan.md: Plan 模式提示词")
    print("  - free.md: 免费模型提示词")
    print("  - tools.md: 工具参考")
    print("  - errors.md: 错误处理参考")
    print("\n修改提示词只需编辑对应的 MD 文件，然后重新部署即可。")
    return True

if __name__ == "__main__":
    success = deploy()
    sys.exit(0 if success else 1)
