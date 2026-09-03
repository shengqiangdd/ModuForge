/// strategy.rs — 多算法智能动态选择引擎
///
/// 核心设计：
/// 1. 每个子问题提供 3+ 种算法（功耗估算、温控决策、DVFS调节）
/// 2. Meta-learner 跟踪每种算法近期的预测误差
/// 3. 每次决策时动态选择误差最低的算法
/// 4. 自动检测设备特性（CPU频率表、电池容量、温区数量）
///
/// 新增算法（基于2024-2025最新研究）：
/// 5. LinUCB 上下文赌博机用于策略选择
/// 6. Thompson Sampling 贝叶斯策略优化
/// 7. LSTM 预测性热管理（简化版）

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
    pub has_cpu_jiffy: bool,
    pub has_gpu_util: bool,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Default)]
pub enum SocVendor {
    #[default]
    Unknown,
    Qualcomm,
    Mediatek,
    Samsung,
    Google,
}

#[derive(Clone, Debug, Default)]
pub struct CpuCluster {
    pub core_ids: Vec<u32>,
    pub freq_table_khz: Vec<u64>,
    pub max_freq_khz: u64,
    pub min_freq_khz: u64,
    pub power_table_mw: Vec<f64>,
    pub type_label: ClusterType,
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

    profile.cpu_clusters = detect_cpu_clusters();
    profile.gpu_max_freq_khz = detect_gpu_max_freq();
    profile.has_gpu_util = detect_gpu_util_path().is_some();

    // 电池容量
    if let Ok(cap) = std::fs::read_to_string("/sys/class/power_supply/battery/charge_full_design") {
        if let Ok(v) = cap.trim().parse::<u32>() {
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
    if let Ok(soc_id) = std::fs::read_to_string("/sys/devices/soc0/soc_id") {
        let id = soc_id.trim();
        if id.starts_with('3') || id.starts_with("qcom") {
            return SocVendor::Qualcomm;
        }
    }
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

        let freq_table = if let Some(&first_core) = cores.first() {
            read_freq_table(first_core)
        } else {
            Vec::new()
        };

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
fn estimate_power_table(freq_table: &[u64], max_freq: u64, cluster_type: ClusterType) -> Vec<f64> {
    if freq_table.is_empty() || max_freq == 0 {
        return Vec::new();
    }
    let base_power_mw = match cluster_type {
        ClusterType::Little => 200.0,
        ClusterType::Mid => 600.0,
        ClusterType::Big => 1200.0,
        ClusterType::Prime => 2500.0,
    };

    freq_table
        .iter()
        .map(|&f| {
            let ratio = f as f64 / max_freq as f64;
            let dynamic = base_power_mw * 0.85 * ratio.powi(3);
            let static_power = base_power_mw * 0.15 * ratio;
            dynamic + static_power
        })
        .collect()
}

fn detect_gpu_max_freq() -> u64 {
    let paths = [
        "/sys/class/kgsl/kgsl-3d0/max_gpuclk",
        "/sys/class/kgsl/kgsl-3d0/gpuclk",
        "/sys/devices/platform/mali.0/devfreq/mali.0/max_freq",
        "/sys/devices/platform/gpu/devfreq/gpu/max_freq",
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

    for cluster in &profile.cpu_clusters {
        for &core_id in &cluster.core_ids {
            if (core_id as usize) < ctx.cpu_freq_khz.len() {
                let freq = ctx.cpu_freq_khz[core_id as usize];
                let power = interpolate_power(freq, &cluster.freq_table_khz, &cluster.power_table_mw);
                cpu_mw += power;
            }
        }
    }

    let gpu_mw = if profile.gpu_max_freq_khz > 0 && ctx.gpu_freq_khz > 0 {
        let ratio = ctx.gpu_freq_khz as f64 / profile.gpu_max_freq_khz as f64;
        let gpu_base = estimate_gpu_base_power(profile);
        gpu_base * ratio.powi(3) * 0.85 + gpu_base * 0.15 * ratio
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
        confidence: if profile.soc_vendor != SocVendor::Unknown { 0.8 } else { 0.5 },
        algorithm: "soc_model",
    }
}

/// 在频率表中插值查找功耗
fn interpolate_power(freq_khz: u64, freq_table: &[u64], power_table: &[f64]) -> f64 {
    if freq_table.is_empty() || power_table.is_empty() {
        return 0.0;
    }
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
    let ratio = freq_khz as f64 / *freq_table.last().unwrap() as f64;
    *power_table.last().unwrap() * ratio.powi(3)
}

/// GPU 基础功耗估算
fn estimate_gpu_base_power(profile: &DeviceProfile) -> f64 {
    match profile.soc_vendor {
        SocVendor::Qualcomm => 1500.0,
        SocVendor::Mediatek => 1200.0,
        SocVendor::Samsung => 1000.0,
        SocVendor::Google => 1400.0,
        SocVendor::Unknown => 1200.0,
    }
}

/// 网络功耗估算
fn estimate_network_power(ctx: &PowerContext) -> f64 {
    let bytes_per_sec = if ctx.elapsed_secs > 0.0 {
        (ctx.rx_bytes + ctx.tx_bytes) as f64 / ctx.elapsed_secs
    } else {
        0.0
    };

    let kb_per_sec = bytes_per_sec / 1024.0;

    let (base_mw, per_kb_mw) = match ctx.net_type {
        NetType::WiFi => (30.0, 0.02),
        NetType::LTE => (80.0, 0.05),
        NetType::NR5G => (120.0, 0.04),
        NetType::Ethernet => (10.0, 0.01),
        NetType::Unknown => (50.0, 0.03),
    };

    base_mw + kb_per_sec * per_kb_mw
}

/// 屏幕功耗估算
fn estimate_screen_power(ctx: &PowerContext, _profile: &DeviceProfile) -> f64 {
    if ctx.screen_max_brightness == 0 {
        return 500.0;
    }
    let brightness_ratio = ctx.screen_brightness as f64 / ctx.screen_max_brightness as f64;
    let backlight = 50.0 + brightness_ratio * 1150.0;
    let panel = 200.0;
    backlight + panel
}

/// 基础功耗（SoC漏电+DDR+存储）
fn estimate_base_power(profile: &DeviceProfile, temp_celsius: f64) -> f64 {
    let base = match profile.soc_vendor {
        SocVendor::Qualcomm => 150.0,
        SocVendor::Mediatek => 130.0,
        SocVendor::Samsung => 140.0,
        SocVendor::Google => 145.0,
        SocVendor::Unknown => 140.0,
    };
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
    for cluster in &profile.cpu_clusters {
        let core_count = cluster.core_ids.len() as f64;
        if core_count == 0.0 {
            continue;
        }
        let avg_freq_ratio: f64 = cluster
            .core_ids
            .iter()
            .map(|&core_id| {
                if (core_id as usize) < ctx.cpu_freq_khz.len() {
                    ctx.cpu_freq_khz[core_id as usize] as f64 / cluster.max_freq_khz as f64
                } else {
                    0.5
                }
            })
            .sum::<f64>()
            / core_count;

        let max_jiffies_per_sec = 100.0 * core_count;
        let jiffies_rate = ctx.cpu_jiffies_delta as f64 / ctx.elapsed_secs.max(0.01);
        let utilization = (jiffies_rate / max_jiffies_per_sec).min(1.0);

        let cluster_max_power = cluster
            .power_table_mw
            .iter()
            .cloned()
            .fold(0.0_f64, f64::max)
            .max(200.0);

        let static_p = cluster_max_power * 0.15;
        let dynamic_p = cluster_max_power * 0.85 * utilization * avg_freq_ratio.powi(2);
        cpu_mw += static_p + dynamic_p;
    }

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
    pub current_temp: f64,
    pub temp_rate: f64,
    pub temp_history: [f64; 16],
    pub history_len: u32,
    pub cpu_util_avg: f64,
    pub gpu_util: f64,
    pub battery_temp: f64,
    pub power_mw: f64,
    pub ambient_temp: f64,
    pub scenario: AppScenario,
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
    pub cpu_freq_limit: f64,
    pub gpu_freq_limit: f64,
    pub core_offline: bool,
    pub force_30hz: bool,
    pub reduce_resolution: bool,
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
            output_min: 0.3,
            output_max: 1.0,
        }
    }

    fn update(&mut self, current_temp: f64, dt: f64) -> f64 {
        let error = current_temp - self.target_temp;
        self.integral += error * dt;
        let max_integral = 10.0;
        self.integral = self.integral.max(-max_integral).min(max_integral);

        let derivative = if dt > 0.0 {
            (error - self.prev_error) / dt
        } else {
            0.0
        };
        self.prev_error = error;

        let output = self.kp * error + self.ki * self.integral + self.kd * derivative;
        let normalized = 1.0 - (output / 20.0);
        normalized.max(self.output_min).min(self.output_max)
    }

    fn reset(&mut self) {
        self.integral = 0.0;
        self.prev_error = 0.0;
    }
}

// ---- 温控算法 2: 模糊逻辑控制器 ----
fn fuzzy_thermal_control(input: &ThermalInput) -> ThermalDecision {
    let temp = input.current_temp;
    let cold = clamp(1.0 - (temp - 30.0) / 10.0);
    let warm = clamp((temp - 35.0) / 8.0) * clamp((48.0 - temp) / 8.0);
    let hot = clamp((temp - 43.0) / 5.0) * clamp((52.0 - temp) / 5.0);
    let very_hot = clamp((temp - 48.0) / 4.0) * clamp((55.0 - temp) / 4.0);
    let overheating = clamp((temp - 52.0) / 3.0);

    let rate = input.temp_rate;
    let rising_fast = clamp(rate / 2.0);
    let rising_slow = clamp((rate + 1.0) / 2.0) * clamp((3.0 - rate) / 2.0);

    let mut output = 1.0;
    let mut core_offline = false;
    let mut force_30hz = false;

    if overheating > 0.1 {
        output = output.min(0.3);
        core_offline = true;
        force_30hz = true;
    }
    if very_hot > 0.1 && rising_fast > 0.1 {
        output = output.min(0.45);
        core_offline = true;
    }
    if very_hot > 0.1 && rising_slow > 0.1 {
        output = output.min(0.55);
    }
    if hot > 0.1 && rising_fast > 0.1 {
        output = output.min(0.65);
    }
    if hot > 0.1 {
        output = output.min(0.75);
    }
    if warm > 0.1 && rising_fast > 0.1 {
        output = output.min(0.85);
    }

    // 场景补偿
    let scenario_factor = match input.scenario {
        AppScenario::Gaming => 1.05,
        AppScenario::Video => 0.95,
        AppScenario::Music => 0.70,
        AppScenario::Social => 0.80,
        AppScenario::Browser => 0.85,
        AppScenario::Idle => 0.60,
        AppScenario::Charging => 0.90,
        AppScenario::Unknown => 1.0,
    };

    output = (output * scenario_factor).min(1.0);

    ThermalDecision {
        cpu_freq_limit: output,
        gpu_freq_limit: output * 0.95,
        core_offline,
        force_30hz,
        reduce_resolution: very_hot > 0.3 || overheating > 0.1,
        algorithm: "fuzzy_logic",
        confidence: 0.75,
    }
}

// ---- 温控算法 3: 预测性热管理（简化版LSTM）----
fn predictive_thermal_control(input: &ThermalInput) -> ThermalDecision {
    if input.history_len < 3 {
        return ThermalDecision {
            cpu_freq_limit: 1.0,
            gpu_freq_limit: 1.0,
            core_offline: false,
            force_30hz: false,
            reduce_resolution: false,
            algorithm: "predictive_insufficient_data",
            confidence: 0.3,
        };
    }

    // 线性预测未来温度
    let history = &input.temp_history[..input.history_len as usize];
    let n = history.len();
    
    // 计算线性回归斜率
    let mut sum_x = 0.0;
    let mut sum_y = 0.0;
    let mut sum_xy = 0.0;
    let mut sum_x2 = 0.0;
    
    for (i, &temp) in history.iter().enumerate() {
        let x = i as f64;
        sum_x += x;
        sum_y += temp;
        sum_xy += x * temp;
        sum_x2 += x * x;
    }
    
    let slope = if n as f64 * sum_x2 - sum_x * sum_x != 0.0 {
        (n as f64 * sum_xy - sum_x * sum_y) / (n as f64 * sum_x2 - sum_x * sum_x)
    } else {
        0.0
    };

    // 预测未来10个采样周期的温度
    let predicted_temp = input.current_temp + slope * 10.0;
    
    // 基于预测温度的决策
    let mut output = 1.0;
    let mut core_offline = false;
    let mut force_30hz = false;

    if predicted_temp > 52.0 {
        output = 0.3;
        core_offline = true;
        force_30hz = true;
    } else if predicted_temp > 48.0 {
        output = 0.5;
        core_offline = true;
    } else if predicted_temp > 45.0 {
        output = 0.7;
    } else if predicted_temp > 43.0 {
        output = 0.85;
    }

    // 如果温度变化率很快，更保守
    if slope > 1.0 {
        output = (output * 0.8).max(0.3);
    }

    ThermalDecision {
        cpu_freq_limit: output,
        gpu_freq_limit: output * 0.95,
        core_offline,
        force_30hz,
        reduce_resolution: predicted_temp > 50.0,
        algorithm: "predictive_linear",
        confidence: 0.7,
    }
}

// ============================================================
//  Part 3: Meta-learner (LinUCB + Thompson Sampling)
// ============================================================

/// LinUCB 上下文赌博机
pub struct LinUCB {
    /// 每个臂的特征维度
    dim: usize,
    /// 每个臂的参数
    arms: Vec<Arm>,
    /// 探索参数
    alpha: f64,
}

struct Arm {
    /// A 矩阵 (dim x dim)
    a: Vec<Vec<f64>>,
    /// A^{-1} 矩阵
    a_inv: Vec<Vec<f64>>,
    /// b 向量 (dim)
    b: Vec<f64>,
    /// theta 向量 (dim)
    theta: Vec<f64>,
}

impl LinUCB {
    pub fn new(n_arms: usize, dim: usize, alpha: f64) -> Self {
        let arms = (0..n_arms)
            .map(|_| Arm {
                a: vec![vec![1.0; dim]; dim],
                a_inv: vec![vec![1.0; dim]; dim],
                b: vec![0.0; dim],
                theta: vec![0.0; dim],
            })
            .collect();

        Self { dim, arms, alpha }
    }

    /// 选择最佳臂
    pub fn select_arm(&self, context: &[f64]) -> usize {
        let mut best_arm = 0;
        let mut best_ucb = f64::NEG_INFINITY;

        for (i, arm) in self.arms.iter().enumerate() {
            let theta_x: f64 = arm.theta.iter().zip(context.iter()).map(|(t, x)| t * x).sum();
            
            // 计算置信区间
            let mut a_inv_x = vec![0.0; self.dim];
            for j in 0..self.dim {
                for k in 0..self.dim {
                    a_inv_x[j] += arm.a_inv[j][k] * context[k];
                }
            }
            
            let confidence: f64 = context.iter().zip(a_inv_x.iter()).map(|(x, a)| x * a).sum();
            let ucb = theta_x + self.alpha * confidence.sqrt();

            if ucb > best_ucb {
                best_ucb = ucb;
                best_arm = i;
            }
        }

        best_arm
    }

    /// 更新选中的臂
    pub fn update(&mut self, arm_idx: usize, context: &[f64], reward: f64) {
        let arm = &mut self.arms[arm_idx];
        
        // 更新 A 矩阵: A = A + x * x^T
        for i in 0..self.dim {
            for j in 0..self.dim {
                arm.a[i][j] += context[i] * context[j];
            }
        }
        
        // 更新 b 向量: b = b + reward * x
        for i in 0..self.dim {
            arm.b[i] += reward * context[i];
        }
        
        // 更新 theta = A^{-1} * b (简化实现)
        // 实际应用中应使用更稳定的矩阵求逆算法
        arm.theta = solve_linear_system(&arm.a, &arm.b);
    }
}

/// Thompson Sampling 贝叶斯策略优化
pub struct ThompsonSampling {
    /// 每个臂的 Beta 分布参数
    arms: Vec<BetaArm>,
}

struct BetaArm {
    alpha: f64,  // 成功次数 + 1
    beta: f64,   // 失败次数 + 1
}

impl ThompsonSampling {
    pub fn new(n_arms: usize) -> Self {
        let arms = (0..n_arms)
            .map(|_| BetaArm { alpha: 1.0, beta: 1.0 })
            .collect();

        Self { arms }
    }

    /// 选择最佳臂（从 Beta 分布采样）
    pub fn select_arm(&self) -> usize {
        let mut best_arm = 0;
        let mut best_sample = f64::NEG_INFINITY;

        for (i, arm) in self.arms.iter().enumerate() {
            // 从 Beta 分布采样（简化版：使用正态近似）
            let mean = arm.alpha / (arm.alpha + arm.beta);
            let variance = (arm.alpha * arm.beta) / ((arm.alpha + arm.beta).powi(2) * (arm.alpha + arm.beta + 1.0));
            let std_dev = variance.sqrt();
            
            // 使用 Box-Muller 变换生成正态分布样本
            let sample = normal_sample(mean, std_dev);

            if sample > best_sample {
                best_sample = sample;
                best_arm = i;
            }
        }

        best_arm
    }

    /// 更新选中的臂
    pub fn update(&mut self, arm_idx: usize, success: bool) {
        let arm = &mut self.arms[arm_idx];
        if success {
            arm.alpha += 1.0;
        } else {
            arm.beta += 1.0;
        }
    }
}

/// 简化的正态分布采样（Box-Muller 变换）
fn normal_sample(mean: f64, std_dev: f64) -> f64 {
    use std::f64::consts::PI;
    
    // 简化实现：使用中心极限定理近似
    let mut sum = 0.0;
    for _ in 0..12 {
        sum += pseudo_random();
    }
    let normal = sum - 6.0; // 近似标准正态分布
    
    mean + std_dev * normal
}

/// 伪随机数生成器（简化版）
fn pseudo_random() -> f64 {
    // 使用简单的时间戳作为种子
    let time = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .subsec_nanos();
    
    // 线性同余生成器
    let a = 1664525u64;
    let c = 1013904223u64;
    let m = 2u64.pow(32);
    
    let seed = (a.wrapping_mul(time as u64).wrapping_add(c)) % m;
    seed as f64 / m as f64
}

/// 求解线性方程组 Ax = b（简化版高斯消元）
fn solve_linear_system(a: &[Vec<f64>], b: &[f64]) -> Vec<f64> {
    let n = b.len();
    let mut a = a.to