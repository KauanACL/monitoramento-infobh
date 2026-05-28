//go:build !windows

package agent

import (
	"context"
	"runtime"
)

func collectHardware(ctx context.Context) (cpuInfo, systemInfo, []MemoryModule) {
	return cpuInfo{
			Name:         runtime.GOARCH,
			Manufacturer: runtime.GOOS,
		},
		systemInfo{
			Manufacturer: runtime.GOOS,
			Model:        runtime.GOARCH,
		},
		nil
}

func collectTemperatures(ctx context.Context) ([]TemperatureRecord, string) {
	return nil, "Temperatura nativa disponivel apenas no agente Windows"
}
