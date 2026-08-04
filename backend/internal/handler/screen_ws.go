package handler

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/moduforge/backend/internal/service"
)

// Binary frame types
const (
	frameTypeFrame uint16 = 0x01
	frameTypeInit  uint16 = 0x02
	frameTypeError uint16 = 0x03
	frameTypeStats uint16 = 0x04
)

// Codec IDs (byte 10 in frame header)
const (
	codecPNG  byte = 0x00
	codecH264 byte = 0x01
	codecJPEG byte = 0x02
)

// ──────────────────────── Session management ────────────────────────

// ScreenClient wraps a fasthttp/websocket.Conn for screen streaming.
// We use the underlying fasthttp conn directly for binary message support.
type ScreenClient struct {
	conn *websocket.Conn
}

type ScreenSession struct {
	serial  string
	cancel  context.CancelFunc
	clients map[*ScreenClient]bool
	mu      sync.RWMutex
}

var (
	sessions   = make(map[string]*ScreenSession)
	sessionsMu sync.Mutex
)

func getOrCreateSession(serial string) *ScreenSession {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	if s, ok := sessions[serial]; ok {
		return s
	}
	s := &ScreenSession{serial: serial, clients: make(map[*ScreenClient]bool)}
	sessions[serial] = s
	return s
}

// ──────────────────────── WebSocket handler ────────────────────────

func RegisterScreenStreamWS(app *fiber.App, adbSvc *service.ADBService, jwtSecret string) {
	wsHandler := websocket.New(func(c *websocket.Conn) {
		token := c.Query("token")
		if token == "" {
			c.WriteJSON(fiber.Map{"error": "missing token"})
			c.Close()
			return
		}
		claims, err := service.ParseJWT(token, jwtSecret)
		if err != nil || claims == nil {
			c.WriteJSON(fiber.Map{"error": "invalid token"})
			c.Close()
			return
		}
		_ = claims // token valid, proceed

		serial := c.Query("serial")
		if serial == "" {
			c.WriteJSON(fiber.Map{"error": "serial required"})
			c.Close()
			return
		}
		// Read quality (JPEG 10-100), scale (2/4/8), and optional target width from query params
		quality := 70
		if q := c.Query("quality"); q != "" {
			fmt.Sscanf(q, "%d", &quality)
		}
		scale := 4
		if s := c.Query("scale"); s != "" {
			fmt.Sscanf(s, "%d", &scale)
		}
		targetWidth := 0
		if w := c.Query("width"); w != "" {
			fmt.Sscanf(w, "%d", &targetWidth)
		}

		slog.Info("screen ws connected", "serial", serial)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Get screen size
		w, h, sizeErr := adbSvc.GetScreenSize(ctx, serial)
		if sizeErr != nil {
			slog.Warn("screen ws: failed to get screen size", "error", sizeErr)
			w, h = 1080, 1920
		}

		// Register client
		client := &ScreenClient{conn: c}
		session := getOrCreateSession(serial)
		session.mu.Lock()
		session.clients[client] = true
		isFirstClient := len(session.clients) == 1
		session.mu.Unlock()

		defer func() {
			session.mu.Lock()
			delete(session.clients, client)
			empty := len(session.clients) == 0
			session.mu.Unlock()
			if empty {
				session.cancel()
				sessionsMu.Lock()
				delete(sessions, serial)
				sessionsMu.Unlock()
			}
		}()

		if isFirstClient {
			session.cancel = cancel
			go streamScreen(ctx, adbSvc, serial, w, h, session, quality, scale, targetWidth)
		}

		// Command reader — run each command in a goroutine so slow
		// operations like `input tap` (~700ms) don't block the reader.
		for {
			msgType, msg, err := c.ReadMessage()
			if err != nil {
				if !strings.Contains(err.Error(), "close") {
					slog.Warn("screen ws: read error", "error", err)
				}
				return
			}
			// Handle ping (text message from client heartbeat)
			if msgType == websocket.TextMessage {
				var ping struct {
					Action string `json:"action"`
				}
				if json.Unmarshal(msg, &ping) == nil && ping.Action == "ping" {
					// Respond with pong to confirm liveness
					c.WriteMessage(websocket.TextMessage, []byte(`{"action":"pong"}`))
					continue
				}
			}
			// Handle binary commands (shouldn't happen, but guard)
			if msgType != websocket.TextMessage {
				continue
			}
			var cmd struct {
				Action string `json:"action"`
				X      int    `json:"x"`
				Y      int    `json:"y"`
				X1     int    `json:"x1"`
				Y1     int    `json:"y1"`
				X2     int    `json:"x2"`
				Y2     int    `json:"y2"`
				Key    string `json:"key"`
				Text   string `json:"text"`
				Dur    int    `json:"duration"`
			}
			if err := json.Unmarshal(msg, &cmd); err != nil {
				slog.Warn("screen ws: bad cmd json", "raw", string(msg), "error", err)
				continue
			}
			go handleInputCommand(ctx, adbSvc, serial, cmd)
		}
	}, websocket.Config{
		HandshakeTimeout: 10 * time.Second,
		AllowEmptyOrigin: true,
		EnableCompression: true,
	})

	app.Get("/api/v1/ws/screen", wsHandler)
}

func handleInputCommand(ctx context.Context, adbSvc *service.ADBService, serial string, cmd struct {
	Action string `json:"action"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	X1     int    `json:"x1"`
	Y1     int    `json:"y1"`
	X2     int    `json:"x2"`
	Y2     int    `json:"y2"`
	Key    string `json:"key"`
	Text   string `json:"text"`
	Dur    int    `json:"duration"`
}) {
	slog.Info("screen ws: input command", "action", cmd.Action, "x", cmd.X, "y", cmd.Y)
	switch cmd.Action {
	case "tap":
		adbSvc.SendTap(ctx, serial, cmd.X, cmd.Y)
	case "double_tap":
		adbSvc.SendTap(ctx, serial, cmd.X, cmd.Y)
		time.Sleep(80 * time.Millisecond)
		adbSvc.SendTap(ctx, serial, cmd.X, cmd.Y)
	case "long_press":
		dur := cmd.Dur
		if dur <= 0 {
			dur = 800
		}
		adbSvc.SendLongPress(ctx, serial, cmd.X, cmd.Y, dur)
	case "swipe":
		dur := cmd.Dur
		if dur <= 0 {
			dur = 300
		}
		adbSvc.SendSwipe(ctx, serial, cmd.X1, cmd.Y1, cmd.X2, cmd.Y2, dur)
	case "pinch":
		adbSvc.SendPinch(ctx, serial, cmd.X1, cmd.Y1, cmd.X2, cmd.Y2, cmd.Dur)
	case "key":
		adbSvc.KeyEvent(ctx, serial, cmd.Key)
	case "input":
		adbSvc.InputText(ctx, serial, cmd.Text)
	case "home":
		adbSvc.KeyEvent(ctx, serial, "KEYCODE_HOME")
	case "back":
		adbSvc.KeyEvent(ctx, serial, "KEYCODE_BACK")
	case "recent":
		adbSvc.KeyEvent(ctx, serial, "KEYCODE_APP_SWITCH")
	}
}

// ──────────────────────── Stream dispatcher ────────────────────────

func streamScreen(ctx context.Context, adbSvc *service.ADBService, serial string, devW, devH int, session *ScreenSession, quality, scale, targetWidth int) {
	// Try JPEG first (universal, ~10fps on Android 13+), then H.264, then PNG
	if err := streamJPEG(ctx, adbSvc, serial, devW, devH, session, quality, scale, targetWidth); err != nil {
		slog.Warn("screen ws: JPEG failed, trying H.264", "error", err)
		if err := streamH264(ctx, adbSvc, serial, devW, devH, session); err != nil {
			slog.Warn("screen ws: H.264 failed, falling back to PNG", "error", err)
			streamPNG(ctx, adbSvc, serial, devW, devH, session)
		}
	}
}

// ──────────────── H.264 streaming via screenrecord ────────────────

func streamH264(ctx context.Context, adbSvc *service.ADBService, serial string, devW, devH int, session *ScreenSession) error {
	slog.Info("screen ws: starting H.264 stream", "serial", serial)

	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", "screenrecord", "--output-format=h264", "--time-limit", "180", "/dev/stdout")

	cmd := createADBCommand(ctx, adbSvc, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start screenrecord: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Send init
	sendInit(session, devW, devH)

	// Parse H.264 stream into frames
	parser := newNALParser()
	reader := bufio.NewReaderSize(stdout, 64*1024)
	tmp := make([]byte, 32*1024)
	stats := &frameStats{}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, readErr := reader.Read(tmp)
		if n > 0 {
			frames := parser.feed(tmp[:n])
			for _, f := range frames {
				stats.record(f)
				// Frame header: type(2) + width(4) + height(4) + codec(1) + flags(1) = 12 bytes
				msg := make([]byte, 12+len(f.data))
				binary.BigEndian.PutUint16(msg[0:2], frameTypeFrame)
				binary.BigEndian.PutUint32(msg[2:6], uint32(devW))
				binary.BigEndian.PutUint32(msg[6:10], uint32(devH))
				msg[10] = codecH264
				if f.isKey {
					msg[11] = 0x01
				}
				copy(msg[12:], f.data)
				broadcastBinary(session, msg)
			}
			stats.maybeReport(session)
		}
		if readErr != nil {
			if readErr != io.EOF {
				slog.Warn("screenrecord read error", "error", readErr)
			}
			return readErr
		}
	}
}

// ──────── JPEG streaming via raw screencap (universal, fast) ────────

func streamJPEG(ctx context.Context, adbSvc *service.ADBService, serial string, devW, devH int, session *ScreenSession, quality, scale, targetWidth int) error {
	// If targetWidth is specified, calculate scale from device width
	if targetWidth > 0 && targetWidth < devW {
		scale = devW / targetWidth
		if scale < 1 {
			scale = 1
		}
	}
	slog.Info("screen ws: starting JPEG stream", "serial", serial, "device", fmt.Sprintf("%dx%d", devW, devH), "quality", quality, "scale", scale, "targetWidth", targetWidth)

	sendInit(session, devW, devH)

	interval := 150 * time.Millisecond // start conservative

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		start := time.Now()

		_, _, jpegData, err := adbSvc.CaptureScreenJPEG(ctx, serial, quality, scale)
		if err != nil {
			slog.Warn("screen ws: CaptureScreenJPEG error", "error", err)
			time.Sleep(time.Second)
			return err // bail out to try H.264
		}

		// Always send the ORIGINAL device dimensions so the frontend can
		// map touch coordinates correctly, even though the image is quarter-res.
		msg := make([]byte, 12+len(jpegData))
		binary.BigEndian.PutUint16(msg[0:2], frameTypeFrame)
		binary.BigEndian.PutUint32(msg[2:6], uint32(devW))
		binary.BigEndian.PutUint32(msg[6:10], uint32(devH))
		msg[10] = codecJPEG
		msg[11] = 0
		copy(msg[12:], jpegData)
		broadcastBinary(session, msg)

		elapsed := time.Since(start)
		// Adaptive interval: target ~80% of capture time
		if elapsed < 50*time.Millisecond {
			interval = 50 * time.Millisecond // cap at ~20fps
		} else if elapsed > 500*time.Millisecond {
			interval = time.Duration(float64(elapsed) * 1.2)
		} else {
			interval = time.Duration(float64(elapsed) * 1.3)
		}

		time.Sleep(interval)
	}
}

// ──────────────── PNG fallback via screencap ────────────────────────

func streamPNG(ctx context.Context, adbSvc *service.ADBService, serial string, devW, devH int, session *ScreenSession) {
	slog.Info("screen ws: starting PNG stream (fallback)", "serial", serial)

	sendInit(session, devW, devH)

	ticker := time.NewTicker(200 * time.Millisecond) // 5fps cap for PNG
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			args := []string{}
			if serial != "" {
				args = append(args, "-s", serial)
			}
			args = append(args, "exec-out", "screencap", "-p")

			stdout, _, err := adbSvc.ExecADBRaw(ctx, args...)
			if err != nil || len(stdout) < 100 {
				continue
			}

			msg := make([]byte, 12+len(stdout))
			binary.BigEndian.PutUint16(msg[0:2], frameTypeFrame)
			binary.BigEndian.PutUint32(msg[2:6], uint32(devW))
			binary.BigEndian.PutUint32(msg[6:10], uint32(devH))
			msg[10] = codecPNG
			msg[11] = 0
			copy(msg[12:], stdout)
			broadcastBinary(session, msg)
		}
	}
}

// ──────────────── Helpers ────────────────────────

func createADBCommand(ctx context.Context, adbSvc *service.ADBService, args ...string) *exec.Cmd {
	return adbSvc.CreateCommand(ctx, args...)
}

func sendInit(session *ScreenSession, w, h int) {
	initBuf := make([]byte, 10)
	binary.BigEndian.PutUint16(initBuf[0:2], frameTypeInit)
	binary.BigEndian.PutUint32(initBuf[2:6], uint32(w))
	binary.BigEndian.PutUint32(initBuf[6:10], uint32(h))
	broadcastBinary(session, initBuf)
}

func broadcastBinary(session *ScreenSession, data []byte) {
	// Gzip compress the data if it's large enough to benefit
	sendData := data
	if len(data) > 256 { // Only compress if > 256 bytes (small frames don't benefit)
		if compressed, err := gzipData(data); err == nil && len(compressed) < len(data) {
			sendData = compressed
		}
	}

	session.mu.RLock()
	defer session.mu.RUnlock()
	for sc := range session.clients {
		sc.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := sc.conn.WriteMessage(websocket.BinaryMessage, sendData); err != nil {
			slog.Warn("screen ws: broadcast failed", "error", err)
		}
	}
}

// gzipData compresses data using gzip
func gzipData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ──────────────── H.264 NAL unit parser ────────────────────────

type nalFrame struct {
	data  []byte
	isKey bool
}

type nalParser struct {
	buf []byte
	sps []byte
	pps []byte
}

func newNALParser() *nalParser {
	return &nalParser{buf: make([]byte, 0, 512*1024)}
}

func (p *nalParser) feed(data []byte) []nalFrame {
	p.buf = append(p.buf, data...)
	var frames []nalFrame

	for {
		idx := findStartCode(p.buf)
		if idx < 0 {
			// Keep last 4 bytes (might be partial start code)
			if len(p.buf) > 4 {
				p.buf = p.buf[len(p.buf)-4:]
			}
			break
		}

		nextIdx := findStartCode(p.buf[idx+3:])
		if nextIdx < 0 {
			// Incomplete NAL, keep from start code
			if idx > 0 {
				p.buf = p.buf[idx:]
			}
			break
		}

		nal := make([]byte, nextIdx)
		copy(nal, p.buf[idx:idx+3+nextIdx])
		p.buf = p.buf[idx+3+nextIdx:]

		nalType := nal[3] & 0x1F

		switch nalType {
		case 7: // SPS
			p.sps = make([]byte, len(nal))
			copy(p.sps, nal)
		case 8: // PPS
			p.pps = make([]byte, len(nal))
			copy(p.pps, nal)
		case 5: // IDR (keyframe)
			var frameData []byte
			if len(p.sps) > 0 {
				frameData = append(frameData, p.sps...)
			}
			if len(p.pps) > 0 {
				frameData = append(frameData, p.pps...)
			}
			frameData = append(frameData, nal...)
			frames = append(frames, nalFrame{data: frameData, isKey: true})
		case 1: // Non-IDR P/B frame
			frames = append(frames, nalFrame{data: nal, isKey: false})
		}
	}
	return frames
}

func findStartCode(data []byte) int {
	for i := 0; i < len(data)-3; i++ {
		if data[i] == 0 && data[i+1] == 0 {
			if data[i+2] == 0 && data[i+3] == 1 {
				return i
			}
			if data[i+2] == 1 {
				return i
			}
		}
	}
	return -1
}

// ──────────────── Frame stats ────────────────────────

type frameStats struct {
	count     int
	bytes     int
	lastReset time.Time
}

func (s *frameStats) record(f nalFrame) {
	s.count++
	s.bytes += len(f.data)
}

func (s *frameStats) maybeReport(session *ScreenSession) {
	if s.lastReset.IsZero() {
		s.lastReset = time.Now()
		return
	}
	if time.Since(s.lastReset) < time.Second {
		return
	}
	statsBuf := make([]byte, 14)
	binary.BigEndian.PutUint16(statsBuf[0:2], frameTypeStats)
	binary.BigEndian.PutUint32(statsBuf[2:6], uint32(s.count))
	binary.BigEndian.PutUint32(statsBuf[6:10], uint32(s.bytes))
	binary.BigEndian.PutUint32(statsBuf[10:14], uint32(time.Since(s.lastReset).Milliseconds()))
	broadcastBinary(session, statsBuf)
	s.count = 0
	s.bytes = 0
	s.lastReset = time.Now()
}
