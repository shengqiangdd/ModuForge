# Magisk / KernelSU / APatch 模块开发规范

本规范定义模块的文件结构、字段约束、权限标准和三平台兼容要求。生成代码时必须严格遵守。

---

## 1. 文件结构

```
module.prop                      # [必须] 模块元数据
customize.sh                     # [必须] 安装脚本
META-INF/com/google/android/
  update-binary                  # [必须] Magisk 安装器入口
  updater-script                 # [必须] 仅含 #MAGISK 一行
service.sh                       # [可选] 开机后常驻服务（Zygote 启动后执行）
uninstall.sh                     # [可选] 卸载清理
system.prop                      # [可选] 注入 system/build.prop 属性
common/                          # [可选] 共享文件
system/bin/                      # [可选] 放置 arm64/arm 通用二进制
system/lib64/                    # [可选] 64位 .so 库
system/lib/                      # [可选] 32位 .so 库
webroot/                         # [可选] WebUI 前端文件
```

**禁止出现的路径：**
- 根目录下的 `.git`、`node_modules`、`target/`、`build/`、`*.zip`
- `META-INF/` 下除 `com/google/android/` 以外的任何内容

---

## 2. module.prop 字段规范

| 字段 | 必须 | 格式 | 说明 |
|------|------|------|------|
| `id` | ✅ | `^[a-z][a-z0-9._-]{0,62}$` | 全小写，字母开头，仅含 `a-z 0-9 . _ -`，最长 63 字符 |
| `name` | ✅ | 任意 UTF-8 | 显示名称，可含空格和中文 |
| `version` | ✅ | 语义化版本 `MAJOR.MINOR.PATCH` | 如 `1.0.0`、`2.1.3`，禁止 `v` 前缀 |
| `versionCode` | ✅ | 正整数 | 每次发版递增，KSU 用此判断是否需要更新 |
| `author` | ✅ | 任意字符串 | 作者名 |
| `description` | ✅ | 任意字符串 | 模块功能描述 |
| `minMagisk` | 推荐 | 正整数 | 最低 Magisk 版本，如 `24000`（24.0） |
| `updateJson` | 可选 | URL | 在线更新地址 |

**`id` 字段反面案例（禁止）：**
- ❌ `MyModule` — 含大写
- ❌ `1module` — 数字开头
- ❌ `my module` — 含空格
- ❌ `my_module@v2` — 含非法字符 `@`

---

## 3. customize.sh 规范

### 3.1 必须包含的内容

```bash
#!/system/bin/sh
# 不要添加 # Magisk 或 # KernelSU 头部注释

# ---- 环境变量（由安装器自动设置，无需手动 export）----
# $TMPDIR   临时目录（安装完成后自动清理）
# $MODPATH  模块安装目标路径
# $ARCH     设备架构：arm64 / arm / x86 / x64
# $API      Android API 级别（如 33）
# $KSU      KernelSU 环境下为 "true"，否则未定义
# $APATCH   APatch 环境下为 "true"，否则未定义
# $MAGISK   Magisk 环境下为版本号字符串，否则未定义

# ---- 权限设置 ----
set_perm_recursive $MODPATH 0 0 0755 0644
set_perm $MODPATH/service.sh 0 0 0755
# 二进制文件必须设置可执行权限
# set_perm $MODPATH/system/bin/my_app 0 0 0755
```

### 3.2 不同文件类型的权限标准

| 文件类型 | 目录权限 | 文件权限 | 说明 |
|---------|---------|---------|------|
| 目录 | `0755` | — | 所有人可读， owner 可写 |
| 脚本（.sh） | — | `0755` | 必须可执行 |
| 二进制可执行文件 | — | `0755` | 必须可执行 |
| 配置文件（.prop/.json/.conf） | — | `0644` | 只读 |
| .so 动态库 | — | `0644` | 系统自动加载，无需可执行 |
| WebUI 文件（HTML/JS/CSS） | — | `0644` | 只读 |

**绝对禁止：**
- ❌ `chmod 777` 任何文件或目录
- ❌ `chmod 755` 配置文件（应为 0644）
- ❌ 遗漏 `set_perm` 导致二进制无法执行

### 3.3 三平台环境检测

```bash
print_device_info() {
    ui_print "- 设备架构: $ARCH"
    ui_print "- Android API: $API"
    if [ -n "$KSU" ]; then
        ui_print "- Root 管理器: KernelSU (v$KSU_KERNEL_VER_CODE)"
    elif [ -n "$APATCH" ]; then
        ui_print "- Root 管理器: APatch"
    else
        ui_print "- Root 管理器: Magisk (v$MAGISK_VER_CODE)"
    fi
}
```

### 3.4 customize.sh 禁止行为

- ❌ 使用 `su -c` — 安装脚本已在 root 环境运行
- ❌ 使用 `exit 0` 或 `exit 1` 终止安装（用 `abort "原因"` 代替）
- ❌ 在 `customize.sh` 中启动后台进程
- ❌ 修改 `/data/system/` 下的任何文件
- ❌ 硬编码设备特定路径（如 `/data/local/tmp/`）

---

## 4. META-INF 规范

### 4.1 updater-script

```
#MAGISK
```

仅此一行，无其他内容，无空行，无 BOM。

### 4.2 update-binary

标准模板（三平台兼容）：

```bash
#!/sbin/sh

#################
# Initialization
#################

umask 022

# echo before loading util_functions
ui_print() { echo "$1"; }

require_new_magisk() {
  ui_print "*******************************"
  ui_print " Please install Magisk v20.4+! "
  ui_print "*******************************"
  exit 1
}

#########################
# Load util_functions.sh
#########################

OUTFD=$2
ZIPFILE=$3

[ -f /data/adb/magisk/util_functions.sh ] || require_new_magisk
. /data/adb/magisk/util_functions.sh
[ $MAGISK_VER_CODE -lt 20400 ] && require_new_magisk

install_module
exit 0
```

**关键点：**
- `ui_print()` 必须在加载 `util_functions.sh` 之前定义（覆盖默认实现）
- 不要在 `update-binary` 中添加自定义逻辑，所有逻辑放 `customize.sh`

---

## 5. service.sh 规范

### 5.1 执行时机

- **Magisk**：Zygote 启动后（设备解锁后）执行，不是开机立即执行
- **KernelSU**：同 Magisk，Zygote 后执行
- **APatch**：同 Magisk

### 5.2 标准结构

```bash
#!/system/bin/sh
# service.sh - 开机后常驻服务

MODDIR=${0%/*}

# 等待系统启动完成
while [ "$(getprop sys.boot_completed)" != "1" ]; do
    sleep 3
done

# 启动后台服务
nohup $MODDIR/system/bin/my_app > /dev/null 2>&1 &
```

### 5.3 service.sh 禁止行为

- ❌ 使用 `while true; do ... done` 无退出条件的死循环（应用 `nohup` + 二进制自身管理重启）
- ❌ 在循环中调用 `sleep` 超过 60 秒（浪费资源）
- ❌ 直接修改系统文件（如 `/system/build.prop`）
- ❌ 使用 `am start` 启动 Activity（无 UI 交互场景禁止）

---

## 6. uninstall.sh 规范

```bash
#!/system/bin/sh
# uninstall.sh - 卸载清理

# 清理模块创建的目录
rm -rf /data/adb/my_module_data

# 清理日志
rm -rf /data/local/tmp/my_module*.log
```

**注意：** `$MODPATH` 在 uninstall.sh 中不可用，需要硬编码路径。

---

## 7. system.prop 规范

格式为标准 Android 属性文件，每行 `key=value`：

```
# 禁用系统更新检查
ro.system.update.enable=false
# 自定义属性（前缀 mymodule. 避免冲突）
mymodule.version=1.0
```

**规则：**
- 属性名必须有自定义前缀（如 `mymodule.`），避免与系统属性冲突
- 禁止修改 `ro.build.` 前缀的属性（会导致 SafetyNet 失败）
- 禁止修改 `persist.sys.` 前缀的属性（会影响系统稳定性）

---

## 8. 二进制文件放置规范

### 8.1 架构目录映射

| `$ARCH` 值 | 源码编译目标 | 放置路径 |
|------------|------------|---------|
| `arm64` | `aarch64-linux-android` | `system/bin/` 或 `system/lib64/` |
| `arm` | `armv7a-linux-androideabi` | `system/bin/` 或 `system/lib/` |
| `x86` | `i686-linux-android` | `system/bin/` 或 `system/lib/` |
| `x64` | `x86_64-linux-android` | `system/bin/` 或 `system/lib64/` |

### 8.2 Go 交叉编译

```bash
# arm64
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=$NDK/bin/aarch64-linux-android33-clang go build -o system/bin/my_app

# arm
CGO_ENABLED=1 GOOS=linux GOARCH=arm CC=$NDK/bin/armv7a-linux-androideabi33-clang go build -o system/bin/my_app
```

### 8.3 Rust 交叉编译

```bash
# 需要 rustup target add
rustup target add aarch64-linux-android armv7-linux-androideabi

# 编译
cargo build --release --target aarch64-linux-android
```

---

## 9. WebUI 规范

模块可提供 WebUI 管理界面（仅 KernelSU 支持）：

```
webroot/
  index.html        # 入口页面
  ...
```

- WebUI 通过 `file://` 协议加载，不能使用 `XMLHttpRequest`
- 使用 `fetch()` 调用本地 API 时，域名是 `http://localhost:端口号`
- CSS/JS 必须内联或使用相对路径，禁止 CDN 引用

---

## 10. 三平台差异对照

| 特性 | Magisk | KernelSU | APatch |
|------|--------|----------|--------|
| 模块安装路径 | `/data/adb/modules/` | `/data/adb/modules/` | `/data/adb/modules/` |
| 二进制路径 | `/data/adb/modules/<id>/system/bin/` | 同左 | 同左 |
| WebUI | ❌ 不支持 | ✅ `webroot/` | ✅ `webroot/` |
| service.sh | ✅ 支持 | ✅ 支持 | ✅ 支持 |
| `customize.sh` 环境变量 | `$MAGISK` 有值 | `$KSU` 有值 | `$APATCH` 有值 |
| SELinux 上下文 | 自动设置 | 可能需要手动 `chcon` | 可能需要手动 `chcon` |
| Zygisk | ✅ 支持 | ❌ 不适用 | ❌ 不适用 |
| 在线更新 | `updateJson` 字段 | `updateJson` 字段 | `updateJson` 字段 |

---

## 11. 代码质量检查清单

生成模块后，逐项验证：

- [ ] `module.prop` 的 `id` 字段符合正则 `^[a-z][a-z0-9._-]{0,62}$`
- [ ] `module.prop` 的 `version` 是语义化版本（无 `v` 前缀）
- [ ] `META-INF/com/google/android/updater-script` 仅含 `#MAGISK`
- [ ] `customize.sh` 包含 `set_perm_recursive $MODPATH 0 0 0755 0644`
- [ ] 所有 `.sh` 文件权限为 `0755`
- [ ] 所有二进制文件权限为 `0755`
- [ ] 所有配置文件权限为 `0644`
- [ ] 无 `chmod 777` 调用
- [ ] Shell 脚本开头有 `#!/system/bin/sh`
- [ ] Shell 脚本变量用双引号包裹 `"$VAR"`
- [ ] Go 文件每个都有 `package` 声明且 import 的包被使用
- [ ] Rust 文件 `Cargo.toml` 的 `[package]` name 与模块 id 一致
- [ ] 二进制交叉编译目标架构与 `$ARCH` 匹配

---

## 12. Android APP 伴侣

模块可携带一个 Android APP（APK），为用户提供图形化管理和监控界面。

### 12.1 何时需要 APP

以下场景建议为模块配套 APP：
- 需要开关控制（启用/禁用模块功能）
- 需要参数配置（设置阈值、频率、路径等）
- 需要状态监控（运行状态、日志查看、性能指标）
- 需要数据可视化（Dashboard、图表、统计信息）
- 用户明确提到"界面"、"UI"、"APP"、"设置页面"、"控制面板"

使用 `android_app` skill 生成完整项目，使用 `build_android_app` 编译 APK。

### 12.2 APP 与模块的通信方式

APP 通过 `SharedPreferences (MODE_WORLD_READABLE)` 与模块共享数据：

- APP 写入配置：`getSharedPreferences("module_config", Context.MODE_WORLD_READABLE)`
- 模块读取配置：`/data/adb/modules/<module_id>/shared_prefs/module_config.xml`
- 共享的 key 示例：`module_enabled`（布尔）、`module_status`（字符串）、`last_update_time`（长整数）

#### 数据流向

```
APP ──写入──> /data/adb/modules/<id>/shared_prefs/module_config.xml ──读取──> 模块脚本
APP <──读取── /data/adb/modules/<id>/shared_prefs/module_status.xml <──写入── 模块脚本
```

#### 常用 SharedPreferences Key

| Key | 类型 | 方向 | 说明 |
|-----|------|------|------|
| `module_enabled` | Boolean | APP→模块 | 模块启用状态 |
| `threshold` | Int | APP→模块 | 阈值参数 |
| `mode` | String | APP→模块 | 运行模式 |
| `module_status` | String | 模块→APP | 运行状态 |
| `cpu_usage` | String | 模块→APP | CPU 使用率 |
| `memory_usage` | String | 模块→APP | 内存使用量 |
| `last_update_time` | Long | 模块→APP | 最后更新时间戳 |

### 12.3 APK 在模块中的放置位置

```
module_root/
  app/
    app.apk          # 编译后的 APK，由 build_android_app 自动生成
  customize.sh       # 安装时自动安装 APK
```

### 12.4 customize.sh 中安装 APK 的最佳实践

```bash
# ---- 安装伴侣 APK（非致命） ----
if [ -f "$MODPATH/app/app.apk" ]; then
    ui_print "- 安装伴侣应用..."
    # 获取包名（从 APK 中提取）
    PACKAGE_NAME=$(pm list packages -f "$MODPATH/app/app.apk" 2>/dev/null | head -1 | cut -d= -f1 | cut -d: -f2)
    
    if [ -n "$PACKAGE_NAME" ]; then
        # 先卸载旧版本（避免签名冲突）
        pm uninstall "$PACKAGE_NAME" 2>/dev/null
        # 安装新版本
        if pm install -r "$MODPATH/app/app.apk" 2>/dev/null; then
            ui_print "  ✅ 伴侣应用安装成功"
            # 设置 APP 数据目录权限（支持 MODE_WORLD_READABLE）
            if [ -d "/data/data/$PACKAGE_NAME/shared_prefs" ]; then
                chmod 777 "/data/data/$PACKAGE_NAME/shared_prefs"
                ui_print "  ✅ 共享目录权限已设置"
            fi
        else
            ui_print "  ⚠️ 伴侣应用安装失败（非致命）"
        fi
    else
        ui_print "  ⚠️ 无法获取包名，跳过安装"
    fi
fi
```

**注意：**
- APK 安装失败不应中断模块安装（用 `if` 判断容错）
- APK 放在 `app/` 目录下，打包时自动包含在 ZIP 中
- APP 需要 Android 8.0+ (API 26+)
- 安装后需设置 `shared_prefs` 目录权限为 777，否则 APP 无法读写

### 12.5 模块端读取 APP 配置 (service.sh)

```bash
#!/system/bin/sh
# service.sh - 读取 APP 配置

MODDIR=${0%/*}
CONFIG_FILE="/data/adb/modules/<module_id>/shared_prefs/module_config.xml"

# 读取配置
if [ -f "$CONFIG_FILE" ]; then
    ENABLED=$(grep -o 'boolean name="module_enabled"[^/]*' "$CONFIG_FILE" | grep -o 'value="[^"]*"' | cut -d'"' -f2)
    THRESHOLD=$(grep -o 'int name="threshold"[^/]*' "$CONFIG_FILE" | grep -o 'value="[^"]*"' | cut -d'"' -f2)
    MODE=$(grep -o 'string name="mode"[^<]*' "$CONFIG_FILE" | sed 's/.*>\([^<]*\)<.*/\1/')
fi

# 默认值
ENABLED=${ENABLED:-"true"}
THRESHOLD=${THRESHOLD:-80}
MODE=${MODE:-"balanced"}

echo "Module enabled: $ENABLED, Threshold: $THRESHOLD, Mode: $MODE"
```

### 12.6 模块端写入状态供 APP 读取

```bash
#!/system/bin/sh
# 写入状态供 APP 读取

MODDIR=${0%/*}
STATUS_FILE="/data/adb/modules/<module_id>/shared_prefs/module_status.xml"

# 收集状态信息
CPU_USAGE=$(cat /proc/stat | head -1 | awk '{print $2+$4, $2+$4+$5}' | awk '{printf "%.1f%%", $1/$2*100}')
MEM_TOTAL=$(free -m | awk '/Mem:/ {print $2}')
MEM_USED=$(free -m | awk '/Mem:/ {print $3}')
MEMORY_USAGE="${MEM_USED}MB/${MEM_TOTAL}MB"
BATTERY=$(cat /sys/class/power_supply/battery/capacity 2>/dev/null || echo "N/A")
UPTIME=$(cat /proc/uptime | awk '{print $1}')

# 写入状态文件
cat > "$STATUS_FILE" << EOF
<?xml version='1.0' encoding='utf-8' standalone='yes' ?>
<map>
    <string name="module_status">running</string>
    <long name="last_update_time" value="$(date +%s)000" />
    <string name="cpu_usage">${CPU_USAGE}</string>
    <string name="memory_usage">${MEMORY_USAGE}</string>
    <string name="battery_level">${BATTERY}%</string>
    <string name="uptime">${UPTIME}s</string>
</map>
EOF
```

### 12.7 APP 架构选择

| 场景 | 架构 | 说明 |
|------|------|------|
| 简单设置 | 单 Activity + SharedPreferences | 只有开关、输入框 |
| 监控仪表盘 | Activity + Handler 定时刷新 | 实时显示 CPU/内存等 |
| 多功能管理 | Activity + Fragment + ViewPager2 | 多个页面切换 |
| 后台服务 | Activity + Foreground Service | 需要常驻后台+通知 |

### 12.8 常见问题排查

| 问题 | 可能原因 | 解决方案 |
|------|---------|---------|
| APP 无法读取模块状态 | shared_prefs 目录权限不对 | customize.sh 中设置 chmod 777 |
| APP 无法写入配置 | 未使用 MODE_WORLD_READABLE | 检查 getSharedPreferences 参数 |
| 前台服务通知不显示 | 未创建 NotificationChannel | Android 8.0+ 需要创建 Channel |
| APK 安装失败 | 签名冲突或版本不兼容 | 先卸载旧版本，检查 minSdkVersion |
