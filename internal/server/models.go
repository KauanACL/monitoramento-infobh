package server

import (
	"encoding/json"
	"time"
)

const (
	DefaultAddr          = ":8080"
	DefaultDBPath        = "data/monitoramento.db"
	DefaultRetentionDays = 30
	DefaultActionPIN     = "110680"
	OfflineAfter         = 2 * time.Minute
)

type Config struct {
	Addr          string
	DBPath        string
	RetentionDays int
	ActionPIN     string
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

type AlertSettings struct {
	CPUPercent     float64
	RAMPercent     float64
	StoragePercent float64
	UpdatedAt      time.Time
}

type HardwareInventory struct {
	MachineID             int64
	CollectedAt           time.Time
	CPUName               string
	CPUManufacturer       string
	CPUCores              int
	CPULogicalProcessors  int
	CPUMaxClockMHz        int
	CPUProcessorID        string
	SystemManufacturer    string
	SystemModel           string
	BaseboardManufacturer string
	BaseboardProduct      string
	BIOSVersion           string
	RAMModules            []RAMModule
}

type RAMModule struct {
	Slot               string `json:"slot"`
	BankLabel          string `json:"bank_label"`
	CapacityBytes      int64  `json:"capacity_bytes"`
	Manufacturer       string `json:"manufacturer"`
	PartNumber         string `json:"part_number"`
	SerialNumber       string `json:"serial_number"`
	SpeedMHz           int    `json:"speed_mhz"`
	ConfiguredClockMHz int    `json:"configured_clock_mhz"`
	MemoryType         string `json:"memory_type"`
	FormFactor         string `json:"form_factor"`
}

type TemperatureReading struct {
	ID             int64
	MachineID      int64
	CollectedAt    time.Time
	Name           string
	SensorType     string
	CurrentCelsius float64
	Source         string
}

type TemperatureStatus struct {
	CollectedAt time.Time
	Available   bool
	Message     string
	Readings    []TemperatureReading
}

type AgentCommand struct {
	ID             int64
	MachineID      int64
	Type           string
	Status         string
	RequestPayload string
	ResultMessage  string
	ErrorMessage   string
	CreatedAt      time.Time
	ClaimedAt      *time.Time
	CompletedAt    *time.Time
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

type AgentHardwarePayload struct {
	CollectedAt string          `json:"collected_at"`
	CPU         AgentCPUInfo    `json:"cpu"`
	System      AgentSystemInfo `json:"system"`
	RAMModules  []RAMModule     `json:"ram_modules"`
}

type AgentCPUInfo struct {
	Name              string `json:"name"`
	Manufacturer      string `json:"manufacturer"`
	Cores             int    `json:"cores"`
	LogicalProcessors int    `json:"logical_processors"`
	MaxClockMHz       int    `json:"max_clock_mhz"`
	ProcessorID       string `json:"processor_id"`
}

type AgentSystemInfo struct {
	Manufacturer          string `json:"manufacturer"`
	Model                 string `json:"model"`
	BaseboardManufacturer string `json:"baseboard_manufacturer"`
	BaseboardProduct      string `json:"baseboard_product"`
	BIOSVersion           string `json:"bios_version"`
}

type AgentTemperaturePayload struct {
	CollectedAt string                    `json:"collected_at"`
	Available   bool                      `json:"available"`
	Message     string                    `json:"message"`
	Readings    []AgentTemperatureReading `json:"readings"`
}

type AgentTemperatureReading struct {
	Name           string  `json:"name"`
	SensorType     string  `json:"sensor_type"`
	CurrentCelsius float64 `json:"current_celsius"`
	Source         string  `json:"source"`
}

type AgentCommandPollResponse struct {
	OK      bool                 `json:"ok"`
	Command *AgentCommandPayload `json:"command,omitempty"`
}

type AgentCommandPayload struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"created_at"`
}

type AgentCommandResultPayload struct {
	Status       string   `json:"status"`
	Message      string   `json:"message"`
	Error        string   `json:"error"`
	RemovedBytes int64    `json:"removed_bytes"`
	Details      []string `json:"details"`
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
	Client           Client
	History          []Metric
	Devices          []DeviceState
	Events           []Event
	Settings         AlertSettings
	Hardware         *HardwareInventory
	Temperatures     TemperatureStatus
	Commands         []AgentCommand
	ActionPINEnabled bool
	Flash            string
	Error            string
}
