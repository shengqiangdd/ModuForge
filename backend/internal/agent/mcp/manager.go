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
//  1. MCP_SERVERS env var — inline JSON array
//  2. MCP_SERVERS_FILE env var — path to a JSON file
//
// Both formats are the same JSON array of ServerConfig objects:
//
//	[
//	  {"name": "github", "url": "http://localhost:8000/mcp",
//	   "headers": {"Authorization": "Bearer xxx"}}
//	]
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client // keyed by Name
}

// NewManager creates an empty manager.
func NewManager() *Manager {
	return &Manager{clients: map[string]*Client{}}
}

// LoadFromEnv reads MCP_SERVERS / MCP_SERVERS_FILE and connects to all
// configured servers. Non-fatal: a failing server is logged and skipped so
// the platform still starts.
func (m *Manager) LoadFromEnv(ctx context.Context) error {
	cfg := loadServerConfigs()
	if len(cfg) == 0 {
		slog.Info("MCP: no servers configured (set MCP_SERVERS or MCP_SERVERS_FILE)")
		return nil
	}

	for _, sc := range cfg {
		client := NewClient(sc)
		m.mu.Lock()
		m.clients[sc.Name] = client
		m.mu.Unlock()

		initCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := client.Initialize(initCtx)
		cancel()
		if err != nil {
			slog.Warn("MCP: server failed to initialize, will not expose tools", "name", sc.Name, "url", sc.URL, "error", err)
			m.mu.Lock()
			delete(m.clients, sc.Name)
			m.mu.Unlock()
		}
	}
	return nil
}

// Clients returns the currently ready clients.
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
