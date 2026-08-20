package handler

import (
	"github.com/moduforge/backend/internal/service"
)

func registerDeviceRoutes(ctx *routeContext) {
	db := ctx.db

	adbSvc := service.NewADBService(db.Conn)
	adbH := NewADBHandler(adbSvc)
	benchH := NewBenchmarkHandler(adbSvc)
	mirrorH := NewMirrorHandler(adbSvc)
	screenshotH := NewScreenshotHandler(adbSvc)

	benchmarkSvc := service.NewBenchmarkService(db.Conn)
	benchmarkAPIH := NewBenchmarkAPIHandler(benchmarkSvc, adbSvc)

	updateSvc := service.NewUpdateService(db.Conn)
	updateH := NewUpdateHandler(updateSvc)

	// ── Public ADB ──
	ctx.api.Get("/adb/check", adbH.CheckADB)
	ctx.api.Get("/adb/status", adbH.GetServerStatus)
	ctx.api.Get("/adb/devices", ctx.jwtMW, adbH.ListDevices)
	ctx.api.Get("/adb/device-info", adbH.GetDeviceInfo)
	ctx.api.Get("/adb/files", adbH.ListFiles)
	ctx.api.Get("/adb/apps", adbH.ListApps)
	ctx.api.Get("/adb/modules", adbH.ListInstalledModules)
	ctx.api.Get("/adb/modules/:name", adbH.GetModuleInfo)
	ctx.api.Get("/adb/logcat", adbH.GetLogcat)
	ctx.api.Post("/adb/benchmark", benchH.Benchmark)
	ctx.api.Get("/adb/mirror", mirrorH.Mirror)

	// Update check
	ctx.api.Post("/update/check", updateH.CheckUpdate)
	ctx.api.Post("/update/check-all", updateH.CheckAllUpdates)

	// ── Protected ──

	// ADB server
	ctx.r("POST", "/adb/start-server", adbH.StartServer)
	ctx.r("POST", "/adb/kill-server", adbH.KillServer)

	// ADB connection
	ctx.r("POST", "/adb/connect", adbH.ConnectDevice)
	ctx.r("POST", "/adb/pair", adbH.PairDevice)
	ctx.r("GET", "/adb/diagnose", adbH.DiagnoseDevice)
	ctx.r("POST", "/adb/disconnect", adbH.DisconnectDevice)
	ctx.r("POST", "/adb/disconnect-all", adbH.DisconnectAll)

	// ADB saved devices
	ctx.r("GET", "/adb/saved-devices", adbH.GetSavedDevices)
	ctx.r("POST", "/adb/saved-devices", adbH.SaveDevice)
	ctx.r("DELETE", "/adb/saved-devices/:id", adbH.DeleteSavedDevice)

	// ADB shell/exec
	ctx.r("POST", "/adb/shell", adbH.RunShell)
	ctx.r("POST", "/adb/exec", adbH.RunExec)

	// ADB file management
	ctx.r("POST", "/adb/push", adbH.PushFile)
	ctx.r("POST", "/adb/pull", adbH.PullFile)
	ctx.r("POST", "/adb/delete", adbH.DeleteFile)
	ctx.r("POST", "/adb/mkdir", adbH.MakeDir)
	ctx.r("POST", "/adb/rename", adbH.RenameFile)
	ctx.r("POST", "/adb/file/read", adbH.ReadFile)
	ctx.r("POST", "/adb/file/write", adbH.WriteFile)
	ctx.r("POST", "/adb/file/copy", adbH.CopyFile)
	ctx.r("GET", "/adb/file/info", adbH.GetFileInfo)
	ctx.r("POST", "/adb/file/upload", adbH.UploadFile)
	ctx.r("GET", "/adb/file/download", adbH.DownloadFile)

	// ADB app management
	ctx.r("POST", "/adb/app/install", adbH.InstallApp)
	ctx.r("POST", "/adb/app/uninstall", adbH.UninstallApp)
	ctx.r("POST", "/adb/app/clear-data", adbH.ClearAppData)
	ctx.r("POST", "/adb/app/force-stop", adbH.ForceStopApp)
	ctx.r("POST", "/adb/app/launch", adbH.LaunchApp)
	ctx.r("POST", "/adb/app/toggle", adbH.ToggleApp)

	// ADB module management
	ctx.r("POST", "/adb/install", adbH.InstallModule)
	ctx.r("POST", "/adb/modules/:name/toggle", adbH.ToggleModule)
	ctx.r("POST", "/adb/modules/:name/uninstall", adbH.UninstallModule)
	ctx.r("POST", "/adb/module/install-url", adbH.InstallModuleFromURL)
	ctx.r("POST", "/adb/module/upload", adbH.UploadAndInstallModule)
	ctx.r("POST", "/adb/module/backup", adbH.BackupModule)
	ctx.r("POST", "/adb/module/restore", adbH.RestoreModule)
	ctx.r("GET", "/adb/module/check-update", adbH.CheckModuleUpdate)
	ctx.r("POST", "/adb/module/export", adbH.ExportModule)
	ctx.r("POST", "/adb/module/push", adbH.PushModuleFolder)
	ctx.r("POST", "/adb/module/push-build", adbH.PushBuildModule)
	ctx.r("POST", "/adb/module/push-zip", adbH.PushBuildZip)

	// ADB root manager
	ctx.r("GET", "/adb/root/managers", adbH.GetAvailableRootManagers)
	ctx.r("POST", "/adb/root/permission", adbH.ManageRootPermission)
	ctx.r("GET", "/adb/root/permissions", adbH.ListRootPermissions)
	ctx.r("GET", "/adb/root/modules", adbH.GetRootModules)

	// ADB screen
	ctx.r("GET", "/adb/screen/screenshot", adbH.ScreenshotBase64)
	ctx.r("GET", "/adb/screen/size", adbH.GetScreenSize)
	ctx.r("POST", "/adb/screen/tap", adbH.TapScreen)
	ctx.r("POST", "/adb/screen/swipe", adbH.SwipeScreen)
	ctx.r("POST", "/adb/screen/input", adbH.InputText)
	ctx.r("POST", "/adb/screen/key", adbH.KeyEvent)
	ctx.r("POST", "/adb/screen/record", adbH.ScreenRecord)

	// ADB screenshot
	ctx.r("GET", "/adb/screenshot", screenshotH.Screenshot)
	ctx.r("GET", "/adb/screenshot/stream", screenshotH.StreamScreenshots)

	// ADB device ops
	ctx.r("POST", "/adb/reboot", adbH.RebootDevice)
	ctx.r("POST", "/adb/prop/set", adbH.SetProp)
	ctx.r("POST", "/adb/logcat/clear", adbH.ClearLogcat)

	// Benchmark
	ctx.r("POST", "/benchmark/run", benchmarkAPIH.RunBenchmark)
	ctx.r("GET", "/benchmark/history", benchmarkAPIH.GetHistory)
}
