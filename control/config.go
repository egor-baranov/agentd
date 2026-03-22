package control

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	ServiceName    string   `json:"service_name"`
	HTTPAddr       string   `json:"http_addr"`
	NodeGRPCListen string   `json:"node_grpc_listen"`
	TemporalAddr   string   `json:"temporal_addr"`
	TemporalNS     string   `json:"temporal_namespace"`
	TaskQueue      string   `json:"task_queue"`
	RegistryURLs   []string `json:"registry_urls"`
	ArtifactDir    string   `json:"artifact_dir"`
	HostedMode     bool     `json:"hosted_mode"`
	NodeEndpoints  []string `json:"node_endpoints"`
}

type ConfigLoader struct {
	path string
	ptr  atomic.Pointer[Config]
	mu   sync.Mutex
}

func NewConfigLoader(path string) *ConfigLoader {
	return &ConfigLoader{path: path}
}

func (l *ConfigLoader) Load() (*Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	raw, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.NodeGRPCListen == "" {
		cfg.NodeGRPCListen = ":9091"
	}
	if cfg.TemporalNS == "" {
		cfg.TemporalNS = "default"
	}
	if cfg.TaskQueue == "" {
		cfg.TaskQueue = "agentd"
	}
	l.ptr.Store(&cfg)
	return &cfg, nil
}

func (l *ConfigLoader) Snapshot() Config {
	if cfg := l.ptr.Load(); cfg != nil {
		return *cfg
	}
	return Config{}
}

func (l *ConfigLoader) ReloadLoop(ctx context.Context, every time.Duration) error {
	if every <= 0 {
		return fmt.Errorf("reload interval must be > 0")
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_, _ = l.Load()
		}
	}
}
