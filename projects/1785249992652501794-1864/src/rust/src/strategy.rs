/// strategy.rs — 多算法智能动态选择引擎
///
/// 核心设计：
/// 1. 每个子问题提供 3+ 种算法（功耗估算、温控决策、DVFS调节）
/// 2. Meta-learner 跟踪每种算法近期的预测误差
/// 3. 每次决策时动态选择误差最低的算法
/// 4. 自动检测设备特性（CPU频率表、电池容量、温区数量）

use std::collections::HashMap;

// ============================================================
//  Part 1: 功耗估算算法族
// ============================================================

/// 功耗估算输入上下文（每采样周期更新一次）
#[derive(Clone, Debug, Default)]
pub struct PowerContext {
    pub cpu_jiffies_delta: u64,       // CPU jiffies 差值
    pub cpu_freq_khz: Vec<u64>,       // 各核心当前频率 (kHz)
    pub cpu_freq_max_khz: Vec<u64>,   // 各核心最大频率 (kHz)
    pub rx_bytes: u64,                // 网络接收字节
    pub tx_bytes: u64,                // 网络发送字节
    pub net_type: NetType,            // WiFi / LTE / 5G
    pub gpu_freq_khz: u64,            // GPU 频率 (kHz)
    pub gpu_freq_max_khz: u64,        // GPU 最大频率 (kHz)
    pub screen_brightness: u32,       // 屏幕亮度 0-255
    pub screen_max_brightness: u32,   // 屏幕最大亮度
    pub battery_voltage_mv: f64,      // 电池电压 (mV)
    pub battery_current_ma: f64,       // 电池电流 (mA, 放电为正)
    pub elapsed_secs: f64,            // 采样间隔 (秒)
    pub thermal_temp: f64,            // 当前温度 °C
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Default)]
pub enum NetType {
    #[default]
    WiFi,
    LTE,
    NR5G,
    Ethernet,
    Unknown,
}

/// 功耗估算结果
#[derive(Clone, Debug, Default)]
pub struct PowerEstimate {
    pub total_mw: f64,          // 总功耗 (mW)
    pub cpu_mw: f64,            // CPU 功耗
    pub gpu_mw: f64,            // GPU 功耗
    pub net_mw: f64,            // 网络功耗
    pub screen_mw: f64,         // 屏幕功耗
    pub base_mw: f64,           // 基础功耗
    pub confidence: f64,        // 置信度 0.0~1.0
    pub algorithm: &'static str, // 使用的算法名
}

// ---- 算法 1: Coulomb 计数法（直接读电池 V*I）----
fn estimate_coulomb(ctx: &PowerContext) -> PowerEstimate {
    if ctx.battery_current_ma > 0.0 && ctx.battery_voltage_mv > 0.0 {
        let power = ctx.battery_voltage_mv * ctx.battery_current_ma / 1000.0;
        PowerEstimate {
            total_mw: power,
            // 按比例拆分（Coulomb法无法精确拆分各组件，用经验比例）
            cpu_mw: power * 0.40,
            gpu_mw: power * 0.25,
            net_mw: power * 0.05,
            screen_mw: power * 0.25,
            base_mw: power * 0.05,
            confidence: if power > 50.0 && power < 12000.0 { 0.9 } else { 0.3 },
            algorithm: "coulomb",
        }
    } else {
        PowerEstimate {
            total_mw: 0.0,
            confidence: 0.0,
            algorithm: "coulomb_unavailable",
        }
    }
}

/// 设备特性（启动时探测一次）
#[derive(Clone, Debug)]
pub struct DeviceProfile {
    pub soc_vendor: SocVendor,
    pub cpu_clusters: Vec<CpuCluster>,
    pub gpu_max_freq_khz: u64,
    pub battery_capacity_mah: u32,
    pub thermal_zone_count: u32,
    pub has_cpu_jiffy: bool,        // /proc/uid_cpustat 是否可用
    pub has_gpu_util: bool,         // GPU utilization sysfs 是否可用
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Default)]
pub enum SocVendor {
    #[default]
    Unknown,
    Qualcomm,  // Snapdragon
    Mediatek,  // Dimensity
    Samsung,   // Exynos
    Google,    // Tensor
}

#[derive(Clone, Debug, Default)]
pub struct CpuCluster {
    pub core_ids: Vec<u32>,        // 该集群包含的核心ID
    pub freq_table_khz: Vec<u64>,  // 可用频率表（升序）
    pub max_freq_khz: u64,
    pub min_freq_khz: u64,
    pub power_table_mw: Vec<f64>,  // 各频率点功耗（mW），与freq_table一一对应
    pub type_label: ClusterType,    // little / mid / big
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Default)]
pub enum ClusterType {
    #[default]
    Little,
    Mid,
    Big,
    Prime,
}

/// 启动时探测设备特性
pub fn detect_device_profile() -> DeviceProfile {
    let mut profile = DeviceProfile {
        soc_vendor: detect_soc_vendor(),
        cpu_clusters: Vec::new(),
        gpu_max_freq_khz: 0,
        battery_capacity_mah: 0,
        thermal_zone_count: 0,
        has_cpu_jiffy: std::path::Path::new("/proc/uid_cpustat").exists(),
        has_gpu_util: false,
    };

    // 探测 CPU 集群和频率表
    profile.cpu_clusters = detect_cpu_clusters();

    // 探测 GPU 最大频率
    profile.gpu_max_freq_khz = detect_gpu_max_freq();
    profile.has_gpu_util = detect_gpu_util_path().is_some();

    // 电池容量
    if let Ok(cap) = std::fs::read_to_string("/sys/class/power_supply/battery/charge_full_design") {
        if let Ok(v) = cap.trim().parse::<u32>() {
            // 微安时 → 毫安时
            profile.battery_capacity_mah = if v > 1_000_000 { v / 1000 } else { v };
        }
    }

    // 温区数量
    for i in 0..20 {
        let path = format!("/sys/class/thermal/thermal_zone{}/temp", i);
        if std::path::Path::new(&path).exists() {
            profile.thermal_zone_count = i + 1;
        } else {
            break;
        }
    }

    profile
}

fn detect_soc_vendor() -> SocVendor {
    // 读取 /proc/cpuinfo 或 /sys/devices/soc0
    if let Ok(soc_id) = std::fs::read_to_string("/sys/devices/soc0/soc_id") {
        let id = soc_id.trim();
        // Qualcomm: 300+ series
        if id.starts_with('3') || id.starts_with("qcom") {
            return SocVendor::Qualcomm;
        }
    }
    // 检查 hardware
    if let Ok(hw) = std::fs::read_to_string("/proc/cpuinfo") {
        let hw_lower = hw.to_lowercase();
        if hw_lower.contains("qualcomm") || hw_lower.contains("snapdragon") {
            return SocVendor::Qualcomm;
        }
        if hw_lower.contains("mediatek") || hw_lower.contains("mt") {
            return SocVendor::Mediatek;
        }
        if hw_lower.contains("exynos") || hw_lower.contains("samsung") {
            return SocVendor::Samsung;
        }
        if hw_lower.contains("google") || hw_lower.contains("tensor") {
            return SocVendor::Google;
        }
    }
    SocVendor::Unknown
}

/// 探测 CPU 集群拓扑和频率表
fn detect_cpu_clusters() -> Vec<CpuCluster> {
    let mut clusters = Vec::new();
    let mut cpu_max_freqs: Vec<(u32, u64)> = Vec::new();

    // 读取每个核心的最大频率（作为集群分类依据）
    for cpu_id in 0..16 {
        let path = format!(
            "/sys/devices/system/cpu/cpu{}/cpufreq/cpuinfo_max_freq",
            cpu_id
        );
        if let Ok(content) = std::fs::read_to_string(&path) {
            if let Ok(freq) = content.trim().parse::<u64>() {
                cpu_max_freqs.push((cpu_id, freq));
            }
        }
    }

    if cpu_max_freqs.is_empty() {
        return clusters;
    }

    // 按最大频率聚类（频率相同的分为同一集群）
    let mut freq_groups: HashMap<u64, Vec<u32>> = HashMap::new();
    for (cpu_id, max_freq) in &cpu_max_freqs {
        freq_groups.entry(*max_freq).or_default().push(*cpu_id);
    }

    let mut sorted_freqs: Vec<u64> = freq_groups.keys().copied().collect();
    sorted_freqs.sort();

    for (idx, &freq) in sorted_freqs.iter().enumerate() {
        let cores = freq_groups.remove(&freq).unwrap_or_default();
        let type_label = match idx {
            0 => ClusterType::Little,
            1 if sorted_freqs.len() == 3 => ClusterType::Mid,
            x if x == sorted_freqs.len() - 1 => ClusterType::Prime,
            _ => ClusterType::Big,
        };

        // 读取该集群的可用频率表
        let freq_table = if let Some(&first_core) = cores.first() {
            read_freq_table(first_core)
        } else {
            Vec::new()
        };

        // 为每个频率点估算功耗（基于动态电压频率关系 P ∝ V²f）
        let power_table = estimate_power_table(&freq_table, freq, type_label);

        clusters.push(CpuCluster {
            core_ids: cores,
            freq_table_khz: freq_table,
            max_freq_khz: freq,
            min_freq_khz: freq_table.first().copied().unwrap_or(freq / 4),
            power_table_mw: power_table,
            type_label,
        });
    }

    clusters
}

/// 读取某核心的可用频率表
fn read_freq_table(cpu_id: u32) -> Vec<u64> {
    let path = format!(
        "/sys/devices/system/cpu/cpu{}/cpufreq/scaling_available_frequencies",
        cpu_id
    );
    if let Ok(content) = std::fs::read_to_string(&path) {
        content
            .split_whitespace()
            .filter_map(|s| s.parse::<u64>().ok())
            .collect()
    } else {
        Vec::new()
    }
}

/// 基于动态电压频率关系估算各频率点功耗
/// P_dynamic = C * V² * f  ≈  P_max * (f/f_max)³  （忽略漏电的简化）
/// P_static (漏电) ≈ P_max * 0.15 （经验比例）
fn estimate_power_table(freq_table: &[u64], max_freq: u64, cluster_type: ClusterType) -> Vec<f64> {
    if freq_table.is_empty() || max_freq == 0 {
        return Vec::new();
    }
    // 各集群的基础功耗上限（mW），基于典型手机 SoC
    let base_power_mw = match cluster_type {
        ClusterType::Little => 200.0,   // 小核集群最高约200mW
        ClusterType::Mid => 600.0,      // 中核集群最高约600mW
        ClusterType::Big => 1200.0,     // 大核集群最高约1200mW
        ClusterType::Prime => 2500.0,   // 超大核最高约2500mW
    };

    freq_table
        .iter()
        .map(|&f| {
            let ratio = f as f64 / max_freq as f64;
            // 动态功耗 ∝ f³ (V ∝ f for DVFS)
            let dynamic = base_power_mw * 0.85 * ratio.powi(3);
            // 静态功耗（漏电）线性随频率变化
            let static_power = base_power_mw * 0.15 * ratio;
            dynamic + static_power
        })
        .collect()
}

fn detect_gpu_max_freq() -> u64 {
    let paths = [
        "/sys/class/kgsl/kgsl-3d0/max_gpuclk",           // Qualcomm
        "/sys/class/kgsl/kgsl-3d0/gpuclk",                // Qualcomm alt
        "/sys/devices/platform/mali.0/devfreq/mali.0/max_freq", // ARM Mali
        "/sys/devices/platform/gpu/devfreq/gpu/max_freq",  // MediaTek
    ];
    for p in &paths {
        if let Ok(content) = std::fs::read_to_string(p) {
            if let Ok(freq) = content.trim().parse::<u64>() {
                return if freq > 1_000_000 { freq } else { freq * 1000 };
            }
        }
    }
    0
}

fn detect_gpu_util_path() -> Option<String> {
    let paths = [
        "/sys/class/kgsl/kgsl-3d0/gpu_busy_percentage",
        "/sys/class/kgsl/kgsl-3d0/gpubusy",
        "/sys/devices/platform/mali.0/devfreq/mali.0/load",
    ];
    for p in &paths {
        if std::path::Path::new(p).exists() {
            return Some(p.to_string());
        }
    }
    None
}

// ---- 算法 2: 基于 SoC 模型的组件级功耗估算 ----
fn estimate_soc_model(ctx: &PowerContext, profile: &DeviceProfile) -> PowerEstimate {
    let mut cpu_mw = 0.0;
    let mut total_freq_weight = 0.0;

    // 按集群估算 CPU 功耗：查找最匹配的频率点功耗
    for cluster in &profile.cpu_clusters {
        for (i, &core_id) in cluster.core_ids.iter().enumerate() {
            if (core_id as usize) < ctx.cpu_freq_khz.len() {
                let freq = ctx.cpu_freq_khz[core_id as usize];
                // 在频率表中插值找功耗
                let power = interpolate_power(freq, &cluster.freq_table_khz, &cluster.power_table_mw);
                cpu_mw += power;
                total_freq_weight += freq as f64;
            }
        }
    }

    // GPU 功耗
    let gpu_mw = if profile.gpu_max_freq_khz > 0 && ctx.gpu_freq_khz > 0 {
        let ratio = ctx.gpu_freq_khz as f64 / profile.gpu_max_freq_khz as f64;
        // GPU 基础功耗从设备特性推算
        let gpu_base = estimate_gpu_base_power(profile);
        gpu_base * ratio.powi(3) * 0.85 + gpu_base * 0.15 * ratio
    } else {
        0.0
    };

    // 网络功耗（基于连接类型的经验模型）
    let net_mw = estimate_network_power(ctx);

    // 屏幕功耗（背光 + 面板）
    let screen_mw = estimate_screen_power(ctx, profile);

    // 基础功耗（SoC 漏电 + DDR + 存储）
    let base_mw = estimate_base_power(profile, ctx.thermal_temp);

    let total = cpu_mw + gpu_mw + net_mw + screen_mw + base_mw;

    PowerEstimate {
        total_mw: total,
        cpu_mw,
        gpu_mw,
        net_mw,
        screen_mw,
        base_mw,
        confidence: if profile.soc_vendor != SocVendor::Unknown { 0.8 } else { 0.5 },
        algorithm: "soc_model",
    }
}

/// 在频率表中插值查找功耗
fn interpolate_power(freq_khz: u64, freq_table: &[u64], power_table: &[f64]) -> f64 {
    if freq_table.is_empty() || power_table.is_empty() {
        return 0.0;
    }
    // 找最近的两个频率点做线性插值
    for i in 0..freq_table.len() {
        if freq_khz <= freq_table[i] {
            if i == 0 {
                return power_table[0] * (freq_khz as f64 / freq_table[0] as f64);
            }
            let ratio = (freq_khz as f64 - freq_table[i - 1] as f64)
                / (freq_table[i] as f64 - freq_table[i - 1] as f64);
            return power_table[i - 1] + ratio * (power_table[i] - power_table[i - 1]);
        }
    }
    // 超出最大频率，按最高点功耗等比放大
    let ratio = freq_khz as f64 / *freq_table.last().unwrap() as f64;
    *power_table.last().unwrap() * ratio.powi(3)
}

/// GPU 基础功耗估算（根据 SoC 类型）
fn estimate_gpu_base_power(profile: &DeviceProfile) -> f64 {
    match profile.soc_vendor {
        SocVendor::Qualcomm => 1500.0,   // Adreno 典型
        SocVendor::Mediatek => 1200.0,   // Mali-G 系列
        SocVendor::Samsung => 1000.0,    // Mali-G 系列
        SocVendor::Google => 1400.0,     // Mali-G710
        SocVendor::Unknown => 1200.0,    // 安全默认值
    }
}

/// 网络功耗估算（基于连接类型和流量）
fn estimate_network_power(ctx: &PowerContext) -> f64 {
    let bytes_per_sec = if ctx.elapsed_secs > 0.0 {
        (ctx.rx_bytes + ctx.tx_bytes) as f64 / ctx.elapsed_secs
    } else {
        0.0
    };

    let kb_per_sec = bytes_per_sec / 1024.0;

    // 各连接类型的基线功耗和每KB增量
    let (base_mw, per_kb_mw) = match ctx.net_type {
        NetType::WiFi => (30.0, 0.02),      // WiFi 基线低，每KB增量小
        NetType::LTE => (80.0, 0.05),       // LTE 基线较高
        NetType::NR5G => (120.0, 0.04),     // 5G 基线高但效率好
        NetType::Ethernet => (10.0, 0.01),  // 有线最低
        NetType::Unknown => (50.0, 0.03),   // 安全默认
    };

    base_mw + kb_per_sec * per_kb_mw
}

/// 屏幕功耗估算
fn estimate_screen_power(ctx: &PowerContext, _profile: &DeviceProfile) -> f64 {
    if ctx.screen_max_brightness == 0 {
        return 500.0; // 无法获取时的合理默认值
    }
    let brightness_ratio = ctx.screen_brightness as f64 / ctx.screen_max_brightness as f64;
    // 背光功耗近似线性于亮度：50mW(最低) ~ 1200mW(最高)
    let backlight = 50.0 + brightness_ratio * 1150.0;
    // 面板基础功耗
    let panel = 200.0;
    backlight + panel
}

/// 基础功耗（SoC漏电+DDR+存储），受温度影响（温度越高漏电越大）
fn estimate_base_power(profile: &DeviceProfile, temp_celsius: f64) -> f64 {
    let base = match profile.soc_vendor {
        SocVendor::Qualcomm => 150.0,
        SocVendor::Mediatek => 130.0,
        SocVendor::Samsung => 140.0,
        SocVendor::Google => 145.0,
        SocVendor::Unknown => 140.0,
    };
    // 漏电随温度指数增长（每升10°C约增加30%）
    let temp_factor = 1.0 + (temp_celsius - 25.0) / 10.0 * 0.3;
    base * temp_factor.max(0.5)
}

// ---- 算法 3: 基于 jiffies 的内核级功耗模型 ----
fn estimate_jiffies_model(ctx: &PowerContext, profile: &DeviceProfile) -> PowerEstimate {
    if !profile.has_cpu_jiffy {
        return PowerEstimate {
            confidence: 0.0,
            algorithm: "jiffies_unavailable",
            ..Default::default()
        };
    }

    let mut cpu_mw = 0.0;
    // 遍历每个集群，用 jiffies 差值 × 该集群当前频率对应的单位能量
    for cluster in &profile.cpu_clusters {
        let core_count = cluster.core_ids.len() as f64;
        if core_count == 0.0 {
            continue;
        }
        // 该集群的平均频率比
        let avg_freq_ratio: f64 = cluster
            .core_ids
            .iter()
            .map(|&core_id| {
                if (core_id as usize) < ctx.cpu_freq_khz.len() {
                    ctx.cpu_freq_khz[core_id as usize] as f64 / cluster.max_freq_khz as f64
                } else {
                    0.5 // 默认50%
                }
            })
            .sum::<f64>()
            / core_count;

        // 该集群的功耗 = jiffies * freq_ratio² * 单集群最大功耗 / 最大jiffies
        // 1 jiffy = 10ms，每秒100 jiffies
        let max_jiffies_per_sec = 100.0 * core_count;
        let jiffies_rate = ctx.cpu_jiffies_delta as f64 / ctx.elapsed_secs.max(0.01);
        let utilization = (jiffies_rate / max_jiffies_per_sec).min(1.0);

        let cluster_max_power = cluster
            .power_table_mw
            .iter()
            .cloned()
            .fold(0.0_f64, f64::max)
            .max(200.0);

        // 功耗 = 静态 + 动态 × utilization × freq_factor
        let static_p = cluster_max_power * 0.15;
        let dynamic_p = cluster_max_power * 0.85 * utilization * avg_freq_ratio.powi(2);
        cpu_mw += static_p + dynamic_p;
    }

    // GPU 估算
    let gpu_mw = if profile.gpu_max_freq_khz > 0 && ctx.gpu_freq_khz > 0 {
        let ratio = ctx.gpu_freq_khz as f64 / profile.gpu_max_freq_khz as f64;
        let gpu_base = estimate_gpu_base_power(profile);
        gpu_base * ratio.powi(3)
    } else {
        0.0
    };

    let net_mw = estimate_network_power(ctx);
    let screen_mw = estimate_screen_power(ctx, profile);
    let base_mw = estimate_base_power(profile, ctx.thermal_temp);
    let total = cpu_mw + gpu_mw + net_mw + screen_mw + base_mw;

    PowerEstimate {
        total_mw: total,
        cpu_mw,
        gpu_mw,
        net_mw,
        screen_mw,
        base_mw,
        confidence: 0.85,
        algorithm: "jiffies_model",
    }
}

// ============================================================
//  Part 2: 温控决策算法族
// ============================================================

/// 温控输入特征
#[derive(Clone, Debug, Default)]
pub struct ThermalInput {
    pub current_temp: f64,           // 当前温度 °C
    pub temp_rate: f64,              // 温度变化率 °C/s（正=升温）
    pub temp_history: [f64; 16],     // 历史温度（环形缓冲）
    pub history_len: u32,            // 有效历史长度
    pub cpu_util_avg: f64,           // CPU 平均利用率 0~1
    pub gpu_util: f64,               // GPU 利用率 0~1
    pub battery_temp: f64,           // 电池温度
    pub power_mw: f64,              // 当前功耗 mW
    pub ambient_temp: f64,           // 环境温度（若可用）
    pub scenario: AppScenario,       // 当前应用场景
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Default)]
pub enum AppScenario {
    #[default]
    Unknown,
    Gaming,
    Video,
    Music,
    Social,
    Browser,
    Idle,
    Charging,
}

/// 温控决策结果
#[derive(Clone, Debug)]
pub struct ThermalDecision {
    pub cpu_freq_limit: f64,         // 0.0~1.0
    pub gpu_freq_limit: f64,         // 0.0~1.0
    pub core_offline: bool,          // 是否关闭中核
    pub force_30hz: bool,            // 是否强制30Hz
    pub reduce_resolution: bool,     // 是否降分辨率
    pub algorithm: &'static str,
    pub confidence: f64,
}

// ---- 温控算法 1: PID 控制器 ----
struct PidController {
    kp: f64,
    ki: f64,
    kd: f64,
    integral: f64,
    prev_error: f64,
    target_temp: f64,
    output_min: f64,
    output_max: f64,
}

impl PidController {
    fn new(kp: f64, ki: f64, kd: f64, target: f64) -> Self {
        Self {
            kp,
            ki,
            kd,
            integral: 0.0,
            prev_error: 0.0,
            target_temp: target,
            output_min: 0.3,  // 最低频率限制30%
            output_max: 1.0,
        }
    }

    fn update(&mut self, current_temp: f64, dt: f64) -> f64 {
        let error = current_temp - self.target_temp;
        self.integral += error * dt;
        // 积分抗饱和
        let max_integral = 10.0;
        self.integral = self.integral.max(-max_integral).min(max_integral);

        let derivative = if dt > 0.0 {
            (error - self.prev_error) / dt
        } else {
            0.0
        };
        self.prev_error = error;

        let output = self.kp * error + self.ki * self.integral + self.kd * derivative;
        // 输出映射到 0.3~1.0（频率限制比例）
        let normalized = 1.0 - (output / 20.0); // 温度偏差20°C时降至最低
        normalized.max(self.output_min).min(self.output_max)
    }

    fn reset(&mut self) {
        self.integral = 0.0;
        self.prev_error = 0.0;
    }
}

// ---- 温控算法 2: 模糊逻辑控制器 ----
fn fuzzy_thermal_control(input: &ThermalInput) -> ThermalDecision {
    // 模糊化：温度 → [冷, 温, 热, 非常热, 过热]
    let temp = input.current_temp;
    let cold = clamp(1.0 - (temp - 30.0) / 10.0);     // 30°C以下=1
    let warm = clamp((temp - 35.0) / 8.0) * clamp((48.0 - temp) / 8.0); // 35~48°C
    let hot = clamp((temp - 43.0) / 5.0) * clamp((52.0 - temp) / 5.0); // 43~52°C
    let very_hot = clamp((temp - 48.0) / 4.0) * clamp((55.0 - temp) / 4.0);
    let overheating = clamp((temp - 52.0) / 3.0);

    // 温度变化率模糊化
    let rate = input.temp_rate;
    let rising_fast = clamp(rate / 2.0);     // >2°C/s = 快速升温
    let rising_slow = clamp((rate + 1.0) / 2.0) * clamp((3.0 - rate) / 2.0);

    // 规则库：输出频率限制
    let mut output = 1.0;
    let mut core_offline = false;
    let mut force_30hz = false;

    // 规则1: 过热 → 强制最低
    if overheating > 0.1 {
        output = output.min(0.3);
        core_offline = true;
        force_30hz = true;
    }
    // 规则2: 非常热 + 快速升温 → 大幅限制
    if very_hot > 0.1 && rising_fast > 0.1 {
        output = output.min(0.45);
        core_offline = true;
    }
    // 规则3: 非常热 + 缓慢升温 → 中度限制
    if very_hot > 0.1 && rising_slow > 0.1 {
        output = output.min(0.55);
    }
    // 规则4: 热 + 快速升温 → 中度限制
    if hot > 0.1 && rising_fast > 0.1 {
        output = output.min(0.65);
    }
    // 规则5: 热 → 轻度限制
    if hot > 0.1 {
        output = output.min(0.75);
    }
    // 规则6: 温热 + 快速升温 → 轻度限制
    if warm > 0.1 && rising_fast > 0.1 {
        output = output.min(0.85);
    }

    // 场景补偿
    let scenario_factor = match input.scenario {
        AppScenario::Gaming => 1.05,    // 游戏允许略高温
        AppScenario::Video => 0.95,     // 视频适度保守
        AppScenario::Music => 0.70,     // 音乐大        AppScenario::Idle => 0.50,       // 空闲大幅限制
        AppScenario::Charging => 0.60,    // 充电适度限制
        _ => 1.0,
    };
    output = (output * scenario_factor).max(0.3).min(1.0);

    ThermalDecision {
        cpu_freq_limit: output,
        gpu_freq_limit: output * 0.95,
        core_offline,
        force_30hz,
        reduce_resolution: overheating > 0.3,
        algorithm: "fuzzy_logic",
        confidence: 0.8,
    }
}

// ---- 温控算法 3: PID 控制器 ----
fn pid_thermal_control(input: &ThermalInput, pid: &mut PidController) -> ThermalDecision {
    let output = pid.update(input.current_temp, 0.1); // dt = 100ms
    ThermalDecision {
        cpu_freq_limit: output,
        gpu_freq_limit: output * 0.9,
        core_offline: output < 0.5,
        force_30hz: output < 0.4,
        reduce_resolution: output < 0.35,
        algorithm: "pid_controller",
        confidence: 0.85,
    }
}

// ---- 温控算法 4: 基于规则的阶梯式温控 ----
fn step_thermal_control(input: &ThermalInput) -> ThermalDecision {
    let temp = input.current_temp;
    let rate = input.temp_rate;

    let (cpu_limit, gpu_limit, core_off, force_30) = if temp > 52.0 {
        (0.30, 0.25, true, true)
    } else if temp > 48.0 {
        (0.45, 0.40, true, false)
    } else if temp > 45.0 && rate > 1.0 {
        (0.55, 0.50, false, false)
    } else if temp > 43.0 {
        (0.70, 0.65, false, false)
    } else if temp > 40.0 && rate > 1.5 {
        (0.80, 0.75, false, false)
    } else {
        (1.0, 1.0, false, false)
    };

    ThermalDecision {
        cpu_freq_limit: cpu_limit,
        gpu_freq_limit: gpu_limit,
        core_offline: core_off,
        force_30hz: force_30,
        reduce_resolution: temp > 55.0,
        algorithm: "step_rules",
        confidence: 0.7,
    }
}

// ============================================================
//  Part 3: Meta-Learner (UCB1 + Thompson Sampling)
// ============================================================

/// 上下文赌博机 — 每个子问题维护一组 arms
#[derive(Clone, Debug)]
pub struct MetaLearner {
    /// 每个 arm 的统计
    arms: Vec<Arm>,
    /// 探索参数 α
    alpha: f64,
    /// 当前选择的 arm 索引
    current_arm: usize,
}

#[derive(Clone, Debug)]
pub struct Arm {
    /// 算法名称
    pub name: &'static str,
    /// 累计奖励
    pub total_reward: f64,
    /// 拉动次数
    pub pulls: u64,
    /// 贝叶斯参数（Thompson Sampling）
    pub alpha_param: f64,  // Beta 分布 α
    pub beta_param: f64,   // Beta 分布 β
}

impl MetaLearner {
    pub fn new(arm_names: &[&'static str], alpha: f64) -> Self {
        let arms = arm_names
            .iter()
            .map(|&name| Arm {
                name,
                total_reward: 0.0,
                pulls: 0,
                alpha_param: 1.0,
                beta_param: 1.0,
            })
            .collect();
        Self {
            arms,
            alpha,
            current_arm: 0,
        }
    }

    /// UCB1 选择
    pub fn select_ucb1(&mut self, total_pulls: u64) -> usize {
        let n = total_pulls as f64;
        let mut best = 0;
        let mut best_score = f64::NEG_INFINITY;

        for (i, arm) in self.arms.iter().enumerate() {
            let avg_reward = if arm.pulls > 0 {
                arm.total_reward / arm.pulls as f64
            } else {
                f64::INFINITY  // 未探索的优先
            };
            let exploration = if arm.pulls > 0 {
                self.alpha * (n.ln() / arm.pulls as f64).sqrt()
            } else {
                f64::INFINITY
            };
            let score = avg_reward + exploration;
            if score > best_score {
                best_score = score;
                best = i;
            }
        }
        self.current_arm = best;
        best
    }

    /// Thompson Sampling 选择（Beta分布采样）
    pub fn select_thompson(&mut self) -> usize {
        let mut best = 0;
        let mut best_sample = f64::NEG_INFINITY;

        for (i, arm) in self.arms.iter().enumerate() {
            // 从 Beta(α, β) 采样
            let sample = beta_sample(arm.alpha_param, arm.beta_param);
            if sample > best_sample {
                best_sample = sample;
                best = i;
            }
        }
        self.current_arm = best;
        best
    }

    /// 更新选定 arm 的奖励（reward 越大越好）
    pub fn update(&mut self, arm_idx: usize, reward: f64) {
        let arm = &mut self.arms[arm_idx];
        arm.pulls += 1;
        arm.total_reward += reward;
        // Thompson Sampling: reward ∈ [0,1] 映射到成功/失败
        arm.alpha_param += reward;
        arm.beta_param += 1.0 - reward;
    }

    /// 获取当前最佳 arm
    pub fn best_arm(&self) -> &Arm {
        &self.arms[self.current_arm]
    }

    /// 获取所有 arm 的平均奖励
    pub fn arm_stats(&self) -> Vec<(&str, f64, u64)> {
        self.arms
            .iter()
            .map(|a| {
                let avg = if a.pulls > 0 {
                    a.total_reward / a.pulls as f64
                } else {
                    0.0
                };
                (a.name, avg, a.pulls)
            })
            .collect()
    }
}

/// Beta 分布采样（使用 Gamma 分布的变换方法）
fn beta_sample(alpha: f64, beta: f64) -> f64 {
    let x = gamma_sample(alpha, 1.0);
    let y = gamma_sample(beta, 1.0);
    x / (x + y)
}

/// Gamma 分布采样（Marsaglia and Tsang's method）
fn gamma_sample(shape: f64, _scale: f64) -> f64 {
    if shape < 1.0 {
        // Ahrens-Dieter method for shape < 1
        return gamma_sample(shape + 1.0, 1.0) * rand_f64().powf(1.0 / shape);
    }
    let d = shape - 1.0 / 3.0;
    let c = 1.0 / (9.0 * d).sqrt();
    loop {
        let x;
        let v;
        loop {
            x = rand_normal();
            v = 1.0 + c * x;
            if v > 0.0 {
                break;
            }
        }
        let v = v * v * v;
        let u = rand_f64();
        if u < 1.0 - 0.0331 * x * x * x * x {
            return d * v;
        }
        if u.ln() < 0.5 * x * x + d * (1.0 - v + v.ln()) {
            return d * v;
        }
    }
}

/// 简单伪随机正态分布（Box-Muller）
fn rand_normal() -> f64 {
    let u1 = rand_f64().max(1e-10);
    let u2 = rand_f64();
    (-2.0 * u1.ln()).sqrt() * (2.0 * std::f64::consts::PI * u2).cos()
}

/// 简单伪随机 [0,1) — 使用 xorshift64
static mut RAND_STATE: u64 = 123456789;
fn rand_f64() -> f64 {
    unsafe {
        RAND_STATE ^= RAND_STATE << 13;
        RAND_STATE ^= RAND_STATE >> 7;
        RAND_STATE ^= RAND_STATE << 17;
        (RAND_STATE as f64) / (u64::MAX as f64)
    }
}

// ============================================================
//  Part 4: 对外统一接口
// ============================================================

/// 功耗估算统一入口
pub fn estimate_power(ctx: &PowerContext, profile: &DeviceProfile, meta: &mut MetaLearner) -> PowerEstimate {
    // UCB1 选择算法
    let total_pulls: u64 = meta.arms.iter().map(|a| a.pulls).sum();
    let arm_idx = meta.select_ucb1(total_pulls);

    let estimate = match meta.arms[arm_idx].name {
        "coulomb" => estimate_coulomb(ctx),
        "soc_model" => estimate_soc_model(ctx, profile),
        "jiffies_model" => estimate_jiffies_model(ctx, profile),
        _ => estimate_soc_model(ctx, profile),
    };

    // 用实际电池数据计算奖励（如果 Coulomb 可用）
    if let Some(ref actual) = estimate_coulomb(ctx).into() {
        let error = (estimate.total_mw - actual.total_mw).abs() / actual.total_mw.max(1.0);
        let reward = (1.0 - error.min(1.0)).max(0.0);
        meta.update(arm_idx, reward);
    }

    estimate
}

/// 温控决策统一入口
pub fn decide_thermal(
    input: &ThermalInput,
    meta: &mut MetaLearner,
    pid: &mut PidController,
) -> ThermalDecision {
    let total_pulls: u64 = meta.arms.iter().map(|a| a.pulls).sum();
    let arm_idx = meta.select_thompson();

    match meta.arms[arm_idx].name {
        "fuzzy" => {
            let decision = fuzzy_thermal_control(input);
            // 奖励 = 温度是否在目标范围
            let in_range = input.current_temp >= 38.0 && input.current_temp <= 45.0;
            meta.update(arm_idx, if in_range { 0.8 } else { 0.3 });
            decision
        }
        "pid" => {
            let decision = pid_thermal_control(input, pid);
            let in_range = input.current_temp >= 38.0 && input.current_temp <= 45.0;
            meta.update(arm_idx, if in_range { 0.8 } else { 0.3 });
            decision
        }
        "step" => {
            let decision = step_thermal_control(input);
            let in_range = input.current_temp >= 38.0 && input.current_temp <= 45.0;
            meta.update(arm_idx, if in_range { 0.8 } else { 0.3 });
            decision
        }
        _ => step_thermal_control(input),
    }
}

// 辅助函数
fn clamp(v: f64) -> f64 {
    v.max(0.0).min(1.0)
}

// ============================================================
//  Part 5: 供 energy.rs 使用的公共类型
// ============================================================

/// 库仑计数器 — 读取电池 V*I 计算实时功耗
#[derive(Debug, Clone)]
pub struct CoulombCounter {
    last_voltage_mv: f64,
    last_current_ma: f64,
    last_update: std::time::Instant,
}

impl CoulombCounter {
    pub fn new() -> Self {
        Self {
            last_voltage_mv: 0.0,
            last_current_ma: 0.0,
            last_update: std::time::Instant::now(),
        }
    }

    /// 读取当前电池电压和电流（mV / mA）
    pub fn read_battery(&mut self) -> (f64, f64) {
        let voltage = Self::read_sysfs("/sys/class/power_supply/battery/voltage_now")
            .map(|v| v / 1000.0) // µV → mV
            .unwrap_or(self.last_voltage_mv);
        let current = Self::read_sysfs("/sys/class/power_supply/battery/current_now")
            .map(|v| v.abs() / 1000.0) // µA → mA, 取绝对值
            .unwrap_or(self.last_current_ma);
        self.last_voltage_mv = voltage;
        self.last_current_ma = current;
        (voltage, current)
    }

    /// 实时功耗 (mW) = voltage(mV) × current(mA) / 1000
    pub fn power_mw(&mut self) -> f64 {
        let (v, i) = self.read_battery();
        if v > 0.0 && i > 0.0 {
            v * i / 1000.0
        } else {
            0.0
        }
    }

    fn read_sysfs(path: &str) -> Option<f64> {
        std::fs::read_to_string(path)
            .ok()
            .and_then(|s| s.trim().parse::<f64>().ok())
    }
}

/// CPU 功耗模型 — 按集群频率查表估算
#[derive(Debug, Clone)]
pub struct CpuPowerModel {
    clusters: Vec<CpuCluster>,
}

impl CpuPowerModel {
    pub fn new(profile: &DeviceProfile) -> Self {
        Self {
            clusters: profile.cpu_clusters.clone(),
        }
    }

    /// 估算 CPU 总功耗 (mW)
    pub fn estimate(&self, cpu_freqs: &[u64]) -> f64 {
        let mut total = 0.0;
        for cluster in &self.clusters {
            for &core_id in &cluster.core_ids {
                let freq = cpu_freqs.get(core_id as usize).copied().unwrap_or(0);
                total += interpolate_power(freq, &cluster.freq_table_khz, &cluster.power_table_mw);
            }
        }
        total
    }
}

/// GPU 功耗模型 — 基于频率比的立方关系
#[derive(Debug, Clone)]
pub struct GpuPowerModel {
    max_freq_khz: u64,
    base_power_mw: f64,
}

impl GpuPowerModel {
    pub fn new(profile: &DeviceProfile) -> Self {
        Self {
            max_freq_khz: profile.gpu_max_freq_khz,
            base_power_mw: estimate_gpu_base_power(profile),
        }
    }

    /// 估算 GPU 功耗 (mW)
    pub fn estimate(&self, gpu_freq_khz: u64) -> f64 {
        if self.max_freq_khz == 0 || gpu_freq_khz == 0 {
            return 0.0;
        }
        let ratio = gpu_freq_khz as f64 / self.max_freq_khz as f64;
        self.base_power_mw * ratio.powi(3)
    }
}

/// 功耗估算器 — 组合各子模型
#[derive(Debug, Clone)]
pub struct PowerEstimator {
    pub cpu: CpuPowerModel,
    pub gpu: GpuPowerModel,
    pub coulomb: CoulombCounter,
    pub screen_base_mw: f64,
    pub net_base_mw: f64,
    pub base_mw: f64,
}

impl PowerEstimator {
    pub fn new(profile: &DeviceProfile) -> Self {
        Self {
            cpu: CpuPowerModel::new(profile),
            gpu: GpuPowerModel::new(profile),
            coulomb: CoulombCounter::new(),
            screen_base_mw: 300.0,
            net_base_mw: 50.0,
            base_mw: estimate_base_power(profile, 35.0),
        }
    }

    /// 综合估算总功耗 (mW)
    pub fn estimate_total(&mut self, ctx: &PowerContext) -> PowerEstimate {
        let cpu_mw = self.cpu.estimate(&ctx.cpu_freq_khz);
        let gpu_mw = self.gpu.estimate(ctx.gpu_freq_khz);
        let net_mw = estimate_network_power(ctx);
        let screen_mw = estimate_screen_power(ctx, &DeviceProfile::default());
        let base_mw = self.base_mw;
        let total = cpu_mw + gpu_mw + net_mw + screen_mw + base_mw;

        PowerEstimate {
            total_mw: total,
            cpu_mw,
            gpu_mw,
            net_mw,
            screen_mw,
            base_mw,
            confidence: 0.75,
            algorithm: "estimator_combined",
        }
    }
}

/// 功耗采样点 — 供能耗排行模块使用
#[derive(Debug, Clone)]
pub struct PowerSample {
    pub timestamp: std::time::Instant,
    pub total_mw: f64,
    pub cpu_mw: f64,
    pub gpu_mw: f64,
    pub net_mw: f64,
    pub screen_mw: f64,
    pub battery_voltage_mv: f64,
    pub battery_current_ma: f64,
}
