#!/system/bin/sh
# {{MODULE_NAME}} - Magisk Module Installer
# Author: {{AUTHOR}}
# Description: {{DESCRIPTION}}
# 兼容: Magisk, KernelSU, APatch

SKIPUNZIP=1

# Print module info
ui_print "============================================"
ui_print "  {{MODULE_NAME}} v{{VERSION}}"
ui_print "  {{DESCRIPTION}}"
ui_print "============================================"

# Extract module files
ui_print "- Extracting module files"
unzip -o "$ZIPFILE" -d "$MODPATH" >&2

# Set permissions
ui_print "- Setting permissions"
set_perm_recursive $MODPATH 0 0 0755 0644
set_perm $MODPATH/system/bin/{{MODULE_ID}} 0 0 0755

# Root manager detection (兼容 Magisk, KernelSU, APatch)
# 根据官方文档:
# - Magisk: 使用 MAGISK_VER_CODE 变量
# - KernelSU: 使用 KSU 变量（始终为 true）
# - APatch: 使用 APATCH 变量（始终为 true）
if [ -n "$KSU" ]; then
    ui_print "- KernelSU detected (version: $KSU_KERNEL_VER_CODE)"
    ROOT_MANAGER="kernelsu"
elif [ -n "$APATCH" ]; then
    ui_print "- APatch detected (version: $APATCH_VER_CODE)"
    ROOT_MANAGER="apatch"
else
    ui_print "- Magisk detected (version: $MAGISK_VER_CODE)"
    ROOT_MANAGER="magisk"
fi

# SELinux policy injection - 兼容所有root管理器
# 根据官方文档:
# - Magisk: 使用 magiskpolicy 命令（位于 /data/adb/magisk/magiskpolicy）
# - KernelSU: 使用 sepolicy.rule 文件（已在模块根目录，会自动加载）
# - APatch: 直接使用 magiskpolicy（兼容Magisk）
ui_print "- Configuring SELinux policy..."

if command -v magiskpolicy >/dev/null 2>&1; then
    ui_print "  Using magiskpolicy for SELinux rules"
    # magiskpolicy 会在 Magisk 和 APatch 中可用
    # KernelSU 也会提供 magiskpolicy 兼容命令
elif [ -f "$MODPATH/sepolicy.rule" ]; then
    ui_print "  Using sepolicy.rule file (KernelSU style)"
    # sepolicy.rule 文件会被 KernelSU 自动加载
else
    ui_print "  Warning: No SELinux policy method available"
    ui_print "  SELinux may restrict some operations"
fi

# Post-install
ui_print "- Installation complete!"
ui_print ""
ui_print "Note: Reboot to activate the module."
