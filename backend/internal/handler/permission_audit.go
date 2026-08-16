package handler

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

type PermissionAuditHandler struct {
	db *sql.DB
	fr *service.FileContentRepo // S3-first content access (optional)
}

func NewPermissionAuditHandler(db *sql.DB) *PermissionAuditHandler {
	return &PermissionAuditHandler{db: db}
}

// SetFileContentRepo injects the S3-first file content repository.
func (h *PermissionAuditHandler) SetFileContentRepo(fr *service.FileContentRepo) {
	h.fr = fr
}

// Android permission classification
var permissionDB = map[string]struct {
	Level       string `json:"level"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}{
	// Dangerous permissions (high risk)
	"WRITE_SECURE_SETTINGS":          {Level: "dangerous", Category: "system", Description: "允许修改系统安全设置", Risk: "可绕过安全限制"},
	"MODIFY_PHONE_STATE":             {Level: "dangerous", Category: "phone", Description: "允许修改电话状态", Risk: "可监听/拦截电话"},
	"INSTALL_PACKAGES":               {Level: "dangerous", Category: "system", Description: "允许安装应用", Risk: "可静默安装恶意应用"},
	"DELETE_PACKAGES":                {Level: "dangerous", Category: "system", Description: "允许删除应用", Risk: "可删除系统关键应用"},
	"SET_ANIMATION_SCALE":            {Level: "dangerous", Category: "display", Description: "允许设置动画缩放", Risk: "影响用户体验"},
	"WRITE_SETTINGS":                 {Level: "dangerous", Category: "system", Description: "允许写入系统设置", Risk: "可修改系统配置"},
	"READ_PHONE_STATE":               {Level: "dangerous", Category: "phone", Description: "读取电话状态（IMEI等）", Risk: "可追踪设备"},
	"ACCESS_FINE_LOCATION":           {Level: "dangerous", Category: "location", Description: "获取精确位置", Risk: "可精确追踪位置"},
	"ACCESS_COARSE_LOCATION":         {Level: "dangerous", Category: "location", Description: "获取粗略位置", Risk: "可追踪大致位置"},
	"ACCESS_BACKGROUND_LOCATION":     {Level: "dangerous", Category: "location", Description: "后台获取位置", Risk: "持续追踪位置"},
	"CAMERA":                         {Level: "dangerous", Category: "camera", Description: "访问摄像头", Risk: "可拍摄照片/视频"},
	"RECORD_AUDIO":                   {Level: "dangerous", Category: "microphone", Description: "录音", Risk: "可录制音频"},
	"READ_CONTACTS":                  {Level: "dangerous", Category: "contacts", Description: "读取联系人", Risk: "可获取联系人信息"},
	"WRITE_CONTACTS":                 {Level: "dangerous", Category: "contacts", Description: "写入联系人", Risk: "可修改联系人"},
	"READ_SMS":                       {Level: "dangerous", Category: "sms", Description: "读取短信", Risk: "可获取短信内容"},
	"SEND_SMS":                       {Level: "dangerous", Category: "sms", Description: "发送短信", Risk: "可发送付费短信"},
	"CALL_PHONE":                     {Level: "dangerous", Category: "phone", Description: "拨打电话", Risk: "可拨打付费电话"},
	"READ_CALL_LOG":                  {Level: "dangerous", Category: "phone", Description: "读取通话记录", Risk: "可获取通话历史"},
	"WRITE_CALL_LOG":                 {Level: "dangerous", Category: "phone", Description: "写入通话记录", Risk: "可修改通话记录"},
	"READ_CALENDAR":                  {Level: "dangerous", Category: "calendar", Description: "读取日历", Risk: "可获取日程信息"},
	"WRITE_CALENDAR":                 {Level: "dangerous", Category: "calendar", Description: "写入日历", Risk: "可修改日程"},
	"READ_MEDIA_IMAGES":              {Level: "dangerous", Category: "storage", Description: "读取图片", Risk: "可获取用户照片"},
	"READ_MEDIA_VIDEO":               {Level: "dangerous", Category: "storage", Description: "读取视频", Risk: "可获取用户视频"},
	"READ_MEDIA_AUDIO":               {Level: "dangerous", Category: "storage", Description: "读取音频", Risk: "可获取用户音乐"},
	"MANAGE_EXTERNAL_STORAGE":        {Level: "dangerous", Category: "storage", Description: "管理所有文件", Risk: "可访问所有文件"},
	"READ_EXTERNAL_STORAGE":          {Level: "dangerous", Category: "storage", Description: "读取外部存储", Risk: "可读取用户文件"},
	"WRITE_EXTERNAL_STORAGE":         {Level: "dangerous", Category: "storage", Description: "写入外部存储", Risk: "可修改用户文件"},
	"ACCESS_MEDIA_LOCATION":          {Level: "dangerous", Category: "storage", Description: "读取媒体位置信息", Risk: "可获取照片拍摄位置"},

	// Normal permissions (low risk)
	"INTERNET":                       {Level: "normal", Category: "network", Description: "访问网络", Risk: "标准网络访问"},
	"ACCESS_NETWORK_STATE":           {Level: "normal", Category: "network", Description: "获取网络状态", Risk: "低风险"},
	"ACCESS_WIFI_STATE":              {Level: "normal", Category: "network", Description: "获取 WiFi 状态", Risk: "低风险"},
	"CHANGE_WIFI_STATE":              {Level: "normal", Category: "network", Description: "修改 WiFi 状态", Risk: "可能影响网络连接"},
	"BLUETOOTH":                      {Level: "normal", Category: "bluetooth", Description: "使用蓝牙", Risk: "低风险"},
	"BLUETOOTH_ADMIN":                {Level: "normal", Category: "bluetooth", Description: "管理蓝牙", Risk: "低风险"},
	"VIBRATE":                        {Level: "normal", Category: "hardware", Description: "振动", Risk: "低风险"},
	"WAKE_LOCK":                      {Level: "normal", Category: "system", Description: "防止休眠", Risk: "可能影响电池"},
	"RECEIVE_BOOT_COMPLETED":         {Level: "normal", Category: "system", Description: "开机启动", Risk: "低风险"},
	"FOREGROUND_SERVICE":             {Level: "normal", Category: "system", Description: "前台服务", Risk: "低风险"},
	"POST_NOTIFICATIONS":             {Level: "normal", Category: "system", Description: "发送通知", Risk: "低风险"},
	"SYSTEM_ALERT_WINDOW":            {Level: "normal", Category: "system", Description: "显示悬浮窗", Risk: "可能影响其他应用"},
	"SET_WALLPAPER":                  {Level: "normal", Category: "display", Description: "设置壁纸", Risk: "低风险"},
	"READ_SYNC_SETTINGS":             {Level: "normal", Category: "system", Description: "读取同步设置", Risk: "低风险"},
	"WRITE_SYNC_SETTINGS":            {Level: "normal", Category: "system", Description: "写入同步设置", Risk: "低风险"},
	"NFC":                            {Level: "normal", Category: "nfc", Description: "使用 NFC", Risk: "低风险"},
	"USE_BIOMETRIC":                  {Level: "normal", Category: "security", Description: "使用生物识别", Risk: "低风险"},
	"USE_FINGERPRINT":                {Level: "normal", Category: "security", Description: "使用指纹", Risk: "低风险"},
	"ACCESS_NOTIFICATION_POLICY":     {Level: "normal", Category: "system", Description: "访问通知策略", Risk: "低风险"},
	"SCHEDULE_EXACT_ALARM":           {Level: "normal", Category: "system", Description: "设置精确闹钟", Risk: "可能影响电池"},
	"REQUEST_IGNORE_BATTERY_OPTIMIZATIONS": {Level: "normal", Category: "system", Description: "忽略电池优化", Risk: "可能影响电池"},
	"RECEIVE_SMS":                    {Level: "normal", Category: "sms", Description: "接收短信", Risk: "低风险"},
	"RECEIVE_MMS":                    {Level: "normal", Category: "sms", Description: "接收彩信", Risk: "低风险"},
	"RECEIVE_WAP_PUSH":               {Level: "normal", Category: "sms", Description: "接收 WAP 推送", Risk: "低风险"},
}

// Module common permissions (expected for Magisk modules)
var moduleCommonPermissions = map[string]bool{
	"INTERNET":              true,
	"ACCESS_NETWORK_STATE":  true,
	"WAKE_LOCK":             true,
	"RECEIVE_BOOT_COMPLETED": true,
	"FOREGROUND_SERVICE":    true,
	"SYSTEM_ALERT_WINDOW":   true,
	"WRITE_SETTINGS":        true,
	"READ_PHONE_STATE":      true,
	"ACCESS_WIFI_STATE":     true,
	"BLUETOOTH":             true,
	"BLUETOOTH_ADMIN":       true,
	"VIBRATE":               true,
}

type PermissionInfo struct {
	Name        string `json:"name"`
	Level       string `json:"level"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
	IsCommon    bool   `json:"is_common"`
	Expected    bool   `json:"expected"`
}

type AuditResult struct {
	TotalPermissions   int              `json:"total_permissions"`
	DangerousCount     int              `json:"dangerous_count"`
	NormalCount        int              `json:"normal_count"`
	UnknownCount       int              `json:"unknown_count"`
	CommonCount        int              `json:"common_count"`
	UncommonCount      int              `json:"uncommon_count"`
	RiskScore          int              `json:"risk_score"`
	RiskLevel          string           `json:"risk_level"`
	Permissions        []PermissionInfo `json:"permissions"`
	DangerousPerms     []PermissionInfo `json:"dangerous_permissions"`
	Warnings           []string         `json:"warnings"`
}

// AuditModulePermissions audits a module's permissions
// POST /projects/:id/audit
func (h *PermissionAuditHandler) AuditModulePermissions(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return BadRequest(c, "项目 ID 不能为空")
	}

	// Read project files (S3 first)
	files, err := h.fr.ReadAllContent(c.Context(), projectID)
	if err != nil {
		return InternalError(c, "读取项目文件失败")
	}

	// Extract permissions from code and manifest
	permSet := make(map[string]bool)
	for _, content := range files {
		// Look for Android permissions in various formats
		// AndroidManifest.xml: android:name="android.permission.XXX"
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			// Match android.permission.XXX
			if idx := strings.Index(line, "android.permission."); idx >= 0 {
				rest := line[idx+len("android.permission."):]
				end := strings.IndexAny(rest, "\"' <")
				if end > 0 {
					perm := "android.permission." + rest[:end]
					permSet[perm] = true
				}
			}
			// Match uses-permission without android. prefix
			if strings.Contains(line, "uses-permission") {
				if idx := strings.Index(line, "\""); idx >= 0 {
					rest := line[idx+1:]
					endIdx := strings.Index(rest, "\"")
					if endIdx > 0 {
						p := rest[:endIdx]
						if strings.HasPrefix(p, "android.permission.") {
							permSet[p] = true
						}
					}
				}
			}
			// Code-level permission checks: checkSelfPermission, requestPermissions
			if strings.Contains(line, "checkSelfPermission") || strings.Contains(line, "requestPermissions") {
				for perm := range permissionDB {
					if strings.Contains(line, perm) {
						permSet[perm] = true
					}
				}
			}
		}
	}

	// Build result
	result := AuditResult{
		Permissions: make([]PermissionInfo, 0),
	}

	for permName := range permSet {
		// Get short name (without prefix)
		shortName := permName
		if strings.HasPrefix(permName, "android.permission.") {
			shortName = permName[len("android.permission."):]
		}

		info, exists := permissionDB[shortName]
		perm := PermissionInfo{
			Name:     permName,
			IsCommon: moduleCommonPermissions[shortName],
			Expected: moduleCommonPermissions[shortName],
		}

		if exists {
			perm.Level = info.Level
			perm.Category = info.Category
			perm.Description = info.Description
			perm.Risk = info.Risk
		} else {
			perm.Level = "unknown"
			perm.Category = "unknown"
			perm.Description = "未在权限数据库中"
			perm.Risk = "未知"
			result.UnknownCount++
		}

		result.Permissions = append(result.Permissions, perm)

		switch perm.Level {
		case "dangerous":
			result.DangerousCount++
			result.DangerousPerms = append(result.DangerousPerms, perm)
		case "normal":
			result.NormalCount++
		}

		if perm.IsCommon {
			result.CommonCount++
		} else {
			result.UncommonCount++
		}
	}

	// Calculate risk score
	total := len(result.Permissions)
	result.TotalPermissions = total
	if total == 0 {
		result.RiskScore = 100
		result.RiskLevel = "safe"
		return c.JSON(result)
	}

	riskScore := 100
	riskScore -= result.DangerousCount * 12
	riskScore -= result.UnknownCount * 5
	riskScore -= result.UncommonCount * 2
	if riskScore < 0 {
		riskScore = 0
	}
	result.RiskScore = riskScore

	switch {
	case riskScore >= 80:
		result.RiskLevel = "safe"
	case riskScore >= 60:
		result.RiskLevel = "low"
	case riskScore >= 40:
		result.RiskLevel = "medium"
	case riskScore >= 20:
		result.RiskLevel = "high"
	default:
		result.RiskLevel = "critical"
	}

	// Generate warnings
	for _, perm := range result.DangerousPerms {
		if !perm.Expected {
			result.Warnings = append(result.Warnings,
				"请求了非常见危险权限: "+perm.Name+" - "+perm.Description)
		}
	}
	if result.DangerousCount > 5 {
		result.Warnings = append(result.Warnings,
			"请求了过多危险权限 ("+strconv.Itoa(result.DangerousCount)+")，请确认是否必要")
	}

	// Save audit result
	resultsJSON, _ := json.Marshal(result.Permissions)
	h.db.Exec(
		`INSERT INTO permission_audits (module_id, project_id, total_permissions, dangerous_count, risk_score, results)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		projectID, projectID, total, result.DangerousCount, riskScore, string(resultsJSON))

	return c.JSON(result)
}

// GetModulePermissions returns the stored permission audit results
// GET /projects/:id/permissions
func (h *PermissionAuditHandler) GetModulePermissions(c fiber.Ctx) error {
	projectID := c.Params("id")
	if projectID == "" {
		return BadRequest(c, "项目 ID 不能为空")
	}

	rows, err := h.db.Query(
		`SELECT id, total_permissions, dangerous_count, risk_score, results, audited_at
		 FROM permission_audits WHERE project_id=? ORDER BY audited_at DESC LIMIT 10`, projectID)
	if err != nil {
		return InternalError(c, "查询审计结果失败")
	}
	defer rows.Close()

	type AuditRecord struct {
		ID                int64            `json:"id"`
		TotalPermissions  int              `json:"total_permissions"`
		DangerousCount    int              `json:"dangerous_count"`
		RiskScore         int              `json:"risk_score"`
		Permissions       []PermissionInfo `json:"permissions"`
		AuditedAt         string           `json:"audited_at"`
	}

	var records []AuditRecord
	for rows.Next() {
		var r AuditRecord
		var resultsJSON string
		if err := rows.Scan(&r.ID, &r.TotalPermissions, &r.DangerousCount, &r.RiskScore, &resultsJSON, &r.AuditedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(resultsJSON), &r.Permissions)
		if r.Permissions == nil {
			r.Permissions = []PermissionInfo{}
		}
		records = append(records, r)
	}
	if records == nil {
		records = []AuditRecord{}
	}

	return c.JSON(fiber.Map{"audits": records})
}
