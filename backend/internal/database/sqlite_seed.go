package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// SeedGlossary seeds the glossary table with predefined terms.
func (db *DB) SeedGlossary() error {
	terms := []struct {
		term, definition, category, related string
	}{
		{"Module", "模块是 ModuForge 中的基本功能单元，可用于扩展或修改 Android 系统的行为。", "general", "ADB,Build,Widget"},
		{"ADB", "Android Debug Bridge，用于与 Android 设备通信的命令行工具。", "dev", "Device,Shell,Debug"},
		{"Build（构建）", "将源代码编译为可分发的模块包的过程。", "dev", "Module,CI/CD,ZIP"},
		{"Widget", "仪表盘上的可自定义显示组件，用于展示特定信息。", "general", "Dashboard,Analytics"},
		{"CI/CD", "持续集成与持续部署，自动化构建和发布流程。", "dev", "Build,Webhook,Auto"},
		{"Webhook", "当特定事件发生时触发的 HTTP 回调，用于通知外部系统。", "dev", "CI/CD,API,Git"},
		{"API Key", "用于程序化访问 ModuForge API 的认证令牌。", "security", "Auth,JWT,Token"},
		{"2FA（两步验证）", "通过额外验证码增强账户安全的认证方式。", "security", "TOTP,Auth,Security"},
		{"Changelog", "记录模块每个版本变更内容的文档。", "general", "Version,Module,Update"},
		{"Vulnerability（漏洞）", "代码中可能被利用的安全缺陷。", "security", "Security,Scan,Audit"},
		{"TOTP", "基于时间的一次性密码，用于双因素认证。", "security", "2FA,Auth,Secret"},
		{"Screenshot（截图）", "模块功能的可视化展示图片。", "general", "Module,Gallery,Preview"},
		{"Benchmark（基准测试）", "测量和评估设备性能的标准化测试。", "dev", "Performance,Device,Test"},
		{"Deploy（部署）", "将构建产物安装到目标设备的过程。", "dev", "Build,Install,Device"},
		{"Plugin（插件）", "可扩展 ModuForge 功能的附加组件。", "general", "Extension,Hook,Module"},
		{"Audit Log（审计日志）", "记录系统中重要操作事件的日志。", "security", "Security,Log,Tracking"},
		{"Provider（提供商）", "提供 AI 模型 API 的服务商配置。", "ai", "AI,LLM,Model"},
		{"Prompt（提示词）", "发送给 AI 模型以引导其输出的指令文本。", "ai", "AI,Generation,Template"},
		{"Rollback（回滚）", "将模块恢复到之前版本的操作。", "general", "Version,Restore,Backup"},
		{"License（许可证）", "定义模块使用、修改和分发条款的法律文件。", "general", "Module,Legal,Open Source"},
	}
	for _, t := range terms {
		db.Conn.Exec("INSERT OR IGNORE INTO glossary (term, definition, category, related_terms) VALUES (?, ?, ?, ?)", t.term, t.definition, t.category, t.related)
	}
	log.Printf("[DB] Seeded %d glossary terms", len(terms))
	return nil
}

// SeedAdminUser ensures an admin user exists in the database.
func (db *DB) SeedAdminUser() error {
	// 确保存在至少一个 admin 用户（即使已有其他用户）
	var adminCount int
	db.Conn.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&adminCount)
	if adminCount > 0 {
		return nil // 已有管理员，跳过
	}

	// 默认管理员凭据，可通过环境变量覆盖
	username := getEnvOrDefault("ADMIN_USERNAME", "admin")
	email := getEnvOrDefault("ADMIN_EMAIL", "admin@moduforge.local")
	password := getEnvOrDefault("ADMIN_PASSWORD", "admin123")

	if password == "admin123" && os.Getenv("ADMIN_PASSWORD") == "" {
		log.Printf("[DB] WARNING: using default admin password 'admin123'. Set ADMIN_PASSWORD env var in production.")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	// 检查用户名是否已存在
	var existingUser int
	db.Conn.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&existingUser)
	if existingUser > 0 {
		// 用户已存在但不是 admin，提升为 admin
		_, err = db.Conn.Exec("UPDATE users SET role = 'admin' WHERE username = ?", username)
		if err != nil {
			return fmt.Errorf("promote admin user: %w", err)
		}
		log.Printf("[DB] Promoted existing user '%s' to admin", username)
		return nil
	}

	_, err = db.Conn.Exec(
		`INSERT INTO users (id, username, email, password_hash, role, email_verified)
		 VALUES (?, ?, ?, ?, 'admin', 1)`,
		uuid.New().String(), username, email, string(hash),
	)
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Printf("[DB] Seeded admin user: %s (password set via env or default)", username)
	return nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// SeedMarketData seeds the market_modules table with sample data.
func (db *DB) SeedMarketData() error {
	var count int
	db.Conn.QueryRow("SELECT COUNT(*) FROM market_modules").Scan(&count)
	if count > 0 {
		now := time.Now()
		db.Conn.Exec("UPDATE market_modules SET created_at = ?, updated_at = ? WHERE created_at = '0001-01-01 00:00:00' OR created_at IS NULL", now, now)
		seedStars := map[string]int{"mod_0001": 128, "mod_0002": 89, "mod_0003": 156, "mod_0004": 234, "mod_0005": 312, "mod_0006": 198, "mod_0007": 76, "mod_0008": 456, "mod_0009": 45, "mod_0010": 67}
		seedInstalls := map[string]int{"mod_0001": 3500, "mod_0002": 2100, "mod_0003": 4200, "mod_0004": 5800, "mod_0005": 7600, "mod_0006": 4500, "mod_0007": 1800, "mod_0008": 12000, "mod_0009": 900, "mod_0010": 2300}
		for id, stars := range seedStars {
			db.Conn.Exec("UPDATE market_modules SET stars = ?, installs = ? WHERE id = ? AND stars = 0", stars, seedInstalls[id], id)
		}
		return nil
	}

	seeds := []struct {
		id, title, slug, desc, cat, tags, ver, author, lic string
		stars, installs                                    int
	}{
		{"mod_0001", "System Prop Tweaks", "system-prop-tweaks", "Comprehensive system property modifications for performance and battery optimization.", "system", "system,prop,performance", "v2.1", "ModuForge Team", "MIT", 128, 3500},
		{"mod_0002", "Custom Boot Animation", "boot-animation", "Replace default boot animation with custom designs.", "ui", "boot,animation,custom", "v1.3", "DevMaster", "Apache-2.0", 89, 2100},
		{"mod_0003", "Audio Enhancement", "audio-enhance", "Improve audio quality with custom DAC configurations.", "audio", "audio,dac,equalizer", "v1.8", "SoundModder", "GPL-3.0", 156, 4200},
		{"mod_0004", "GPU Overclock Pro", "gpu-overclock", "Safe GPU frequency adjustments for better gaming.", "display", "gpu,overclock,gaming", "v1.5", "GameTuner", "MIT", 234, 5800},
		{"mod_0005", "Network Firewall", "network-firewall", "Per-app network access control with ad blocking.", "utility", "network,firewall,adblock", "v2.0", "PrivacyGuard", "GPL-3.0", 312, 7600},
		{"mod_0006", "Battery Saver Max", "battery-saver", "Intelligent battery management with Doze optimization.", "system", "battery,doze,performance", "v1.4", "BatteryPro", "MIT", 198, 4500},
		{"mod_0007", "Display Calibrator", "display-calibrate", "Professional display calibration with ICC profiles.", "display", "display,calibrate,color", "v1.2", "ColorExpert", "MIT", 76, 1800},
		{"mod_0008", "Hosts AdBlock", "hosts-adblock", "Hosts file based ad blocker with auto-update.", "utility", "adblock,hosts,privacy", "v3.0", "AdGuardFork", "GPL-3.0", 456, 12000},
		{"mod_0009", "Magisk Manager Lite", "magisk-lite", "Lightweight Magisk module management alternative.", "system", "magisk,manager,lite", "v1.1", "LiteDev", "Apache-2.0", 45, 900},
		{"mod_0010", "Notification Sound Pack", "notification-sounds", "50+ notification sounds organized by category.", "ui", "notification,sounds,ringtones", "v1.6", "SoundPack", "CC-BY-4.0", 67, 2300},
	}

	stmt, err := db.Conn.Prepare("INSERT INTO market_modules (id, title, slug, description, category, tags, version, version_code, author, license, stars, installs, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, s := range seeds {
		_, err := stmt.Exec(s.id, s.title, s.slug, s.desc, s.cat, s.tags, s.ver, 0, s.author, s.lic, s.stars, s.installs, now, now)
		if err != nil {
			return fmt.Errorf("seed %s: %w", s.title, err)
		}
	}

	log.Printf("[DB] Seeded %d market modules\n", len(seeds))
	return nil
}
