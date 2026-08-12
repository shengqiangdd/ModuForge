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
