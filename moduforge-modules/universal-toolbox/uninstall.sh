#!/system/bin/sh

# ============================================
# 系统优化工具箱 - 卸载脚本
# 清理所有优化设置
# ============================================

# 删除符号链接和文件
rm -rf /data/local/tmp/system_toolbox 2>/dev/null

# 恢复默认内核参数（仅影响运行时，重启后失效）
echo "cubic" > /proc/sys/net/ipv4/tcp_congestion_control 2>/dev/null
echo "0" > /proc/sys/net/ipv4/tcp_fastopen 2>/dev/null
echo "1" > /proc/sys/net/ipv4/tcp_slow_start_after_idle 2>/dev/null
echo "60" > /proc/sys/net/ipv4/tcp_fin_timeout 2>/dev/null
echo "7200" > /proc/sys/net/ipv4/tcp_keepalive_time 2>/dev/null
echo "10" > /proc/sys/vm/dirty_ratio 2>/dev/null
echo "3" > /proc/sys/vm/dirty_background_ratio 2>/dev/null
echo "100" > /proc/sys/vm/swappiness 2>/dev/null

# 删除模块目录
rm -rf /data/adb/modules/universal_system_toolbox 2>/dev/null
rm -rf /data/adb/ksu/modules/universal_system_toolbox 2>/dev/null
rm -rf /data/adb/ap/modules/universal_system_toolbox 2>/dev/null

log -t "SystemToolbox" "卸载完成"
