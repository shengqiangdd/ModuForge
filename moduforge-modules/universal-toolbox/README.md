# 系统优化工具箱

Universal System Optimization Toolbox — 兼容 Magisk / KernelSU / APatch

## 功能

- **网络优化**: TCP BBR 拥塞控制, 缓冲区调优, Fast Open
- **内存管理**: VM 参数优化, ZRAM 配置, LMK 调整
- **电池省电**: 电源管理优化, wakelock 控制
- **I/O 性能**: BFQ 调度器, 读取前移优化
- **GPU 渲染**: 硬件加速, 渲染管线优化

## 兼容性

| 管理器 | 支持 | 安装方式 |
|--------|------|----------|
| Magisk | ✅ | module.prop + customize.sh |
| KernelSU | ✅ | customize.sh (自动检测) |
| APatch | ✅ | customize.sh (自动检测) |

## 文件结构

```
universal-toolbox/
├── META-INF/com/google/android/  # Magisk 兼容
│   ├── update-binary
│   └── updater-script
├── module.prop                    # 模块信息 (Magisk)
├── apatch_module.prop             # 模块信息 (APatch)
├── system.prop                    # 系统属性
├── customize.sh                   # 智能安装脚本
├── post-fs-data.sh                # 开机脚本
├── service.sh                     # 服务脚本
├── uninstall.sh                   # 卸载脚本
├── system/etc/
│   └── optimize.conf              # 配置文件
└── README.md
```

## 安装

1. 在 Magisk / KernelSU / APatch 中刷入
2. 重启设备
3. 所有优化自动生效

## 自定义

编辑 `/data/adb/modules/universal_system_toolbox/system/etc/optimize.conf` 可调整优化行为。
