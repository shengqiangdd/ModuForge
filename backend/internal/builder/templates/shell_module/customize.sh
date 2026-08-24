#!/system/bin/sh
# {{MODULE_NAME}} - Magisk Module Installer
# Author: {{AUTHOR}}
# Description: {{DESCRIPTION}}

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

# Platform detection
if [ -n "$KSU" ]; then
    ui_print "- KernelSU detected (version: $KSU_KERNEL_VER_CODE)"
elif [ -n "$APATCH" ]; then
    ui_print "- APatch detected"
else
    ui_print "- Magisk detected (version: $MAGISK_VER_CODE)"
fi

# Post-install
ui_print "- Installation complete!"
ui_print ""
ui_print "Note: Reboot to activate the module."
