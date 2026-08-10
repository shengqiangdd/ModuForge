#!/usr/bin/env python3
"""添加 optimize_refresh_rate 方法到 display.rs"""
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
    
    # 1. 读取当前 display.rs
    print("=== 1. 读取当前 display.rs ===")
    out, _ = run(f"cat {project_path}/src/rust/src/display.rs")
    current_content = out
    
    # 2. 检查是否已有 optimize_refresh_rate 方法
    if "optimize_refresh_rate" in current_content:
        print("已有 optimize_refresh_rate 方法，跳过")
        return
    
    # 3. 在 DisplayController impl 块中添加方法
    print("\n=== 2. 添加 optimize_refresh_rate 方法 ===")
    
    # 找到 impl DisplayController 块的结束位置
    # 在最后一个方法后添加新方法
    new_method = """
    /// 根据场景、特征和热状态优化刷新率
    pub fn optimize_refresh_rate(
        &mut self,
        scenario: Option<&str>,
        feature: &dyn std::fmt::Debug,
        thermal_state: crate::thermal_control::ThermalState,
    ) -> RefreshRate {
        // 根据热状态调整
        let target = match thermal_state {
            crate::thermal_control::ThermalState::Critical => RefreshRate::Hz30,
            crate::thermal_control::ThermalState::Hot => RefreshRate::Hz60,
            crate::thermal_control::ThermalState::Warm => RefreshRate::Hz90,
            crate::thermal_control::ThermalState::Cool => {
                // 根据场景调整
                match scenario {
                    Some("HeavyGame") => RefreshRate::Hz120,
                    Some("LightGame") => RefreshRate::Hz90,
                    Some("Video") => RefreshRate::Hz60,
                    _ => RefreshRate::Hz60,
                }
            }
        };
        
        self.set_target_refresh(target);
        target
    }
"""
    
    # 在最后一个方法后添加
    # 找到 "}  // impl DisplayController" 的位置
    if "}  // impl DisplayController" in current_content:
        new_content = current_content.replace(
            "}  // impl DisplayController",
            f"{new_method}}}  // impl DisplayController"
        )
    else:
        # 如果没有注释，找最后一个 } 并在前面添加
        # 简单方法：在文件末尾添加
        new_content = current_content + new_method + "\n"
    
    # 4. 写入新文件
    print("\n=== 3. 写入新文件 ===")
    sftp = client.open_sftp()
    with sftp.open(f"{project_path}/src/rust/src/display.rs", "w") as f:
        f.write(new_content)
    sftp.close()
    print("文件已更新")
    
    # 5. 验证
    print("\n=== 4. 验证 ===")
    out, _ = run(f"grep -n 'optimize_refresh_rate' {project_path}/src/rust/src/display.rs")
    print(f"方法检查:\n{out}")
    
    out, _ = run(f"wc -l {project_path}/src/rust/src/display.rs")
    print(f"总行数: {out.strip()}")
    
    client.close()

if __name__ == "__main__":
    main()
