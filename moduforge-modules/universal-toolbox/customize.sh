#!/system/bin/sh

# ============================================
# 系统优化工具箱 - 智能安装脚本
# 自动检测 Magisk / KernelSU / APatch
# ============================================

# 检测 root 管理器类型
detect_manager() {
  if [ -n "$KSU" ] && [ "$KSU" = "true" ]; then
    echo "KernelSU"
  elif [ -n "$APATCH" ] && [ "$APATCH" = "true" ]; then
    echo "APatch"
  elif [ -d "/data/adb/magisk" ] || [ -f "/sbin/magisk" ] || command -v magisk >/dev/null 2>&1; then
    echo "Magisk"
  elif [ -d "/data/adb/ksu" ] || [ -f "/sbin/ksu" ] || command -v ksu >/dev/null 2>&1; then
    echo "KernelSU"
  elif [ -d "/data/adb/ap" ] || command -v apd >/dev/null 2>&1; then
    echo "APatch"
  else
    echo "Magisk"
  fi
}

MANAGER=$(detect_manager)
MODDIR=${0%/*}
MODPATH=$MODDIR

ui_print "- 系统优化工具箱 v1.0"
ui_print "- 检测到 root 管理器: $MANAGER"
ui_print "- 正在安装..."

# 设置权限
set_permissions() {
  set_perm_recursive "$MODPATH" 0 0 0755 0644
  set_perm_recursive "$MODPATH/system/etc" 0 0 0755 0644
  set_perm "$MODPATH/post-fs-data.sh" 0 0 0755
  set_perm "$MODPATH/service.sh" 0 0 0755
  set_perm "$MODPATH/uninstall.sh" 0 0 0755
  set_perm "$MODPATH/customize.sh" 0 0 0755
}

# 根据管理器执行安装逻辑
case "$MANAGER" in
  "Magisk")
    ui_print "- [Magisk] 标准模块安装"
    ui_print "- 加载 system.prop 系统属性..."
    ui_print "- 注册 post-fs-data.sh 启动脚本"
    ui_print "- 注册 service.sh 服务脚本"
    ;;
  "KernelSU")
    ui_print "- [KernelSU] 使用 WebUI 兼容模式"
    if [ ! -d "$MODPATH/ksud" ]; then
      ui_print "- KernelSU 模块无需 META-INF"
    fi
    ;;
  "APatch")
    ui_print "- [APatch] 使用 action.sh 模式"
    if [ -f "$MODPATH/apatch_module.prop" ]; then
      ui_print "- 加载 APatch 模块配置"
    fi
    ;;
esac

# KernelSU 和 APatch 需要确保 customize.sh 正确执行
if [ "$MANAGER" = "KernelSU" ] || [ "$MANAGER" = "APatch" ]; then
  # 确保 post-fs-data.sh 和 service.sh 有执行权限
  chmod 0755 "$MODPATH/post-fs-data.sh" 2>/dev/null
  chmod 0755 "$MODPATH/service.sh" 2>/dev/null
  chmod 0755 "$MODPATH/uninstall.sh" 2>/dev/null
fi

# 应用 system.prop（所有管理器通用）
if [ -f "$MODPATH/system.prop" ]; then
  ui_print "- 系统属性文件就绪"
fi

# 创建必要的目录
mkdir -p "$MODPATH/system/etc" 2>/dev/null

ui_print ""
ui_print "- 安装完成！"
ui_print "- 请重启设备以应用优化"
ui_print ""

# 设置权限
set_permissions 2>/dev/null

# KernelSU 和 APatch 需要返回值 0
return 0 2>/dev/null
exit 0
