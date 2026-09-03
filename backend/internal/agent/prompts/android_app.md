# Android Companion APP Development Guide

本指南定义何时生成 APP、如何设计 APP 架构、模块与 APP 之间的通信协议，以及各场景的完整代码模板。

---

## 1. 何时生成 APP（自然语言触发规则）

当用户请求包含以下任一特征时，**必须**生成配套 Android APP：

### 1.1 UI 关键词
- 界面、UI、APP、应用、控制面板、仪表盘、设置页面、配置页面

### 1.2 交互关键词
- 配置、调整、开关、滑块、选择、输入、参数设置

### 1.3 监控关键词
- 监控、查看、显示、实时、刷新、状态、日志

### 1.4 管理关键词
- 管理、控制、操作、启用、禁用

### 1.5 明确需求
- "我想在手机上管理"
- "需要一个 APP 来控制"
- "需要图形化界面"
- "需要可视化展示"

### 1.6 不需要 APP 的场景
- 纯后台服务（无用户交互需求）
- 纯脚本执行（命令行工具）
- 用户明确表示不需要 UI

---

## 2. APP 架构设计指南

根据模块功能自动选择 APP 架构：

### 2.1 简单设置型
**触发条件**：只有开关、输入框、滑块等基本控件
**架构**：单 Activity + SharedPreferences

```
app/
├── src/main/java/com/moduforge/<module_id>/
│   ├── MainActivity.kt
│   └── SettingsFragment.kt
├── src/main/res/layout/
│   ├── activity_main.xml
│   └── fragment_settings.xml
└── build.gradle.kts
```

### 2.2 监控仪表盘型
**触发条件**：需要实时显示 CPU/内存/电池等数据
**架构**：单 Activity + Handler 定时刷新 + TextView

```
app/
├── src/main/java/com/moduforge/<module_id>/
│   ├── MainActivity.kt          # 主界面 + 定时刷新
│   └── StatusMonitor.kt         # 状态读取逻辑
├── src/main/res/layout/
│   └── activity_main.xml
└── build.gradle.kts
```

### 2.3 多功能管理型
**触发条件**：多个功能模块，需要切换页面
**架构**：Activity + Fragment + ViewPager2 + BottomNavigation

```
app/
├── src/main/java/com/moduforge/<module_id>/
│   ├── MainActivity.kt
│   ├── DashboardFragment.kt     # 首页仪表盘
│   ├── SettingsFragment.kt      # 设置页面
│   ├── LogsFragment.kt          # 日志查看
│   └── adapter/
│       └── ViewPagerAdapter.kt
├── src/main/res/layout/
│   ├── activity_main.xml
│   ├── fragment_dashboard.xml
│   ├── fragment_settings.xml
│   └── fragment_logs.xml
├── src/main/res/menu/
│   └── bottom_nav_menu.xml
└── build.gradle.kts
```

### 2.4 后台服务型
**触发条件**：需要常驻后台、发送通知
**架构**：Activity + Foreground Service + Notification

```
app/
├── src/main/java/com/moduforge/<module_id>/
│   ├── MainActivity.kt
│   ├── ModuleService.kt         # 前台服务
│   └── NotificationHelper.kt    # 通知管理
├── src/main/res/layout/
│   └── activity_main.xml
└── build.gradle.kts
```

---

## 3. 模块 ↔ APP 通信协议

### 3.1 数据存储位置

```
/data/adb/modules/<module_id>/shared_prefs/
├── module_config.xml    # APP → 模块（配置数据）
└── module_status.xml    # 模块 → APP（状态数据）
```

### 3.2 APP 端读取模块配置

```kotlin
// 读取模块写入的状态文件
val statusFile = File("/data/adb/modules/<module_id>/shared_prefs/module_status.xml")
if (statusFile.exists()) {
    val statusPrefs = getSharedPreferences("module_status", MODE_WORLD_READABLE)
    val status = statusPrefs.getString("module_status", "unknown")
    val cpu = statusPrefs.getString("cpu_usage", "N/A")
    val mem = statusPrefs.getString("memory_usage", "N/A")
    
    // 更新 UI
    findViewById<TextView>(R.id.textStatus).text = "状态: $status"
    findViewById<TextView>(R.id.textCpu).text = "CPU: $cpu"
    findViewById<TextView>(R.id.textMemory).text = "内存: $mem"
}
```

### 3.3 APP 端写入配置

```kotlin
// 写入配置供模块读取
val prefs = getSharedPreferences("module_config", Context.MODE_WORLD_READABLE)
prefs.edit()
    .putBoolean("module_enabled", true)
    .putInt("threshold", 80)
    .putString("mode", "performance")
    .apply()
```

### 3.4 模块端读取 APP 配置 (service.sh)

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
```

### 3.5 模块端写入状态供 APP 读取

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

### 3.6 常用 SharedPreferences Key 定义

| Key | 类型 | 方向 | 说明 |
|-----|------|------|------|
| `module_enabled` | Boolean | APP→模块 | 模块启用状态 |
| `threshold` | Int | APP→模块 | 阈值参数 |
| `mode` | String | APP→模块 | 运行模式 |
| `module_status` | String | 模块→APP | 运行状态 |
| `cpu_usage` | String | 模块→APP | CPU 使用率 |
| `memory_usage` | String | 模块→APP | 内存使用量 |
| `last_update_time` | Long | 模块→APP | 最后更新时间戳 |

---

## 4. APP 组件模板库

### 场景 1：简单设置页面

```kotlin
package com.moduforge.<module_id>

import android.content.Context
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import com.google.android.material.switchmaterial.SwitchMaterial
import com.google.android.material.slider.Slider
import android.widget.TextView
import android.widget.EditText

class MainActivity : AppCompatActivity() {
    private lateinit var prefs: android.content.SharedPreferences

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        prefs = getSharedPreferences("module_config", Context.MODE_WORLD_READABLE)

        // 绑定启用开关
        findViewById<SwitchMaterial>(R.id.switchEnable).apply {
            isChecked = prefs.getBoolean("module_enabled", true)
            setOnCheckedChangeListener { _, isChecked ->
                prefs.edit().putBoolean("module_enabled", isChecked).apply()
            }
        }

        // 绑定阈值滑块
        val slider = findViewById<Slider>(R.id.sliderThreshold)
        val valueText = findViewById<TextView>(R.id.textThresholdValue)
        slider.value = prefs.getInt("threshold", 80).toFloat()
        valueText.text = "${slider.value.toInt()}%"
        slider.addOnChangeListener { _, value, fromUser ->
            if (fromUser) {
                valueText.text = "${value.toInt()}%"
                prefs.edit().putInt("threshold", value.toInt()).apply()
            }
        }

        // 绑定模式输入
        val modeInput = findViewById<EditText>(R.id.editMode)
        modeInput.setText(prefs.getString("mode", "balanced"))
        modeInput.setOnFocusChangeListener { _, hasFocus ->
            if (!hasFocus) {
                prefs.edit().putString("mode", modeInput.text.toString()).apply()
            }
        }
    }
}
```

### 场景 2：实时监控仪表盘

```kotlin
package com.moduforge.<module_id>

import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity
import java.io.File
import java.text.SimpleDateFormat
import java.util.*

class MainActivity : AppCompatActivity() {
    private val handler = Handler(Looper.getMainLooper())
    private val refreshInterval = 2000L // 2 秒刷新一次
    private val dateFormat = SimpleDateFormat("HH:mm:ss", Locale.getDefault())

    private val refreshRunnable = object : Runnable {
        override fun run() {
            refreshStatus()
            handler.postDelayed(this, refreshInterval)
        }
    }

    override fun onResume() {
        super.onResume()
        handler.post(refreshRunnable)
    }

    override fun onPause() {
        super.onPause()
        handler.removeCallbacks(refreshRunnable)
    }

    private fun refreshStatus() {
        val moduleId = packageName.substringAfterLast(".")
        val statusFile = File("/data/adb/modules/$moduleId/shared_prefs/module_status.xml")
        
        if (statusFile.exists()) {
            val statusPrefs = getSharedPreferences("module_status", MODE_WORLD_READABLE)
            val status = statusPrefs.getString("module_status", "unknown")
            val cpu = statusPrefs.getString("cpu_usage", "N/A")
            val mem = statusPrefs.getString("memory_usage", "N/A")
            val battery = statusPrefs.getString("battery_level", "N/A")
            val updateTime = statusPrefs.getLong("last_update_time", 0)

            findViewById<TextView>(R.id.textStatus).text = "状态: $status"
            findViewById<TextView>(R.id.textCpu).text = "CPU: $cpu"
            findViewById<TextView>(R.id.textMemory).text = "内存: $mem"
            findViewById<TextView>(R.id.textBattery).text = "电量: $battery"
            findViewById<TextView>(R.id.textLastUpdate).text = "更新: ${dateFormat.format(Date(updateTime))}"
        } else {
            findViewById<TextView>(R.id.textStatus).text = "状态: 等待模块启动..."
        }
    }
}
```

### 场景 3：前台服务 + 通知

```kotlin
package com.moduforge.<module_id>

import android.app.*
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat

class ModuleService : Service() {
    companion object {
        private const val CHANNEL_ID = "module_service_channel"
        private const val NOTIFICATION_ID = 1

        fun start(context: Context) {
            val intent = Intent(context, ModuleService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        fun stop(context: Context) {
            context.stopService(Intent(context, ModuleService::class.java))
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val notificationIntent = Intent(this, MainActivity::class.java)
        val pendingIntent = PendingIntent.getActivity(
            this, 0, notificationIntent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )

        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("模块运行中")
            .setContentText("点击查看详细状态")
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()

        startForeground(NOTIFICATION_ID, notification)
        return START_STICKY
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "模块服务",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "模块后台服务通知"
            }
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(channel)
        }
    }
}
```

### 场景 4：多 Fragment + 底部导航

```kotlin
// MainActivity.kt
package com.moduforge.<module_id>

import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.fragment.app.Fragment
import com.google.android.material.bottomnavigation.BottomNavigationView

class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        if (savedInstanceState == null) {
            loadFragment(DashboardFragment())
        }

        findViewById<BottomNavigationView>(R.id.bottomNav).setOnItemSelectedListener { item ->
            when (item.itemId) {
                R.id.nav_dashboard -> loadFragment(DashboardFragment())
                R.id.nav_settings -> loadFragment(SettingsFragment())
                R.id.nav_logs -> loadFragment(LogsFragment())
            }
            true
        }
    }

    private fun loadFragment(fragment: Fragment) {
        supportFragmentManager.beginTransaction()
            .replace(R.id.fragmentContainer, fragment)
            .commit()
    }
}
```

```kotlin
// DashboardFragment.kt
package com.moduforge.<module_id>

import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.TextView
import androidx.fragment.app.Fragment
import java.io.File

class DashboardFragment : Fragment() {
    private val handler = Handler(Looper.getMainLooper())

    override fun onCreateView(
        inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?
    ): View? = inflater.inflate(R.layout.fragment_dashboard, container, false)

    override fun onResume() {
        super.onResume()
        handler.post(refreshRunnable)
    }

    override fun onPause() {
        super.onPause()
        handler.removeCallbacks(refreshRunnable)
    }

    private val refreshRunnable = object : Runnable {
        override fun run() {
            refreshStatus()
            handler.postDelayed(this, 2000)
        }
    }

    private fun refreshStatus() {
        view ?: return
        val moduleId = requireContext().packageName.substringAfterLast(".")
        val statusFile = File("/data/adb/modules/$moduleId/shared_prefs/module_status.xml")
        if (statusFile.exists()) {
            val prefs = requireContext().getSharedPreferences("module_status", android.content.Context.MODE_WORLD_READABLE)
            view?.findViewById<TextView>(R.id.textStatus)?.text = "状态: ${prefs.getString("module_status", "unknown")}"
            view?.findViewById<TextView>(R.id.textCpu)?.text = "CPU: ${prefs.getString("cpu_usage", "N/A")}"
            view?.findViewById<TextView>(R.id.textMemory)?.text = "内存: ${prefs.getString("memory_usage", "N/A")}"
        }
    }
}
```

### 场景 5：带图表的数据可视化

```kotlin
// 使用简单的自定义 View 绘制柱状图
package com.moduforge.<module_id>

import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.util.AttributeSet
import android.view.View

class BarChartView @JvmOverloads constructor(
    context: Context, attrs: AttributeSet? = null
) : View(context, attrs) {

    private val barPaint = Paint().apply {
        color = Color.parseColor("#4CAF50")
        style = Paint.Style.FILL
    }
    private val textPaint = Paint().apply {
        color = Color.WHITE
        textSize = 36f
        textAlign = Paint.Align.CENTER
    }

    private var dataPoints = listOf<Float>()

    fun setData(data: List<Float>) {
        dataPoints = data
        invalidate()
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        if (dataPoints.isEmpty()) return

        val maxValue = dataPoints.maxOrNull() ?: 1f
        val barWidth = width.toFloat() / dataPoints.size
        val maxBarHeight = height * 0.8f

        dataPoints.forEachIndexed { index, value ->
            val barHeight = (value / maxValue) * maxBarHeight
            val left = index * barWidth + barWidth * 0.1f
            val right = (index + 1) * barWidth - barWidth * 0.1f
            val top = height - barHeight
            val bottom = height.toFloat()

            canvas.drawRect(left, top, right, bottom, barPaint)
            canvas.drawText("${value.toInt()}%", (left + right) / 2, top - 10, textPaint)
        }
    }
}
```

---

## 5. customize.sh 中安装 APK 的最佳实践

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

---

## 6. 常见问题排查

### APP 无法读取模块状态
- 检查 `shared_prefs` 目录权限是否为 `777`
- 检查文件路径是否正确：`/data/adb/modules/<module_id>/shared_prefs/`
- 检查 XML 文件格式是否正确

### APP 无法写入配置
- 检查 `getSharedPreferences` 是否使用 `MODE_WORLD_READABLE`
- 检查 `customize.sh` 是否设置了正确的目录权限

### 前台服务通知不显示
- 检查是否创建了 NotificationChannel（Android 8.0+）
- 检查 `startForeground()` 是否在 `onStartCommand()` 中调用
- 检查 `FOREGROUND_SERVICE` 权限是否声明

### APK 安装失败
- 检查签名是否一致（同一模块的 APK 应使用相同签名）
- 检查 Android 版本是否满足最低要求（API 26+）
- 检查存储空间是否充足
