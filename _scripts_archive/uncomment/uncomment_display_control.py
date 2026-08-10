#!/usr/bin/env python3
"""取消注释 display_control 调用"""
import sys
import io
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
    
    project_path = "/vol1/docker/volumes/moduforge_moduforge_data/_data/projects/1785249992652501794-1864"
    
    # 1. 读取当前 main.rs
    print("=== 1. 读取当前 main.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/main.rs")
    current_content = out
    
    # 2. 取消注释 display_control 调用
    print("\n=== 2. 取消注释 display_control 调用 ===")
    new_content = current_content.replace(
        "// Apply display control based on scenario\n                    // TODO: Add optimize_refresh_rate method to DisplayController\n                    // let target_refresh_rate = display_controller.optimize_refresh_rate(\n                    //     current_scenario,\n                    //     &feature,\n                    //     thermal_state\n                    // );\n                    \n                    // Get arm (policy level) decision",
        "// Apply display control based on scenario\n                    let target_refresh_rate = display_controller.optimize_refresh_rate(\n                        current_scenario,\n                        &feature,\n                        thermal_state\n                    );\n                    \n                    // Get arm (policy level) decision"
    )
    
    # 3. 写入新文件
    print("\n=== 3. 写入新文件 ===")
    sftp = client.open_sftp()
    with sftp.open(f"{project_path}/src/rust/src/main.rs", "w") as f:
        f.write(new_content)
    sftp.close()
    print("文件已更新")
    
    # 4. 验证
    print("\n=== 4. 验证 ===")
    out, _ = run(f"grep -n 'optimize_refresh_rate' {project_path}/src/rust/src/main.rs")
    print(f"方法调用:\n{out}")
    
    out, _ = run(f"wc -l {project_path}/src/rust/src/main.rs")
    print(f"总行数: {out.strip()}")
    
    client.close()

if __name__ == "__main__":
    main()
