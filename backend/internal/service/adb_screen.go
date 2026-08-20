package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)


// ─── Screenshot & Screen Record & Touch ───
// ─── Screenshot & Screen Record ───

func (s *ADBService) Screenshot(ctx context.Context, serial, localPath string) (string, error) {
	remotePath := "/data/local/tmp/screenshot.png"
	if _, err := s.RunShell(ctx, serial, "screencap -p "+remotePath); err != nil {
		return "", fmt.Errorf("screencap failed: %w", err)
	}
	if _, err := s.run(ctx, "-s", serial, "pull", remotePath, localPath); err != nil {
		return "", fmt.Errorf("pull screenshot failed: %w", err)
	}
	s.RunShell(ctx, serial, "rm "+remotePath)
	return localPath, nil
}

// ScreenshotBase64 captures the screen and returns the PNG data as base64.
func (s *ADBService) ScreenshotBase64(ctx context.Context, serial string) (string, error) {
	cmd := exec.CommandContext(ctx, s.ADBPath(), "-s", serial, "exec-out", "screencap", "-p")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("screencap failed: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(stdout.Bytes())
	return encoded, nil
}

// GetScreenSize returns the device screen width and height.
func (s *ADBService) GetScreenSize(ctx context.Context, serial string) (int, int, error) {
	out, err := s.RunShell(ctx, serial, "wm size")
	if err != nil {
		return 0, 0, fmt.Errorf("get screen size failed: %w", err)
	}
	// Output format: "Physical size: 1080x2340" or "Override size: 1080x2340"
	// Find the last line with "x" in it
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "x") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				sizeStr := strings.TrimSpace(parts[len(parts)-1])
				dim := strings.Split(sizeStr, "x")
				if len(dim) == 2 {
					w, _ := strconv.Atoi(strings.TrimSpace(dim[0]))
					h, _ := strconv.Atoi(strings.TrimSpace(dim[1]))
					if w > 0 && h > 0 {
						return w, h, nil
					}
				}
			}
		}
	}
	return 0, 0, fmt.Errorf("cannot parse screen size from: %s", out)
}

// CaptureScreenJPEG captures the screen as raw PPM via `screencap`, resizes
// by the given scale factor, and returns JPEG bytes. This avoids the slow PNG
// compression on the device side, giving 5-10× higher throughput.
func (s *ADBService) CaptureScreenJPEG(ctx context.Context, serial string, quality, scale int) (int, int, []byte, error) {
	if quality <= 0 {
		quality = 70
	}
	if scale <= 0 {
		scale = 4
	}

	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "exec-out", "screencap")

	out, _, err := s.ExecADBRaw(ctx, args...)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("screencap failed: %w", err)
	}
	if len(out) < 20 {
		return 0, 0, nil, fmt.Errorf("screencap output too short (%d bytes)", len(out))
	}

	// Parse PPM P6 header
	width, height, headerLen, err := parsePPMHeader(out)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("PPM parse error: %w", err)
	}

	pixelData := out[headerLen:]
	expected := width * height * 3
	if len(pixelData) < expected {
		return 0, 0, nil, fmt.Errorf("PPM pixel data short: got %d, need %d", len(pixelData), expected)
	}

	// Resize by scale factor
	newW, newH := width/scale, height/scale
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	resized := make([]byte, newW*newH*3)
	scaleX := float64(width) / float64(newW)
	scaleY := float64(height) / float64(newH)

	for ny := 0; ny < newH; ny++ {
		sy := int(float64(ny) * scaleY)
		for nx := 0; nx < newW; nx++ {
			sx := int(float64(nx) * scaleX)
			srcIdx := (sy*width + sx) * 3
			dstIdx := (ny*newW + nx) * 3
			resized[dstIdx] = pixelData[srcIdx]
			resized[dstIdx+1] = pixelData[srcIdx+1]
			resized[dstIdx+2] = pixelData[srcIdx+2]
		}
	}

	// Build image.RGBA for jpeg encoder
	img := image.NewRGBA(image.Rect(0, 0, newW, newH))
	pix := img.Pix
	for i := 0; i < newW*newH; i++ {
		pix[i*4] = resized[i*3]
		pix[i*4+1] = resized[i*3+1]
		pix[i*4+2] = resized[i*3+2]
		pix[i*4+3] = 0xFF
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return 0, 0, nil, fmt.Errorf("JPEG encode failed: %w", err)
	}

	return newW, newH, buf.Bytes(), nil
}

// parsePPMHeader parses a P6 PPM header and returns width, height, and the
// byte offset where pixel data begins.
func parsePPMHeader(data []byte) (int, int, int, error) {
	s := bufio.NewScanner(bytes.NewReader(data))
	// Magic number
	if !s.Scan() {
		return 0, 0, 0, fmt.Errorf("empty PPM")
	}
	magic := strings.TrimSpace(s.Text())
	if magic != "P6" {
		return 0, 0, 0, fmt.Errorf("not P6: %s", magic)
	}

	// Dimensions (skip comment lines starting with #)
	width, height := 0, 0
	found := false
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			width, _ = strconv.Atoi(parts[0])
			height, _ = strconv.Atoi(parts[1])
			found = true
			break
		}
	}
	if !found || width <= 0 || height <= 0 {
		return 0, 0, 0, fmt.Errorf("cannot parse dimensions")
	}

	// Max value line
	if s.Scan() {
		// typically "255" — we just skip it
	}

	// Calculate header end offset: everything after the scanner consumed
	// the header text. We re-find the position in the raw bytes.
	headerEnd := 0
	lines := 0
	for i, b := range data {
		if b == '\n' {
			lines++
			if lines >= 3 {
				// After the 3rd newline (magic, dims, maxval) the pixel data starts
				headerEnd = i + 1
				break
			}
		}
	}

	return width, height, headerEnd, nil
}

// getTouchDevice finds the input device path for touch events.
func (s *ADBService) getTouchDevice(ctx context.Context, serial string) (string, error) {
	out, err := s.RunShell(ctx, serial, "getevent -pl 2>/dev/null | grep -B5 'ABS_MT_POSITION' | grep 'add device' | head -1 | awk '{print $NF}'")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("no touch device found")
	}
	return strings.TrimSpace(out), nil
}

// TapScreen sends a tap event at (x, y). Uses sendevent for speed, falls back to input tap.
func (s *ADBService) TapScreen(ctx context.Context, serial string, x, y int) error {
	devicePath, err := s.getTouchDevice(ctx, serial)
	if err != nil {
		// Fallback to input tap (slower, ~700ms)
		_, err = s.RunShell(ctx, serial, fmt.Sprintf("input tap %d %d", x, y))
		if err != nil {
			return fmt.Errorf("adb fallback tap: %w", err)
		}
		return nil
	}
	// Use sendevent for fast tap (~50ms)
	cmds := []string{
		fmt.Sprintf("sendevent %s 3 57 0", devicePath),
		fmt.Sprintf("sendevent %s 3 53 %d", devicePath, x),
		fmt.Sprintf("sendevent %s 3 54 %d", devicePath, y),
		fmt.Sprintf("sendevent %s 3 48 5", devicePath),
		fmt.Sprintf("sendevent %s 3 58 50", devicePath),
		fmt.Sprintf("sendevent %s 0 2 0", devicePath),
		fmt.Sprintf("sendevent %s 0 0 0", devicePath),
		fmt.Sprintf("sendevent %s 3 57 -1", devicePath),
		fmt.Sprintf("sendevent %s 0 2 0", devicePath),
		fmt.Sprintf("sendevent %s 0 0 0", devicePath),
	}
	fullCmd := strings.Join(cmds, " && ")
	_, err = s.RunShell(ctx, serial, fullCmd)
	if err != nil {
		return fmt.Errorf("adb long press: %w", err)
	}
	return nil
}

// SwipeScreen sends a swipe/drag event.
func (s *ADBService) SwipeScreen(ctx context.Context, serial string, x1, y1, x2, y2, duration int) error {
	if duration <= 0 {
		duration = 300
	}
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, duration))
	if err != nil {
		return fmt.Errorf("adb swipe: %w", err)
	}
	return nil
}

// SendTap uses input tap for reliable touch response
func (s *ADBService) SendTap(ctx context.Context, serial string, x, y int) error {
	// Try `input tap` first (works on most devices)
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input tap %d %d", x, y))
	if err != nil {
		log.Printf("[ADB] input tap failed (%v), retrying with input touchscreen tap", err)
		_, err = s.RunShell(ctx, serial, fmt.Sprintf("input touchscreen tap %d %d", x, y))
	}
	if err != nil {
		return fmt.Errorf("adb tap: %w", err)
	}
	return nil
}

// SendLongPress uses input swipe (same point) for long press
func (s *ADBService) SendLongPress(ctx context.Context, serial string, x, y, durationMs int) error {
	if durationMs <= 0 {
		durationMs = 800
	}
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", x, y, x, y, durationMs))
	if err != nil {
		return fmt.Errorf("adb long press: %w", err)
	}
	return nil
}

// SendSwipe uses input swipe for smooth gesture
func (s *ADBService) SendSwipe(ctx context.Context, serial string, x1, y1, x2, y2, durationMs int) error {
	if durationMs <= 0 {
		durationMs = 300
	}
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", x1, y1, x2, y2, durationMs))
	if err != nil {
		return fmt.Errorf("adb pinch: %w", err)
	}
	return nil
}

// SendPinch approximates pinch with two sequential swipes
func (s *ADBService) SendPinch(ctx context.Context, serial string, x1, y1, x2, y2, durationMs int) error {
	if durationMs <= 0 {
		durationMs = 300
	}
	// Approximate pinch as two swipes from center outward
	midX := (x1 + x2) / 2
	midY := (y1 + y2) / 2
	// First swipe (finger 1)
	_, err := s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", midX, midY, x1, y1, durationMs))
	if err != nil {
		return fmt.Errorf("adb pinch swipe 1: %w", err)
	}
	// Second swipe (finger 2)
	_, err = s.RunShell(ctx, serial, fmt.Sprintf("input swipe %d %d %d %d %d", midX, midY, x2, y2, durationMs))
	if err != nil {
		return fmt.Errorf("adb pinch swipe 2: %w", err)
	}
	return nil
}

// InputText sends text input.
func (s *ADBService) InputText(ctx context.Context, serial, text string) error {
	// Android's input text command: spaces must be encoded as %s
	// and single quotes / special chars need shell escaping
	var buf strings.Builder
	for _, ch := range text {
		switch ch {
		case ' ':
			buf.WriteString("%s")
		case '\'':
			buf.WriteString("'\\''")
		case '&', '<', '>', '|', ';', '(', ')', '$', '`', '"', '\\':
			buf.WriteByte('\\')
			buf.WriteRune(ch)
		default:
			buf.WriteRune(ch)
		}
	}
	escaped := buf.String()
	_, err := s.RunShell(ctx, serial, "input text '"+escaped+"'")
	if err != nil {
		return fmt.Errorf("adb input text: %w", err)
	}
	return nil
}

// KeyEvent sends an Android key event.
func (s *ADBService) KeyEvent(ctx context.Context, serial, key string) error {
	code, ok := keyMap[key]
	if !ok {
		return fmt.Errorf("unknown key: %s", key)
	}
	_, err := s.RunShell(ctx, serial, "input keyevent "+code)
	if err != nil {
		return fmt.Errorf("adb key event %s: %w", key, err)
	}
	return nil
}

func (s *ADBService) ScreenRecord(ctx context.Context, serial, localPath, duration string) (string, error) {
	remotePath := "/data/local/tmp/record.mp4"
	if duration == "" {
		duration = "10"
	}
	if _, err := s.RunShell(ctx, serial, "screenrecord --time-limit "+duration+" "+remotePath); err != nil {
		return "", fmt.Errorf("screenrecord failed: %w", err)
	}
	if _, err := s.run(ctx, "-s", serial, "pull", remotePath, localPath); err != nil {
		return "", fmt.Errorf("pull recording failed: %w", err)
	}
	s.RunShell(ctx, serial, "rm "+remotePath)
	return localPath, nil
}

type flusher interface {
	Flush() error
}

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
		// Use raw PPM + server-side JPEG instead of screencap -p: the device
		// compresses PNG in software (~100-200ms/frame), while raw PPM is
		// ~5-10× faster to produce. We resize + JPEG-encode on the server.
		_, _, jpegData, err := s.CaptureScreenJPEG(ctx, serial, 60, 3)
		if err != nil {
			time.Sleep(interval)
			continue
		}
		header := fmt.Sprintf("--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(jpegData))
		writer.Write([]byte(header))
		writer.Write(jpegData)
		writer.Write([]byte("\r\n"))
		if f, ok := writer.(flusher); ok {
			f.Flush()
		}
		time.Sleep(interval)
	}
}

// ─── Log Viewer ───

func (s *ADBService) GetLogcat(ctx context.Context, serial, filter, level string, lines int) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "logcat", "-d", "-v", "threadtime")
	if lines > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", lines))
	}
	if level != "" {
		args = append(args, "*:"+strings.ToUpper(level))
	}
	if filter != "" {
		args = append(args, filter)
	}
	out, err := s.run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("logcat failed: %w", err)
	}
	return out, nil
}

func (s *ADBService) ClearLogcat(ctx context.Context, serial string) (string, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "logcat", "-c")
	_, err := s.run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("clear logcat failed: %w", err)
	}
	return "Log cleared", nil
}

// ─── Device Operations ───

func (s *ADBService) RebootDevice(ctx context.Context, serial, mode string) error {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "reboot")
	if mode != "" {
		args = append(args, mode)
	}
	_, err := s.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("adb command: %w", err)
	}
	return nil
}

func (s *ADBService) GetProp(ctx context.Context, serial, prop string) (string, error) {
	return s.RunShell(ctx, serial, "getprop "+prop)
}

func (s *ADBService) SetProp(ctx context.Context, serial, prop, value string) (string, error) {
	return s.RunShell(ctx, serial, fmt.Sprintf("setprop %s %q", prop, value))
}

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
	if out, err := s.RunShell(ctx, serial, "getprop ro.hardware.chipname || getprop ro.board.platform"); err == nil {
		result["chipset"] = strings.TrimSpace(out)
	}
	if out, err := s.RunShell(ctx, serial, "cat /proc/uptime"); err == nil {
		result["uptime"] = strings.TrimSpace(out)
	}
	return result, nil
}

// ScreenshotJPEG captures screen and returns raw PNG bytes (screencap -p output).
// The caller (screen_ws.go) sends these bytes directly as binary WebSocket frames.
// Android screencap outputs PNG natively — we skip JPEG conversion to avoid CPU overhead.
func (s *ADBService) ScreenshotJPEG(ctx context.Context, serial string, quality int) ([]byte, error) {
	return s.ScreenshotRaw(ctx, serial)
}

// ScreenshotRaw captures screen and returns raw image bytes (PNG from screencap -p).
func (s *ADBService) ScreenshotRaw(ctx context.Context, serial string) ([]byte, error) {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "exec-out", "screencap", "-p")

	// Execute ADB and capture raw bytes (not string)
	cmd := exec.CommandContext(ctx, s.ADBPath(), args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// ─── Enhanced Module Push (Folder + Build) ───

// PushModuleFolder zips a local module directory (excluding source code) and installs
// it on the device via the detected root manager (APatch/KernelSU/Magisk).
// The root manager handles: unzip → execute customize.sh → place in /data/adb/modules/.
