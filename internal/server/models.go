package server

import "time"

const (
	DefaultAddr          = ":8080"
	DefaultDBPath        = "data/monitoramento.db"
	DefaultRetentionDays = 30
	OfflineAfter         = 2 * time.Minute
)

type Config struct {
	Addr          string
	DBPath        string
	RetentionDays int
}

type Client struct {
	ID        int64
	Name      string
	Contact   string
	Notes     string
	CreatedAt time.Time
}

type Machine struct {
	ID             int64
	ClientID       int64
	ClientName     string
	Name           string
	Hostname       string
	TokenHint      string
	AgentVersion   string
	OSName         string
	IPAddress      string
	InternetOnline bool
	LastSeenAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Metric struct {
	ID             int64
	MachineID      int64
	CollectedAt    time.Time
	CPUPercent     float64
	RAMTotalBytes  int64
	RAMUsedBytes   int64
	RAMPercent     float64
	InternetOnline bool
	UptimeSeconds  int64
	CreatedAt      time.Time
}

type Disk struct {
	ID          int64
	MetricID    int64
	MachineID   int64
	CollectedAt time.Time
	Name        string
	MountPoint  string
	FileSystem  string
	DriveType   string
	TotalBytes  int64
	UsedBytes   int64
	FreeBytes   int64
	UsedPercent float64
}

type DeviceState struct {
	MachineID  int64
	Category   string
	Identifier string
	Name       string
	Status     string
	Connected  bool
	Details    string
	FirstSeen  time.Time
	LastSeen   time.Time
}

type Event struct {
	ID        int64
	MachineID int64
	Severity  string
	Type      string
	Message   string
	Metadata  string
	CreatedAt time.Time
}

type AgentHeartbeat struct {
	Hostname       string `json:"hostname"`
	OSName         string `json:"os_name"`
	AgentVersion   string `json:"agent_version"`
	IPAddress      string `json:"ip_address"`
	InternetOnline bool   `json:"internet_online"`
	CollectedAt    string `json:"collected_at"`
}

type AgentMetricPayload struct {
	CollectedAt    string      `json:"collected_at"`
	CPUPercent     float64     `json:"cpu_percent"`
	RAMTotalBytes  int64       `json:"ram_total_bytes"`
	RAMUsedBytes   int64       `json:"ram_used_bytes"`
	RAMPercent     float64     `json:"ram_percent"`
	InternetOnline bool        `json:"internet_online"`
	UptimeSeconds  int64       `json:"uptime_seconds"`
	Disks          []AgentDisk `json:"disks"`
}

type AgentDisk struct {
	Name        string  `json:"name"`
	MountPoint  string  `json:"mount_point"`
	FileSystem  string  `json:"file_system"`
	DriveType   string  `json:"drive_type"`
	TotalBytes  int64   `json:"total_bytes"`
	UsedBytes   int64   `json:"used_bytes"`
	FreeBytes   int64   `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type AgentDevicePayload struct {
	CollectedAt string        `json:"collected_at"`
	Categories  []string      `json:"categories,omitempty"`
	Devices     []AgentDevice `json:"devices"`
}

type AgentDevice struct {
	Category   string `json:"category"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Connected  bool   `json:"connected"`
	Details    string `json:"details"`
}

type MachineOverview struct {
	Machine
	Online     bool
	LastMetric *Metric
	Disks      []Disk
	Alerts     []string
	AlertLevel string
}

type ClientOverview struct {
	Client
	TotalMachines   int
	OnlineMachines  int
	OfflineMachines int
	AlertCount      int
}

type DashboardData struct {
	GeneratedAt     time.Time
	Clients         []ClientOverview
	Machines        []MachineOverview
	TotalClients    int
	TotalMachines   int
	OnlineMachines  int
	OfflineMachines int
	AlertCount      int
}

type MachineDetail struct {
	MachineOverview
	Client  Client
	History []Metric
	Devices []DeviceState
	Events  []Event
}
