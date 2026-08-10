#!/usr/bin/env python3
"""发送 Agent 任务修复 main.rs 模块集成"""
import sys
import io
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

import paramiko
import time
import json

HOST = "192.168.2.9"
USER = "admin"
PASSWORD = "csq0216"

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(HOST, username=USER, password=PASSWORD, timeout=30)
    
    def run(cmd, timeout=30):
        stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout*1000)
        return stdout.read().decode(), stderr.read().decode()
    
    # 1. 检查 ModuForge 登录
    print("=== 1. 检查 ModuForge 登录 ===")
    out, _ = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"csq","password":"csq0216"}\'')
    print(out[:200])
    
    # 2. 获取 token
    print("\n=== 2. 获取 Token ===")
    out, _ = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"csq","password":"csq0216"}\' | python -c "import sys, json; print(json.load(sys.stdin).get(\'token\',\'\'))"')
    token = out.strip()
    print(f"Token: {token[:50]}...")
    
    # 3. 发送 Agent 任务
    print("\n=== 3. 发送 Agent 任务 ===")
    task = """请修复 AndroBoost-SmartTune 项目的 main.rs 模块集成问题：

问题：main.rs 缺少以下模块的声明和集成：
1. energy - 能耗透视引擎
2. fingerprint - 进程指纹识别  
3. display - 显示控制
4. error - 统一错误类型
5. constants - 集中常量管理

要求：
1. 在 main.rs 顶部添加 mod 声明：
   mod energy;
   mod fingerprint;
   mod display;
   mod error;
   mod constants;

2. 在 main.rs 中集成这些模块：
   - 使用 error::SmartTuneError 作为错误类型
   - 使用 constants 中的常量（如 thermal::THROTTLE_LIGHT）
   - 在主循环中调用 energy 模块记录能耗数据
   - 使用 fingerprint 识别应用类型
   - 使用 display 控制刷新率

3. 修改 main 函数签名，返回 Result<(), SmartTuneError>

4. 替换所有 expect() 调用为 ? 操作符

项目路径：/app/data/storage/projects/1785249992652501794-1864
文件：src/rust/src/main.rs"""

    out, _ = run(f'curl -s -X POST http://localhost:8086/api/v1/agent/run -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d \'{{"task":"{task.replace(chr(10)," ").replace('"',chr(92)+chr(34))}","provider_id":"opencode-go","model":"mimo-v2.5","agent_mode":"plan"}}\'')
    print(out[:500])
    
    client.close()

if __name__ == "__main__":
    main()
