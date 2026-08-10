#!/usr/bin/env python3
"""修复重复调用并改进"""
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
    
    # 2. 修复重复调用
    print("\n=== 2. 修复重复调用 ===")
    # 删除重复的 energy_profiler.record_sample
    new_content = current_content.replace(
        """                // Record energy data
                energy_profiler.record_sample(&feature);


                // Detect application type
                let app_type = fingerprint_detector.detect(&feature);
                eprintln!("[AndroSt] App type: {:?}", app_type);

                // Record energy data
                energy_profiler.record_sample(&feature);""",
        """                // Record energy data
                energy_profiler.record_sample(&feature);

                // Detect application type
                let app_type = fingerprint_detector.detect(&feature);
                eprintln!("[AndroSt] App type: {:?}", app_type);"""
    )
    
    # 3. 添加 EEI 奖励权重
    print("\n=== 3. 添加 EEI 奖励权重 ===")
    new_content = new_content.replace(
        "let reward = (jitter_score * 0.5 + temp_penalty * 0.3 + battery_factor * 0.2)\n                        * thermal_factor * doze_factor * 100.0;",
        """// Calculate EEI (Energy Efficiency Index)
                    let eei = energy_profiler.calculate_eei(&feature);
                    
                    let reward = (jitter_score * 0.5 + temp_penalty * 0.3 + battery_factor * 0.1 + eei * 0.1)
                        * thermal_factor * doze_factor * 100.0;"""
    )
    
    # 4. 添加显示控制调用
    print("\n=== 4. 添加显示控制调用 ===")
    new_content = new_content.replace(
        "let arm = model.decide();",
        """// Apply display control based on scenario
                    let target_refresh_rate = display_controller.optimize_refresh_rate(
                        current_scenario,
                        &feature,
                        thermal_state
                    );
                    
                    // Get arm (policy level) decision
                    let arm = model.decide();"""
    )
    
    # 5. 写入新文件
    print("\n=== 5. 写入新文件 ===")
    sftp = client.open_sftp()
    with sftp.open(f"{project_path}/src/rust/src/main.rs", "w") as f:
        f.write(new_content)
    sftp.close()
    print("文件已更新")
    
    # 6. 验证
    print("\n=== 6. 验证 ===")
    out, _ = run(f"grep -n 'energy_profiler\\|fingerprint_detector\\|display_controller' {project_path}/src/rust/src/main.rs")
    print(f"模块调用:\n{out}")
    
    out, _ = run(f"wc -l {project_path}/src/rust/src/main.rs")
    print(f"总行数: {out.strip()}")
    
    client.close()

if __name__ == "__main__":
    main()
