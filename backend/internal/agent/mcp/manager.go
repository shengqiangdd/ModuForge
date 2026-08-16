package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// Manager owns a set of MCP server clients and exposes their tools as
// dynamic agent skills. Configuration sources (in priority order):
//
//  1. ServerConfigs loaded via AddServer (UI/API managed, persisted in DB)
//  2. MCP_SERVERS env var — inline JSON array (static, env-driven)
//  3. MCP_SERVERS_FILE env var — path to a JSON file
//
// Servers that fail to initialize are kept in the map (ready=false with a
// lastError) so the UI can surface diagnostics and offer a reconnect —
// mirroring how Claude Code / OpenCode handle broken MCP configs instead
// of silently dropping them.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client // keyed by Name

	// OnServerReady is invoked after a client completes the handshake.
	// Used to register the server's tools as agent skills.
	onServerReady func(*Client)
}

// NewManager creates an empty manager.
func NewManager() *Manager {
	return &Manager{clients: map[string]*Client{}}
}

// SetOnServerReady installs the callback fired after each successful handshake.
func (m *Manager) SetOnServerReady(fn func(*Client)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onServerReady = fn
}

// LoadFromEnv reads MCP_SERVERS / MCP_SERVERS_FILE and connects to all
// configured servers. Non-fatal: a failing server is kept with its error so
// the platform still starts and the failure is visible.
func (m *Manager) LoadFromEnv(ctx context.Context) error {
	cfg := loadServerConfigs()
	if len(cfg) == 0 {
		slog.Info("MCP: no servers configured (set MCP_SERVERS or MCP_SERVERS_FILE)")
		return nil
	}
	for _, sc := range cfg {
		m.AddServer(ctx, sc)
	}
	return nil
}

// AddServer creates a client for cfg and attempts the handshake. The client
// is always stored (ready or not); on success onServerReady fires with the
// client so callers can register its tools. Returns the client (never nil)
// and the handshake error if any.
func (m *Manager) AddServer(ctx context.Context, cfg ServerConfig) (*Client, error) {
	client := NewClient(cfg)

	m.mu.Lock()
	// If a previous client with this name existed, drop its registration;
	// the new one replaces it.
	if old, ok := m.clients[cfg.Name]; ok && old != nil {
		old.tools = nil
	}
	m.clients[cfg.Name] = client
	onReady := m.onServerReady
	m.mu.Unlock()

	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	err := client.Initialize(initCtx)
	cancel()
	if err != nil {
		client.setError(err)
		slog.Warn("MCP: server failed to initialize (kept for diagnostics)", "name", cfg.Name, "url", cfg.URL, "error", err)
		return client, err
	}
	if onReady != nil {
		onReady(client)
	}
	return client, nil
}

// RemoveServer drops a client by name. Returns false if it did not exist.
func (m *Manager) RemoveServer(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.clients[name]; !ok {
		return false
	}
	delete(m.clients, name)
	return true
}

// Reconnect re-runs the handshake for an existing (possibly failed) server.
// Returns the client and error. Tools are re-registered via onServerReady on
// success.
func (m *Manager) Reconnect(ctx context.Context, name string) (*Client, error) {
	m.mu.RLock()
	client, ok := m.clients[name]
	onReady := m.onServerReady
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("mcp server not found: %s", name)
	}

	initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	err := client.Initialize(initCtx)
	cancel()
	if err != nil {
		client.setError(err)
		slog.Warn("MCP: reconnect failed", "name", name, "error", err)
		return client, err
	}
	if onReady != nil {
		onReady(client)
	}
	return client, nil
}

// HasServer reports whether a named server exists (ready or not).
func (m *Manager) HasServer(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[name]
	return ok
}

// Clients returns the currently known clients (ready and failed).
func (m *Manager) Clients() []*Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		out = append(out, c)
	}
	return out
}

// Get returns a client by name.
func (m *Manager) Get(name string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clients[name]
	return c, ok
}

// Close terminates all client sessions (best-effort).
func (m *Manager) Close() {
	// No persistent connections to close for HTTP transport; kept for API parity.
}

// loadServerConfigs parses MCP server config from env/file.
func loadServerConfigs() []ServerConfig {
	raw := os.Getenv("MCP_SERVERS")
	if strings.TrimSpace(raw) == "" {
		if path := os.Getenv("MCP_SERVERS_FILE"); path != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				slog.Warn("MCP: cannot read MCP_SERVERS_FILE", "path", path, "error", err)
				return nil
			}
			raw = string(data)
		}
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var cfg []ServerConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		slog.Error("MCP: invalid MCP_SERVERS JSON", "error", err)
		return nil
	}

	valid := cfg[:0]
	for _, sc := range cfg {
		if sc.Name == "" || sc.URL == "" {
			slog.Warn("MCP: skipping server with empty name/url", "config", fmt.Sprintf("%+v", sc))
			continue
		}
		valid = append(valid, sc)
	}
	return valid
}
