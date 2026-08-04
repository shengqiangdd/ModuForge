#!/usr/bin/env python3
"""
ModuForge 部署脚本
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
LOCAL_FRONTEND = os.path.join(LOCAL_PROJECT, "frontend")

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
    print("=" * 50)
    print("ModuForge 部署脚本")
    print("=" * 50)

    # Step 1: 使用已有的 Linux 二进制
    print("\n[1/6] 检查后端二进制...")
    local_binary = os.path.join(LOCAL_BACKEND, "cmd", "moduforge", "moduforge")
    if not os.path.exists(local_binary):
        print("  [ERROR] 找不到后端二进制文件")
        return False
    print(f"  [OK] 找到二进制: {local_binary}")

    # Step 2: 检查前端构建
    print("\n[2/6] 检查前端构建...")
    dist_dir = os.path.join(LOCAL_FRONTEND, "dist")
    if not os.path.exists(dist_dir):
        print("  [ERROR] 前端 dist 目录不存在")
        return False
    print(f"  [OK] 找到前端构建: {dist_dir}")

    # Step 3: 连接服务器
    print("\n[3/6] 连接服务器...")
    client = create_ssh_client(SERVER_HOST, SERVER_PORT, SERVER_USER, SERVER_PASS)
    print("  [OK] 连接成功")

    # Step 4: 上传后端二进制
    print("\n[4/6] 上传后端二进制...")
    if not os.path.exists(local_binary):
        print("  [ERROR] 找不到构建产物")
        client.close()
        return False
    
    # 上传到 /tmp，然后 docker cp
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

    # Step 5: 上传前端
    print("\n[5/6] 上传前端...")
    dist_dir = os.path.join(LOCAL_FRONTEND, "dist")
    if not os.path.exists(dist_dir):
        print("  [WARN] 前端 dist 目录不存在，跳过")
    else:
        # 上传 dist 目录中的文件
        for root, dirs, files in os.walk(dist_dir):
            for file in files:
                local_file = os.path.join(root, file)
                rel_path = os.path.relpath(local_file, dist_dir)
                remote_file = f"/tmp/dist_upload/{rel_path}"
                
                # 创建远程目录
                remote_dir = os.path.dirname(remote_file)
                execute_remote(client, f"mkdir -p {remote_dir}")
                
                sftp_put(client, local_file, remote_file)
        
        # 复制到容器
        code, out, err = execute_remote(client, f"docker cp /tmp/dist_upload/. {CONTAINER_NAME}:/app/dist/ && rm -rf /tmp/dist_upload")
        if code != 0:
            print(f"  [WARN] 前端复制可能有问题: {err}")
        else:
            print("  [OK] 前端上传成功")

    # Step 6: 重启容器
    print("\n[6/6] 重启容器...")
    code, out, err = execute_remote(client, f"docker restart {CONTAINER_NAME}")
    if code != 0:
        print(f"  [ERROR] 重启失败: {err}")
        client.close()
        return False
    print("  [OK] 重启成功")

    # 等待容器启动
    print("\n等待容器启动...")
    time.sleep(5)

    # 检查容器状态
    code, out, err = execute_remote(client, f"docker ps --filter name={CONTAINER_NAME} --format '{{{{.Status}}}}'")
    print(f"  状态: {out.strip()}")

    client.close()

    print("\n" + "=" * 50)
    print("[OK] 部署完成!")
    print("=" * 50)
    return True

if __name__ == "__main__":
    success = deploy()
    sys.exit(0 if success else 1)
