//go:build windows

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func collectHardware(ctx context.Context) (cpuInfo, systemInfo, []MemoryModule) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	script := `
$processor = Get-CimInstance Win32_Processor | Select-Object -First 1 Name,Manufacturer,NumberOfCores,NumberOfLogicalProcessors,MaxClockSpeed,ProcessorId
$system = Get-CimInstance Win32_ComputerSystem | Select-Object -First 1 Manufacturer,Model
$baseboard = Get-CimInstance Win32_BaseBoard | Select-Object -First 1 Manufacturer,Product
$bios = Get-CimInstance Win32_BIOS | Select-Object -First 1 SMBIOSBIOSVersion
$memory = @(Get-CimInstance Win32_PhysicalMemory | Select-Object BankLabel,DeviceLocator,Capacity,Manufacturer,PartNumber,SerialNumber,Speed,ConfiguredClockSpeed,SMBIOSMemoryType,FormFactor)
[pscustomobject]@{
  CPU = $processor
  System = $system
  Baseboard = $baseboard
  BIOS = $bios
  RAM = $memory
} | ConvertTo-Json -Compress -Depth 5
`
	out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script).Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return cpuInfo{}, systemInfo{}, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return cpuInfo{}, systemInfo{}, nil
	}
	cpuRaw := mapField(raw, "CPU")
	systemRaw := mapField(raw, "System")
	baseboardRaw := mapField(raw, "Baseboard")
	biosRaw := mapField(raw, "BIOS")

	cpu := cpuInfo{
		Name:              stringField(cpuRaw, "Name"),
		Manufacturer:      stringField(cpuRaw, "Manufacturer"),
		Cores:             intField(cpuRaw, "NumberOfCores"),
		LogicalProcessors: intField(cpuRaw, "NumberOfLogicalProcessors"),
		MaxClockMHz:       intField(cpuRaw, "MaxClockSpeed"),
		ProcessorID:       stringField(cpuRaw, "ProcessorId"),
	}
	system := systemInfo{
		Manufacturer:          stringField(systemRaw, "Manufacturer"),
		Model:                 stringField(systemRaw, "Model"),
		BaseboardManufacturer: stringField(baseboardRaw, "Manufacturer"),
		BaseboardProduct:      stringField(baseboardRaw, "Product"),
		BIOSVersion:           stringField(biosRaw, "SMBIOSBIOSVersion"),
	}
	return cpu, system, memoryModules(raw["RAM"])
}

func collectTemperatures(ctx context.Context) ([]TemperatureRecord, string) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	script := `
$items = @(Get-CimInstance -Namespace root/wmi -ClassName MSAcpi_ThermalZoneTemperature -ErrorAction SilentlyContinue | Select-Object InstanceName,CurrentTemperature)
$items | ForEach-Object {
  $celsius = [math]::Round((($_.CurrentTemperature / 10) - 273.15), 1)
  [pscustomobject]@{
    Name = $_.InstanceName
    SensorType = 'thermal_zone'
    CurrentCelsius = $celsius
    Source = 'MSAcpi_ThermalZoneTemperature'
  }
} | ConvertTo-Json -Compress -Depth 3
`
	out, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script).Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil, "Sensores nativos indisponiveis"
	}

	var raw []map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		var single map[string]any
		if err := json.Unmarshal(out, &single); err != nil {
			return nil, "Sensores nativos indisponiveis"
		}
		raw = []map[string]any{single}
	}

	readings := make([]TemperatureRecord, 0, len(raw))
	for _, item := range raw {
		name := stringField(item, "Name")
		celsius := floatField(item, "CurrentCelsius")
		if name == "" || celsius < -50 || celsius > 150 {
			continue
		}
		readings = append(readings, TemperatureRecord{
			Name:           name,
			SensorType:     firstNonEmpty(stringField(item, "SensorType"), "thermal_zone"),
			CurrentCelsius: celsius,
			Source:         firstNonEmpty(stringField(item, "Source"), "Windows WMI"),
		})
	}
	if len(readings) == 0 {
		return nil, "Sensores nativos indisponiveis"
	}
	return readings, ""
}

func mapField(item map[string]any, key string) map[string]any {
	value, ok := item[key]
	if !ok || value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func memoryModules(value any) []MemoryModule {
	var raw []map[string]any
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				raw = append(raw, mapped)
			}
		}
	case map[string]any:
		raw = append(raw, typed)
	}
	modules := make([]MemoryModule, 0, len(raw))
	for _, item := range raw {
		module := MemoryModule{
			Slot:               stringField(item, "DeviceLocator"),
			BankLabel:          stringField(item, "BankLabel"),
			CapacityBytes:      int64Field(item, "Capacity"),
			Manufacturer:       stringField(item, "Manufacturer"),
			PartNumber:         stringField(item, "PartNumber"),
			SerialNumber:       stringField(item, "SerialNumber"),
			SpeedMHz:           intField(item, "Speed"),
			ConfiguredClockMHz: intField(item, "ConfiguredClockSpeed"),
			MemoryType:         memoryTypeName(intField(item, "SMBIOSMemoryType")),
			FormFactor:         formFactorName(intField(item, "FormFactor")),
		}
		if strings.TrimSpace(module.Slot) != "" || module.CapacityBytes > 0 {
			modules = append(modules, module)
		}
	}
	return modules
}

func intField(item map[string]any, key string) int {
	value, ok := item[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		parsed, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(typed)))
		return parsed
	}
}

func int64Field(item map[string]any, key string) int64 {
	value, ok := item[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(typed)), 10, 64)
		return parsed
	}
}

func floatField(item map[string]any, key string) float64 {
	value, ok := item[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	default:
		parsed, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(fmt.Sprint(typed)), ",", "."), 64)
		return parsed
	}
}

func memoryTypeName(value int) string {
	switch value {
	case 20:
		return "DDR"
	case 21:
		return "DDR2"
	case 24:
		return "DDR3"
	case 26:
		return "DDR4"
	case 30:
		return "LPDDR4"
	case 34:
		return "DDR5"
	default:
		if value == 0 {
			return ""
		}
		return fmt.Sprintf("SMBIOS %d", value)
	}
}

func formFactorName(value int) string {
	switch value {
	case 8:
		return "DIMM"
	case 12:
		return "SODIMM"
	case 13:
		return "Micro-DIMM"
	default:
		if value == 0 {
			return ""
		}
		return fmt.Sprintf("FormFactor %d", value)
	}
}
