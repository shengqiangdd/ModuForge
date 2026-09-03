//! 多算法智能动态选择引擎 (Multi-Algorithm Intelligent Selector)
//!
//! 替换所有硬编码/简化公式，提供多套算法并在运行时根据预测误差自适应切换。
//!
//! # Architecture
//! ```text
//! ┌─────────────────────────────────────────────────────┐
//! │              StrategySelector (meta-learner)         │
//! │  tracks prediction_error per algorithm, switches    │
//! │  to best-performing variant for current conditions  │
//! └────────┬──────────────┬──────────────┬──────────────┘
//!          │              │              │
//!  ┌───────▼──────┐ ┌────▼─────┐ ┌─────▼──────┐
//!  │ EnergyModels │ │ThermalAlg│ │ DvfsAlg    │
//!  │ - Coulomb    │ │ - PID    │ │ - Linear   │
//!  │ - Utilization│ │ - EMA    │ │ - PowerBudget│
//!  │ - Network    │ │ - LinReg │ │ - ThermalFB│
//!  │ - GPU        │ │ - Arrhenius│ │- PerCluster│
//!  └──────────────┘ └──────────┘ └────────────┘
//! ```

use std::collections::HashMap;

// ============================================================================
// 元学习器：跟踪每个算法的预测误差，动态选择最优算法
// ============================================================================

/// 算法性能追踪器
#[derive(Debug, Clone)]
struct AlgorithmStats {
    /// 指数加权移动平均误差 (EMA of absolute prediction error)
    ema_error: f64,
    /// 累计预测次数
    samples: u64,
    /// 最近 N 次误差的滑动窗口（用于检测突变）
    recent_errors: Vec<f64>,
    /// 算法在当前条件下的权重 (0.0~1.0)
    weight: f64,
}

impl AlgorithmStats {
    fn new() -> Self {
        Self {
            ema_error: 1.0, // 初始假设中等误差
            samples: 0,
            recent_errors: Vec::with_capacity(32),
            weight: 1.0 / 4.0, // 初始均等权重
        }
    }

    /// 更新误差观察，EMA alpha = 2/(N+1)，N=10
    fn update(&mut self, error: f64) {
        self.samples += 1;
        let alpha = 2.0 / (self.samples.min(10) as f64 + 1.0);
        self.ema_error = alpha * error + (1.0 - alpha) * self.ema_error;

        self.recent_errors.push(error);
        if self.recent_errors.len() > 32 {
            self.recent_errors.remove(0);
        }
    }

    /// 计算分数 (越低越好)，融合 EMA 误差 + 最近稳定性
    fn score(&self) -> f64 {
        if self.samples < 5 {
            return 0.5; // 样本不足，给中等分数
        }
        let stability = if self.recent_errors.len() >= 8 {
            let recent_avg: f64 =
                self.recent_errors.iter().rev().take(8).sum::<f64>() / 8.0;
            recent_avg
        } else {
            self.ema_error
        };
        // 融合：70% EMA + 30% 最近稳定性
        0.7 * self.ema_error + 0.3 * stability
    }
}

/// 元学习器：为每组候选算法维护统计，动态选择最优
#[derive(Debug)]
pub struct StrategySelector {
    /// algorithm_name -> stats
    stats: HashMap<String, AlgorithmStats>,
    /// 当前选中的算法名
    current: String,
    /// 切换冷却期（防止频繁切换）
    cooldown: u32,
    cooldown_remaining: u32,
}

impl StrategySelector {
    pub fn new(candidates: &[&str], initial: &str) -> Self {
        let mut stats = HashMap::new();
        for name in candidates {
            stats.insert(name.to_string(), AlgorithmStats::new());
        }
        Self {
            stats,
            current: initial.to_string(),
            cooldown: 20, // 20 个采样周期内不重复切换
            cooldown_remaining: 0,
        }
    }

    /// 记录当前算法的预测误差
    pub fn record_error(&mut self, algorithm: &str, error: f64) {
        if let Some(s) = self.stats.get_mut(algorithm) {
            s.update(error);
        }
    }

    /// 选择当前最优算法（带冷却期保护）
    pub fn select_best(&mut self) -> String {
        if self.cooldown_remaining > 0 {
            self.cooldown_remaining -= 1;
            return self.current.clone();
        }

        // 找到分数最低的算法
        let mut best_name = self.current.clone();
        let mut best_score = self
            .stats
            .get(&self.current)
            .map(|s| s.score())
            .unwrap_or(f64::MAX);

        for (name, stats) in &self.stats {
            let s = stats.score();
            if s < best_score - 0.05 {
                // 需要显著优于当前才切换 (5% hysteresis)
                best_score = s;
                best_name = name.clone();
            }
        }

        if best_name != self.current {
            self.current = best_name.clone();
            self.cooldown_remaining = self.cooldown;
        }

        self.current.clone()
    }

    /// 获取当前算法名称
    pub fn current(&self) -> &str {
        &self.current
    }

    /// 获取所有算法的诊断信息
    pub fn diagnostics(&self) -> Vec<(String, f64, f64, u64)> {
        self.stats
            .iter()
            .map(|(name, s)| (name.clone(), s.ema_error, s.weight, s.samples))
            .collect()
    }
}

// ============================================================================
// 能耗估算算法族
// ============================================================================

/// 能耗采样输入（来自系统监控）
#[derive(Debug, Clone, Default)]
pub struct EnergyInput {
    /// CPU 时间增量 (jiffies, 1 jiffy = 10ms)
    pub cpu_jiffies_delta: u64,
    /// CPU 频率 (kHz)
    pub cpu_freq_khz: u64,
    /// CPU 核心数
    pub cpu_cores: u32,
    /// GPU 时间增量 (ms)
    pub gpu_ms_delta: f64,
    /// GPU 频率 (Hz)
    pub gpu_freq_hz: f64,
    /// 网络接收增量 (bytes)
    pub net_rx_delta: u64,
    /// 网络发送增量 (bytes)
    pub net_tx_delta: u64,
    /// 网络类型 ("wifi" / "4g" / "5g")
    pub net_type: String,
    /// 显示亮度 (0.0~1.0)
    pub display_brightness: f64,
    /// 屏幕刷新率 (Hz)
    pub display_refresh_hz: f64,
    /// 电池电压 (mV)
    pub battery_voltage_mv: f64,
    /// 电池电流 (mA, 放电为正)
    pub battery_current_ma: f64,
    /// IO 延迟增量 (ms)
    pub io_delay_ms_delta: f64,
    /// 采样间隔 (秒)
    pub elapsed_secs: f64,
}

/// 能耗算法输出
#[derive(Debug, Clone)]
pub struct EnergyOutput {
    /// 总功耗估算 (mW)
    pub total_mw: f64,
    /// CPU 功耗 (mW)
    pub cpu_mw: f64,
    /// GPU 功耗 (mW)
    pub gpu_mw: f64,
    /// 网络功耗 (mW)
    pub net_mw: f64,
    /// 显示功耗 (mW)
    pub display_mw: f64,
    /// IO 功耗 (mW)
    pub io_mw: f64,
    /// 使用的算法名
    pub algorithm: String,
    /// 置信度 (0.0~1.0)
    pub confidence: f64,
}

// --- 算法 1: 库仑计数法 (Coulomb Counting) ---
// 直接用电池 V*I，最准确但需要硬件支持
fn energy_coulomb(input: &EnergyInput) -> EnergyOutput {
    let total_mw = if input.battery_current_ma > 0.0 && input.battery_voltage_mv > 0.0 {
        // P = V * I (mV * mA / 1000 = mW)
        input.battery_voltage_mv * input.battery_current_ma / 1000.0
    } else {
        0.0
    };

    // 无法分解各组件，用比例估算
    let cpu_ratio = estimate_cpu_power_ratio(input);
    let gpu_ratio = estimate_gpu_power_ratio(input);

    EnergyOutput {
        total_mw,
        cpu_mw: total_mw * cpu_ratio,
        gpu_mw: total_mw * gpu_ratio,
        net_mw: total_mw * 0.05, // 网络约 5%
        display_mw: total_mw * estimate_display_power_ratio(input),
        io_mw: total_mw * 0.03,
        algorithm: "coulomb".into(),
        confidence: if total_mw > 10.0 { 0.9 } else { 0.3 },
    }
}

// --- 算法 2: 利用率模型 (Utilization-Based) ---
// 基于 CPU/GPU 利用率 × 频率 × 电压平方 的物理模型
fn energy_utilization(input: &EnergyInput) -> EnergyOutput {
    if input.elapsed_secs <= 0.0 {
        return EnergyOutput {
            algorithm: "utilization".into(),
            ..Default::default()
        };
    }

    // CPU 功耗: P_cpu = N * C * V^2 * f
    // 简化: 利用率 * 核心数 * 频率系数
    let cpu_util = (input.cpu_jiffies_delta as f64 * 10.0)
        / (input.elapsed_secs * 1000.0 * input.cpu_cores as f64);
    let cpu_util = cpu_util.clamp(0.0, 1.0);
    // 动态功耗 ∝ f * V^2 ≈ f^3 (DVFS 下 V ∝ f)
    let freq_ratio = input.cpu_freq_khz as f64 / 2_000_000.0; // 归一化到 2GHz
    let cpu_dynamic_mw = cpu_util * input.cpu_cores as f64 * 50.0 * freq_ratio.powi(3);
    let cpu_static_mw = input.cpu_cores as f64 * 20.0; // 每核静态漏电 ~20mW
    let cpu_mw = (cpu_dynamic_mw + cpu_static_mw).clamp(0.0, 8000.0);

    // GPU 功耗
    let gpu_util = if input.gpu_ms_delta > 0.0 {
        (input.gpu_ms_delta / (input.elapsed_secs * 1000.0)).clamp(0.0, 1.0)
    } else {
        0.0
    };
    let gpu_freq_ratio = input.gpu_freq_hz / 800_000_000.0; // 归一化到 800MHz
    let gpu_mw = (gpu_util * 200.0 * gpu_freq_ratio.powi(2)).clamp(0.0, 3000.0);

    // 网络功耗 (基于连接类型)
    let net_bytes = input.net_rx_delta + input.net_tx_delta;
    let net_power_per_byte = match input.net_type.as_str() {
        "5g" => 0.0008,  // 0.8 mJ/KB
        "4g" => 0.0005,  // 0.5 mJ/KB
        "wifi" => 0.0001, // 0.1 mJ/KB
        _ => 0.0003,
    };
    let net_mw = (net_bytes as f64 * net_power_per_byte / input.elapsed_secs).clamp(0.0, 500.0);

    // 显示功耗 (基于亮度和刷新率)
    let display_mw = estimate_display_power_mw(input);

    // IO 功耗
    let io_mw = (input.io_delay_ms_delta * 0.5).clamp(0.0, 200.0);

    let total_mw = cpu_mw + gpu_mw + net_mw + display_mw + io_mw;

    EnergyOutput {
        total_mw,
        cpu_mw,
        gpu_mw,
        net_mw,
        display_mw,
        io_mw,
        algorithm: "utilization".into(),
        confidence: 0.7,
    }
}

// --- 算法 3: 混合回归模型 (Hybrid Regression) ---
// 使用多项式回归拟合历史 V*I 数据与各传感器的映射关系
fn energy_hybrid_regression(input: &EnergyInput) -> EnergyOutput {
    // 基于多特征线性回归: P = w0 + w1*cpu_util + w2*gpu_util + w3*net_rate + w4*brightness^2
    // 权重来自离线训练（典型 Android 设备）
    let cpu_util = if input.elapsed_secs > 0.0 {
        ((input.cpu_jiffies_delta as f64 * 10.0)
            / (input.elapsed_secs * 1000.0 * input.cpu_cores as f64))
            .clamp(0.0, 1.0)
    } else {
        0.0
    };

    let gpu_util = if input.elapsed_secs > 0.0 {
        (input.gpu_ms_delta / (input.elapsed_secs * 1000.0)).clamp(0.0, 1.0)
    } else {
        0.0
    };

    let net_rate_kbps = if input.elapsed_secs > 0.0 {
        ((input.net_rx_delta + input.net_tx_delta) as f64 / input.elapsed_secs / 1024.0)
    } else {
        0.0
    };

    // 预训练权重 (Snapdragon 8 Gen 系列典型值)
    let p_base = 80.0; // 基础功耗 (SoC 漏电 + 子系统)
    let w_cpu = 1800.0; // CPU 利用率系数
    let w_cpu2 = 600.0; // CPU 利用率二次项 (非线性)
    let w_gpu = 1200.0; // GPU 利用率系数
    let w_net = 0.8; // 网络速率系数 (mW per KB/s)
    let w_disp = 150.0; // 显示基础功耗
    let w_bright = 350.0; // 亮度系数 (满亮度额外 350mW)
    let w_io = 0.3; // IO 延迟系数

    let total_mw = p_base
        + w_cpu * cpu_util
        + w_cpu2 * cpu_util * cpu_util
        + w_gpu * gpu_util
        + w_net * net_rate_kbps
        + w_disp
        + w_bright * input.display_brightness
        + w_io * input.io_delay_ms_delta;

    let cpu_mw = p_base * 0.3 + w_cpu * cpu_util + w_cpu2 * cpu_util * cpu_util;
    let gpu_mw = w_gpu * gpu_util;
    let net_mw = w_net * net_rate_kbps;
    let display_mw = w_disp + w_bright * input.display_brightness;
    let io_mw = w_io * input.io_delay_ms_delta;

    EnergyOutput {
        total_mw: total_mw.clamp(0.0, 12000.0),
        cpu_mw: cpu_mw.clamp(0.0, 8000.0),
        gpu_mw: gpu_mw.clamp(0.0, 3000.0),
        net_mw: net_mw.clamp(0.0, 500.0),
        display_mw: display_mw.clamp(0.0, 1500.0),
        io_mw: io_mw.clamp(0.0, 200.0),
        algorithm: "hybrid_regression".into(),
        confidence: 0.75,
    }
}

// --- 算法 4: 自适应指数模型 (Adaptive Exponential) ---
// 用指数衰减加权历史样本，适应设备老化和负载变化
fn energy_adaptive_exponential(
    input: &EnergyInput,
    history: &[(f64, f64)], // (total_mw, elapsed_secs)
) -> EnergyOutput {
    let base = energy_utilization(input);

    if history.is_empty() {
        return base;
    }

    // 指数加权移动平均修正
    let alpha = 0.3; // 遗忘因子
    let mut weighted_sum = 0.0;
    let mut weight_sum = 0.0;
    for (i, &(mw, _)) in history.iter().rev().enumerate() {
        let w = alpha.powi(i as i32);
        weighted_sum += mw * w;
        weight_sum += w;
    }
    let historical_avg = if weight_sum > 0.0 {
        weighted_sum / weight_sum
    } else {
        base.total_mw
    };

    // 混合: 60% 当前模型 + 40% 历史趋势
    let corrected_mw = 0.6 * base.total_mw + 0.4 * historical_avg;

    let ratio = if base.total_mw > 1.0 {
        corrected_mw / base.total_mw
    } else {
        1.0
    };

    EnergyOutput {
        total_mw: corrected_mw,
        cpu_mw: base.cpu_mw * ratio,
        gpu_mw: base.gpu_mw * ratio,
        net_mw: base.net_mw * ratio,
        display_mw: base.display_mw * ratio,
        io_mw: base.io_mw * ratio,
        algorithm: "adaptive_exponential".into(),
        confidence: 0.85,
    }
}

// ============================================================================
// 温控算法族
// ============================================================================

/// 温控输入
#[derive(Debug, Clone, Default)]
pub struct ThermalInput {
    /// 当前温度 (°C)
    pub current_temp: f64,
    /// 温度变化率 (°C/s)
    pub temp_rate: f64,
    /// CPU 负载 (0.0~1.0)
    pub cpu_load: f64,
    /// 电池温度 (°C)
    pub battery_temp: f64,
    /// 充电状态
    pub is_charging: bool,
    /// 环境温度估算 (°C)
    pub ambient_temp: f64,
    /// 采样间隔 (秒)
    pub elapsed_secs: f64,
    /// 场景 ("gaming" / "video" / "social" / "idle")
    pub scenario: String,
}

/// 温控输出
#[derive(Debug, Clone)]
pub struct ThermalOutput {
    /// 推荐 CPU 频率限制 (0.0~1.0)
    pub cpu_freq_limit: f64,
    /// 推荐 GPU 频率限制 (0.0~1.0)
    pub gpu_freq_limit: f64,
    /// 是否强制 30Hz
    pub force_30hz: bool,
    /// 是否关闭中核
    pub middle_cores_offline: bool,
    /// 紧急程度 (0.0~1.0)
    pub urgency: f64,
    /// 使用的算法名
    pub algorithm: String,
    /// 预测温度 (°C)
    pub predicted_temp: f64,
}

// --- 算法 A: PID 控制器 ---
fn thermal_pid(
    input: &ThermalInput,
    state: &mut ThermalPidState,
) -> ThermalOutput {
    let target_temp = 42.0; // 目标温度
    let error = input.current_temp - target_temp;

    // PID 参数 (自适应：温度越高，P 越激进)
    let kp = if input.current_temp > 48.0 { 0.15 } else { 0.08 };
    let ki = 0.02;
    let kd = 0.05;

    state.integral += error * input.elapsed_secs;
    state.integral = state.integral.clamp(-10.0, 10.0); // 积分限幅

    let derivative = if state.prev_error.is_finite() {
        (error - state.prev_error) / input.elapsed_secs.max(0.01)
    } else {
        0.0
    };
    state.prev_error = error;

    let output = kp * error + ki * state.integral + kd * derivative;
    // output > 0 → 需要降频，output < 0 → 可以升频
    let freq_limit = (1.0 - output).clamp(0.3, 1.0);

    // 预测: 线性外推 3 秒后的温度
    let predicted_temp = input.current_temp + input.temp_rate * 3.0;

    ThermalOutput {
        cpu_freq_limit: freq_limit,
        gpu_freq_limit: (freq_limit * 1.05).min(1.0), // GPU 稍宽松
        force_30hz: predicted_temp > 52.0,
        middle_cores_offline: predicted_temp > 50.0,
        urgency: ((input.current_temp - 40.0) / 15.0).clamp(0.0, 1.0),
        algorithm: "pid".into(),
        predicted_temp,
    }
}

/// PID 控制器内部状态
#[derive(Debug, Default)]
pub struct ThermalPidState {
    integral: f64,
    prev_error: f64,
}

// --- 算法 B: 指数加权移动平均 + 预测 (EMA-Predictive) ---
fn thermal_ema_predictive(
    input: &ThermalInput,
    state: &mut ThermalEmaState,
) -> ThermalOutput {
    // 自适应 alpha：温度变化快时降低 alpha (更关注近期)
    let base_alpha = 0.3;
    let alpha = if input.temp_rate.abs() > 1.0 {
        base_alpha * 0.5 // 变化快 → 更平滑
    } else {
        base_alpha * 1.5 // 变化慢 → 更灵敏
    };

    state.ema_temp = alpha * input.current_temp + (1.0 - alpha) * state.ema_temp;
    state.ema_rate = alpha * input.temp_rate + (1.0 - alpha) * state.ema_rate;

    // 二次预测: T(t+3) = T + rate*3 + 0.5*accel*9
    let accel = (input.temp_rate - state.prev_rate) / input.elapsed_secs.max(0.01);
    state.prev_rate = input.temp_rate;
    let predicted_temp = state.ema_temp + state.ema_rate * 3.0 + 0.5 * accel * 9.0;

    // 查表法: 温度区间 → 频率限制
    let freq_limit = match state.ema_temp {
        t if t < 38.0 => 1.0,
        t if t < 42.0 => 1.0 - (t - 38.0) * 0.05, // 1.0 → 0.8
        t if t < 46.0 => 0.8 - (t - 42.0) * 0.1, // 0.8 → 0.4
        t if t < 50.0 => 0.4 - (t - 46.0) * 0.075, // 0.4 → 0.1
        _ => 0.3,
    };

    ThermalOutput {
        cpu_freq_limit: freq_limit.clamp(0.3, 1.0),
        gpu_freq_limit: (freq_limit * 1.1).clamp(0.3, 1.0),
        force_30hz: predicted_temp > 52.0,
        middle_cores_offline: state.ema_temp > 48.0,
        urgency: ((state.ema_temp - 38.0) / 15.0).clamp(0.0, 1.0),
        algorithm: "ema_predictive".into(),
        predicted_temp,
    }
}

/// EMA 状态
#[derive(Debug, Default)]
pub struct ThermalEmaState {
    ema_temp: f64,
    ema_rate: f64,
    prev_rate: f64,
}

// --- 算法 C: 场景感知模糊控制 (Fuzzy Scene-Aware) ---
fn thermal_fuzzy_scene(input: &ThermalInput) -> ThermalOutput {
    // 多维模糊推理: 温度 × 负载 × 场景 → 频率限制
    let temp_membership = fuzzy_temperature(input.current_temp);
    let load_membership = fuzzy_load(input.cpu_load);

    // 场景权重 (不同场景对性能/温度的偏好不同)
    let (perf_weight, thermal_weight) = match input.scenario.as_str() {
        "gaming" => (0.7, 0.3), // 游戏偏性能
        "video" => (0.5, 0.5), // 视频均衡
        "social" => (0.4, 0.6), // 社交偏省电
        "music" => (0.2, 0.8), // 音乐偏省电
        "idle" => (0.1, 0.9), // 空闲极端省电
        _ => (0.5, 0.5),
    };

    // 模糊规则融合
    let hot_factor = temp_membership.2; // "hot" 隶属度
    let heavy_factor = load_membership.2; // "heavy" 隶属度

    let perf_pressure = perf_weight * heavy_factor;
    let thermal_pressure = thermal_weight * hot_factor;

    let raw_limit = 1.0 - thermal_pressure * 0.7 - perf_pressure * 0.2;
    let freq_limit = raw_limit.clamp(0.3, 1.0);

    // 环境温度补偿：高温环境降低目标
    let ambient_offset = if input.ambient_temp > 35.0 {
        (input.ambient_temp - 35.0) * 0.02
    } else {
        0.0
    };

    let freq_limit = (freq_limit - ambient_offset).clamp(0.3, 1.0);
    let predicted_temp = input.current_temp + input.temp_rate * 3.0;

    ThermalOutput {
        cpu_freq_limit: freq_limit,
        gpu_freq_limit: (freq_limit * 1.08).clamp(0.3, 1.0),
        force_30hz: predicted_temp > 52.0 || input.current_temp > 50.0,
        middle_cores_offline: input.current_temp > 48.0 && input.cpu_load > 0.7,
        urgency: ((input.current_temp - 38.0) / 15.0).clamp(0.0, 1.0),
        algorithm: "fuzzy_scene".into(),
        predicted_temp,
    }
}

/// 模糊隶属函数: 温度
/// 返回 (cold, warm, hot) 隶属度
fn fuzzy_temperature(temp: f64) -> (f64, f64, f64) {
    let cold = if temp < 40.0 { 1.0 } else if temp < 44.0 { (44.0 - temp) / 4.0 } else { 0.0 };
    let warm = if temp < 38.0 {
        0.0
    } else if temp < 42.0 {
        (temp - 38.0) / 4.0
    } else if temp < 48.0 {
        (48.0 - temp) / 6.0
    } else {
        0.0
    };
    let hot = if temp < 44.0 {
        0.0
    } else if temp < 50.0 {
        (temp - 44.0) / 6.0
    } else {
        1.0
    };
    (cold, warm, hot)
}

/// 模糊隶属函数: CPU 负载
fn fuzzy_load(load: f64) -> (f64, f64, f64) {
    let light = if load < 0.3 { 1.0 } else if load < 0.6 { (0.6 - load) / 0.3 } else { 0.0 };
    let medium = if load < 0.2 {
        0.0
    } else if load < 0.5 {
        (load - 0.2) / 0.3
    } else if load < 0.8 {
        (0.8 - load) / 0.3
    } else {
        0.0
    };
    let heavy = if load < 0.6 {
        0.0
    } else if load < 0.9 {
        (load - 0.6) / 0.3
    } else {
        1.0
    };
    (light, medium, heavy)
}

// ============================================================================
// DVFS 调频算法族
// ============================================================================

/// DVFS 输入
#[derive(Debug, Clone, Default)]
pub struct DvfsInput {
    /// 当前 CPU 温度 (°C)
    pub temp: f64,
    /// CPU 负载 (0.0~1.0)
    pub cpu_load: f64,
    /// GPU 负载 (0.0~1.0)
    pub gpu_load: f64,
    /// 帧率 (fps)
    pub fps: f64,
    /// 目标帧率
    pub target_fps: f64,
    /// 功耗预算 (mW, 0=不限)
    pub power_budget_mw: f64,
    /// 当前功耗 (mW)
    pub current_power_mw: f64,
    /// 电池电量 (0~100)
    pub battery_level: i32,
    /// 充电状态
    pub is_charging: bool,
    /// 场景
    pub scenario: String,
}

/// DVFS 输出
#[derive(Debug, Clone)]
pub struct DvfsOutput {
    /// CPU 频率限制 (0.0~1.0)
    pub cpu_freq_limit: f64,
    /// GPU 频率限制 (0.0~1.0)
    pub gpu_freq_limit: f64,
    /// CPU 核心策略 ("all" / "big_only" / "little_only")
    pub core_policy: String,
    /// 使用的算法
    pub algorithm: String,
}

// --- 算法 I: 负载跟踪 (Load Tracking) ---
fn dvfs_load_tracking(input: &DvfsInput) -> DvfsOutput {
    // 根据负载动态调频，带迟滞防止抖动
    let cpu_limit = if input.cpu_load > 0.85 {
        1.0 // 高负载不限制
    