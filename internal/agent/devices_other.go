//go:build !windows

package agent

import "context"

func collectDevices(ctx context.Context) ([]DeviceRecord, []string) {
	return []DeviceRecord{
		{
			Category:   "Armazenamento",
			Identifier: "local-host-storage",
			Name:       "Armazenamento local",
			Status:     "OK",
			Connected:  true,
			Details:    "Coleta detalhada de USB e impressoras disponivel no Windows",
		},
	}, []string{"Armazenamento"}
}
