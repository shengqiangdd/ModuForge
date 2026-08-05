#!/usr/bin/env python3
"""Run ModuForge Agent for Phase 3"""
import paramiko
import json
import time

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect('192.168.2.9', username='admin', password='csq0216', timeout=10)

def run(cmd):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=30)
    return stdout.read().decode('utf-8', errors='replace')

# Get auth token
login_resp = run("curl -s -X POST http://localhost:8086/api/v1/auth/login -H 'Content-Type: application/json' -d '{\"username\":\"csq\",\"password\":\"csq0216\"}'")
token = json.loads(login_resp).get('token', '')
print(f"Token: {token[:30]}...")

# Phase 3 Task: 能耗估算模型
task = """请为AndroBoost-SmartTune项目实现Phase 3的第一个核心功能：应用能耗估算模型。

## 需求背景
根据PRD文档Phase 3要求，需要实现：
1. 轮询/proc/uid_stat/或累加应用各线程CPU时间片×频率×能效系数，估算实时功率(mW)
2. 引入动态能效系数自校准：记录充电周期内总mAh消耗与CPU/GPU累计时间，线性回归拟合独有系数矩阵，使误差控制在±5%以内
3. 计算EEI（能效指数）= (平均FPS × 100) / (平均功率mW)

## 技术要求
- 在 src/rust/src/ 下创建 energy_monitor.rs
- 实现 EnergyMonitor 结构体
- 实现以下功能：
  1. read_cpu_times() - 读取/proc/[pid]/stat获取CPU时间片
  2. estimate_power() - 根据CPU时间片×频率×能效系数估算功率
  3. calibrate_coefficients() - 线性回归校准能效系数
  4. calculate_eei() - 计算能效指数
- 使用 serde 进行序列化
- 添加适当的错误处理和日志

## 参考
- 现有文件：src/rust/src/linucb.rs（LinUCB算法）
- 现有文件：src/rust/src/thermal.rs（温度监控）
- 共享内存结构：src/rust/src/ipc.rs

请直接创建文件并实现功能，不要询问确认。"""

print("\n=== Running Agent Task: Phase 3 Energy Monitor ===")
print(f"Task length: {len(task)} chars")

# Run Agent task
agent_resp = run(f"""curl -s -X POST http://localhost:8086/api/v1/agent/run \\
  -H 'Authorization: Bearer {token}' \\
  -H 'Content-Type: application/json' \\
  -d '{json.dumps({"task": task, "provider_id": "opencode-go", "model": "mimo-v2.5", "agent_mode": "plan"})}'""")

print("\n=== Agent Response ===")
clean_resp = agent_resp.encode('ascii', errors='replace').decode()
print(clean_resp[:2000])

ssh.close()
