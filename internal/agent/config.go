package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	Version            = "0.1.0"
	DefaultServiceName = "InfoBHMonitorAgent"
)

type Config struct {
	ServerURL      string        `json:"server_url"`
	Token          string        `json:"token"`
	HeartbeatEvery time.Duration `json:"heartbeat_every"`
	MetricsEvery   time.Duration `json:"metrics_every"`
	DevicesEvery   time.Duration `json:"devices_every"`
	RequestTimeout time.Duration `json:"request_timeout"`
}

type diskPayload struct {
	Name        string  `json:"name"`
	MountPoint  string  `json:"mount_point"`
	FileSystem  string  `json:"file_system"`
	DriveType   string  `json:"drive_type"`
	TotalBytes  int64   `json:"total_bytes"`
	UsedBytes   int64   `json:"used_bytes"`
	FreeBytes   int64   `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type heartbeatPayload struct {
	Hostname       string `json:"hostname"`
	OSName         string `json:"os_name"`
	AgentVersion   string `json:"agent_version"`
	IPAddress      string `json:"ip_address"`
	InternetOnline bool   `json:"internet_online"`
	CollectedAt    string `json:"collected_at"`
}

type metricPayload struct {
	CollectedAt    string        `json:"collected_at"`
	CPUPercent     float64       `json:"cpu_percent"`
	RAMTotalBytes  int64         `json:"ram_total_bytes"`
	RAMUsedBytes   int64         `json:"ram_used_bytes"`
	RAMPercent     float64       `json:"ram_percent"`
	InternetOnline bool          `json:"internet_online"`
	UptimeSeconds  int64         `json:"uptime_seconds"`
	Disks          []diskPayload `json:"disks"`
}

type devicePayload struct {
	CollectedAt string         `json:"collected_at"`
	Categories  []string       `json:"categories,omitempty"`
	Devices     []DeviceRecord `json:"devices"`
}

type DeviceRecord struct {
	Category   string `json:"category"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Connected  bool   `json:"connected"`
	Details    string `json:"details"`
}

func DefaultConfig() Config {
	return Config{
		HeartbeatEvery: 30 * time.Second,
		MetricsEvery:   60 * time.Second,
		DevicesEvery:   5 * time.Minute,
		RequestTimeout: 10 * time.Second,
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.ServerURL) == "" {
		return errors.New("server_url e obrigatorio")
	}
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("token e obrigatorio")
	}
	return nil
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	normalizeDurations(&cfg)
	return cfg, cfg.Validate()
}

func SaveConfig(path string, cfg Config) error {
	normalizeDurations(&cfg)
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func DefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		if base := os.Getenv("ProgramData"); base != "" {
			return filepath.Join(base, "InfoBHMonitor", "agent.json")
		}
		return filepath.Join(`C:\ProgramData`, "InfoBHMonitor", "agent.json")
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "infobh-monitor", "agent.json")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".config", "infobh-monitor", "agent.json")
	}
	return "agent.json"
}

func normalizeDurations(cfg *Config) {
	if cfg.HeartbeatEvery <= 0 {
		cfg.HeartbeatEvery = 30 * time.Second
	}
	if cfg.MetricsEvery <= 0 {
		cfg.MetricsEvery = 60 * time.Second
	}
	if cfg.DevicesEvery <= 0 {
		cfg.DevicesEvery = 5 * time.Minute
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}
}
