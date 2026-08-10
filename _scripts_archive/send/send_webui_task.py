#!/usr/bin/env python3
"""发送 WebUI 改进任务给 ModuForge Agent"""
import sys, io, json, time
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
import paramiko

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

    # 1. 登录获取 token
    print("=== 1. 登录 ===")
    out, _ = run('curl -s -X POST http://localhost:8086/api/v1/auth/login -H "Content-Type: application/json" -d \'{"username":"admin","password":"admin123"}\'')
    data = json.loads(out)
    token = data.get("token", "")
    print(f"Token: {token[:50]}...")

    # 2. 发送任务：调参实验室 UI
    print("\n=== 2. 发送任务：调参实验室 UI ===")
    task = """请在 AndroBoost-SmartTune 的 WebUI 中添加「调参实验室」面板。

项目路径：/app/data/storage/projects/1785249992652501794-1864
文件：src/go/web/index.html

具体要求：
1. 在 HTML 的策略面板和日志面板之间，添加一个「调参实验室」面板
2. 面板标题：「🔬 调参实验室」
3. 包含以下控件（使用已有的 CSS class：tune-card, tune-row, tune-label, tune-slider, tune-value, tune-btn）：
   - Alpha 滑块（范围 0.01-2.0，步长 0.01，默认 0.25）
   - 探索率滑块（范围 0.01-1.0，步长 0.01，默认 0.1）
   - 臂数选择（下拉框：3/5/7/10，默认 5）
   - 维度选择（下拉框：5/10/15/20，默认 10）
   - 「应用参数」按钮
4. 点击「应用参数」按钮时：
   - 用 fetch POST /api/tune 发送 JSON：{"alpha": 值, "exploration_rate": 值, "arms": 值, "dim": 值}
   - 成功后显示「参数已更新」提示
5. 页面加载时用 fetch GET /api/tune 读取当前参数并填充滑块值
6. 在调参实验室下方添加「调参历史」区域，用 fetch GET /api/tune/history 读取并显示

注意：
- 只修改 index.html 文件，不要创建新文件
- 使用已有的 CSS class，不要新增样式
- 先 read_file 读取当前 index.html，然后用 write_file 写入完整新版本"""

    task_escaped = task.replace('"', '\\"').replace('\n', '\\n')
    out, _ = run(f'curl -s -X POST http://localhost:8086/api/v1/agent/run -H "Authorization: Bearer {token}" -H "Content-Type: application/json" -d "{{\\"task\\":\\"{task_escaped}\\",\\"provider_id\\":\\"opencode-go\\",\\"model\\":\\"mimo-v2.5\\",\\"agent_mode\\":\\"plan\\"}}"')
    
    # 解析 SSE 流
    print("\n=== Agent 响应 ===")
    for line in out.split('\n'):
        if line.startswith('data: '):
            try:
                data = json.loads(line[6:])
                if 'content' in data:
                    print(f"[{data.get('step', '?')}] {data['content'][:200]}")
                if 'error' in data:
                    print(f"[ERROR] {data['error']}")
            except:
                print(line[:200])

    # 3. 等待并检查结果
    print("\n=== 3. 等待 15 秒后检查文件 ===")
    time.sleep(15)
    
    proj = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"
    
    # 检查文件是否更新
    out, _ = run(f"ls -la {proj}/src/go/web/index.html")
    print(f"文件时间: {out.strip()}")
    
    # 检查是否有调参实验室相关内容
    out, _ = run(f"grep -c 'tune-card\\|tune-slider\\|调参实验室\\|api/tune' {proj}/src/go/web/index.html")
    print(f"调参相关代码行数: {out.strip()}")
    
    # 检查是否有 fetch API 调用
    out, _ = run(f"grep -c 'fetch(' {proj}/src/go/web/index.html")
    print(f"fetch 调用数: {out.strip()}")
    
    client.close()

if __name__ == "__main__":
    main()
