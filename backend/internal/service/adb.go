package service

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type ADBDevice struct {
	Serial  string `json:"serial"`
	Model   string `json:"model"`
	State   string `json:"android,omitempty"`
}

type ADBService struct{}

func NewADBService() *ADBService {
	return &ADBService{}
}

func (s *ADBService) adbPath(ctx context.Context) string {
	candidates := []string{"adb", "/usr/bin/adb", "/usr/local/bin/adb", "/opt/homebrew/bin/adb", "platform-tools/adb"}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, "C:\\platform-tools\\adb.exe", "adb.exe")
	}
	for _, p := range candidates {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}
	return "adb"
}

func (s *ADBService) CheckADBAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, s.adbPath(ctx), "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "Android Debug Bridge")
}

func (s *ADBService) ListDevices(ctx context.Context) ([]ADBDevice, error) {
	cmd := exec.CommandContext(ctx, s.adbPath(ctx), "devices", "-l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("adb devices failed: %v", err)
	}
	var devices []ADBDevice
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of") || strings.HasPrefix(line, "*") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		dev := ADBDevice{
			Serial: parts[0],
			State:  parts[1],
		}
		for _, p := range parts[2:] {
			if strings.HasPrefix(p, "model:") {
				dev.Model = strings.TrimPrefix(p, "model:")
			}
		}
		if dev.State == "device" {
			// Try to get Android version
			cmdV := exec.CommandContext(ctx, s.adbPath(ctx), "-s", dev.Serial, "shell", "getprop", "ro.build.version.release")
			if vOut, err := cmdV.CombinedOutput(); err == nil {
				dev.State = strings.TrimSpace(string(vOut))
			}
		}
		devices = append(devices, dev)
	}
	return devices, nil
}

func (s *ADBService) PushFile(ctx context.Context, serial, local, remote string) (string, error) {
	cmd := exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "push", local, remote)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("adb push failed: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *ADBService) InstallModule(ctx context.Context, serial, zipPath string) (string, error) {
	remotePath := "/data/local/tmp/module.zip"
	if _, err := s.PushFile(ctx, serial, zipPath, remotePath); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "shell", "su", "-c", "magisk --install-module "+remotePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("install failed: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *ADBService) RunShell(ctx context.Context, serial, shellCmd string) (string, error) {
	cmd := exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "shell", shellCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("shell command failed: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *ADBService) RebootDevice(ctx context.Context, serial string) error {
	cmd := exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "reboot")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reboot failed: %v", string(out))
	}
	return nil
}

func (s *ADBService) Screenshot(ctx context.Context, serial, localPath string) (string, error) {
	remotePath := "/data/local/tmp/screenshot.png"

	cmd := exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "shell", "screencap", "-p", remotePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("screencap: %s: %w", string(output), err)
	}

	cmd = exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "pull", remotePath, localPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pull screenshot: %s: %w", string(output), err)
	}

	exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "shell", "rm", remotePath).Run()

	return localPath, nil
}

func (s *ADBService) ScreenRecord(ctx context.Context, serial, localPath, duration string) (string, error) {
	remotePath := "/data/local/tmp/record.mp4"
	if duration == "" {
		duration = "10"
	}

	cmd := exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "shell", "screenrecord", "--time-limit", duration, remotePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("screenrecord: %s: %w", string(output), err)
	}

	cmd = exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "pull", remotePath, localPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pull recording: %s: %w", string(output), err)
	}

	exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "shell", "rm", remotePath).Run()

	return localPath, nil
}

// flusher is satisfied by *bufio.Writer and http.Flusher
type flusher interface {
	Flush() error
}

// StreamScreen continuously captures the device screen and writes MJPEG frames to the writer
func (s *ADBService) StreamScreen(ctx context.Context, serial string, fps int, writer io.Writer) error {
	if fps <= 0 {
		fps = 2
	}
	if fps > 10 {
		fps = 10
	}
	interval := time.Duration(1000/fps) * time.Millisecond

	boundary := "MJPEG_BOUNDARY"

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		cmd := exec.CommandContext(ctx, s.adbPath(ctx), "-s", serial, "exec-out", "screencap", "-p")
		imgData, err := cmd.Output()
		if err != nil {
			time.Sleep(interval)
			continue
		}

		header := fmt.Sprintf("--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(imgData))
		writer.Write([]byte(header))
		writer.Write(imgData)
		writer.Write([]byte("\r\n"))

		if f, ok := writer.(flusher); ok {
			f.Flush()
		}

		time.Sleep(interval)
	}
}

// BenchmarkDevice collects device performance metrics
func (s *ADBService) BenchmarkDevice(ctx context.Context, serial string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	if out, err := s.RunShell(ctx, serial, "cat /proc/cpuinfo | grep 'model name' | head -1"); err == nil {
		result["cpu"] = strings.TrimSpace(out)
	}

	if out, err := s.RunShell(ctx, serial, "cat /proc/meminfo | head -3"); err == nil {
		result["memory"] = strings.TrimSpace(out)
	}

	if out, err := s.RunShell(ctx, serial, "dd if=/dev/zero of=/data/local/tmp/bench bs=1M count=10 2>&1 && rm /data/local/tmp/bench"); err == nil {
		result["storage_write"] = strings.TrimSpace(out)
	}

	if out, err := s.RunShell(ctx, serial, "getprop ro.build.version.release"); err == nil {
		result["android_version"] = strings.TrimSpace(out)
	}

	if out, err := s.RunShell(ctx, serial, "uname -r"); err == nil {
		result["kernel"] = strings.TrimSpace(out)
	}

	if out, err := s.RunShell(ctx, serial, "getprop ro.hardware.chipname || getprop ro.board.platform"); err == nil {
		result["gpu"] = strings.TrimSpace(out)
	}

	if out, err := s.RunShell(ctx, serial, "cat /proc/uptime"); err == nil {
		result["uptime"] = strings.TrimSpace(out)
	}

	return result, nil
}
