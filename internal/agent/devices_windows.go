//go:build windows

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func collectDevices(ctx context.Context) ([]DeviceRecord, []string) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var devices []DeviceRecord
	var categories []string
	if collected, ok := collectPowerShell(ctx, "USB", `
$items = @(Get-CimInstance Win32_PnPEntity | Where-Object { $_.PNPClass -eq 'USB' -or $_.DeviceID -like 'USB*' } | Select-Object Name,DeviceID,Status,PNPClass)
$items | ConvertTo-Json -Compress -Depth 3
`); ok {
		devices = append(devices, collected...)
		categories = append(categories, "USB")
	}
	if collected, ok := collectPowerShell(ctx, "Impressora", `
$items = @(Get-CimInstance Win32_Printer | Select-Object Name,DeviceID,PrinterStatus,Default,WorkOffline)
$items | ConvertTo-Json -Compress -Depth 3
`); ok {
		devices = append(devices, collected...)
		categories = append(categories, "Impressora")
	}
	if collected, ok := collectPowerShell(ctx, "Armazenamento", `
$items = @(Get-CimInstance Win32_DiskDrive | Select-Object Model,SerialNumber,InterfaceType,MediaType,Status,Size)
$items | ConvertTo-Json -Compress -Depth 3
`); ok {
		devices = append(devices, collected...)
		categories = append(categories, "Armazenamento")
	}
	return devices, categories
}

func collectPowerShell(ctx context.Context, category, script string) ([]DeviceRecord, bool) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil, false
	}

	var raw []map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		var single map[string]any
		if err := json.Unmarshal(out, &single); err != nil {
			return nil, false
		}
		raw = []map[string]any{single}
	}

	devices := make([]DeviceRecord, 0, len(raw))
	for _, item := range raw {
		device := DeviceRecord{Category: category, Connected: true}
		switch category {
		case "USB":
			device.Name = stringField(item, "Name")
			device.Identifier = firstNonEmpty(stringField(item, "DeviceID"), device.Name)
			device.Status = stringField(item, "Status")
			device.Details = "Classe: " + stringField(item, "PNPClass")
		case "Impressora":
			device.Name = stringField(item, "Name")
			device.Identifier = firstNonEmpty(stringField(item, "DeviceID"), device.Name)
			device.Status = firstNonEmpty(stringField(item, "PrinterStatus"), "OK")
			device.Details = "Default: " + stringField(item, "Default") + " Offline: " + stringField(item, "WorkOffline")
		case "Armazenamento":
			device.Name = firstNonEmpty(stringField(item, "Model"), "Disco")
			device.Identifier = firstNonEmpty(stringField(item, "SerialNumber"), device.Name)
			device.Status = stringField(item, "Status")
			device.Details = "Interface: " + stringField(item, "InterfaceType") + " Midia: " + stringField(item, "MediaType") + " Tamanho: " + stringField(item, "Size")
		}
		if strings.TrimSpace(device.Name) != "" {
			devices = append(devices, device)
		}
	}
	return devices, true
}

func stringField(item map[string]any, key string) string {
	value, ok := item[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case bool:
		if typed {
			return "sim"
		}
		return "nao"
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
