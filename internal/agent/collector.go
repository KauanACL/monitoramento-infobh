package agent

import (
	"context"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type Collector struct{}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) Heartbeat(ctx context.Context) heartbeatPayload {
	hostname, _ := os.Hostname()
	return heartbeatPayload{
		Hostname:       hostname,
		OSName:         osDescription(ctx),
		AgentVersion:   Version,
		IPAddress:      primaryIPv4(),
		InternetOnline: internetOnline(),
		CollectedAt:    nowRFC3339(),
	}
}

func (c *Collector) Metrics(ctx context.Context) metricPayload {
	vm, _ := mem.VirtualMemoryWithContext(ctx)
	percentages, _ := cpu.PercentWithContext(ctx, time.Second, false)
	var cpuPercent float64
	if len(percentages) > 0 {
		cpuPercent = percentages[0]
	}
	boot, _ := host.BootTimeWithContext(ctx)

	payload := metricPayload{
		CollectedAt:    nowRFC3339(),
		CPUPercent:     cpuPercent,
		RAMTotalBytes:  int64(vm.Total),
		RAMUsedBytes:   int64(vm.Used),
		RAMPercent:     vm.UsedPercent,
		InternetOnline: internetOnline(),
		UptimeSeconds:  int64(time.Since(time.Unix(int64(boot), 0)).Seconds()),
		Disks:          collectDisks(ctx),
	}
	return payload
}

func (c *Collector) Devices(ctx context.Context) devicePayload {
	devices, categories := collectDevices(ctx)
	return devicePayload{
		CollectedAt: nowRFC3339(),
		Categories:  categories,
		Devices:     devices,
	}
}

func (c *Collector) Hardware(ctx context.Context) hardwarePayload {
	cpu, system, modules := collectHardware(ctx)
	return hardwarePayload{
		CollectedAt: nowRFC3339(),
		CPU:         cpu,
		System:      system,
		RAMModules:  modules,
	}
}

func (c *Collector) Temperatures(ctx context.Context) temperaturePayload {
	readings, message := collectTemperatures(ctx)
	return temperaturePayload{
		CollectedAt: nowRFC3339(),
		Available:   len(readings) > 0,
		Message:     message,
		Readings:    readings,
	}
}

func collectDisks(ctx context.Context) []diskPayload {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var disks []diskPayload
	for _, partition := range partitions {
		key := partition.Mountpoint
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
		if err != nil {
			continue
		}
		disks = append(disks, diskPayload{
			Name:        firstNonEmpty(partition.Device, partition.Mountpoint),
			MountPoint:  partition.Mountpoint,
			FileSystem:  partition.Fstype,
			DriveType:   driveType(partition.Opts),
			TotalBytes:  int64(usage.Total),
			UsedBytes:   int64(usage.Used),
			FreeBytes:   int64(usage.Free),
			UsedPercent: usage.UsedPercent,
		})
	}
	return disks
}

func osDescription(ctx context.Context) string {
	info, err := host.InfoWithContext(ctx)
	if err == nil && strings.TrimSpace(info.Platform) != "" {
		return strings.TrimSpace(info.Platform + " " + info.PlatformVersion + " " + info.KernelArch)
	}
	return runtime.GOOS + "/" + runtime.GOARCH
}

func internetOnline() bool {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.Dial("tcp", "1.1.1.1:53")
	if err != nil {
		conn, err = dialer.Dial("tcp", "8.8.8.8:53")
	}
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func primaryIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			ip = ip.To4()
			if ip != nil {
				return ip.String()
			}
		}
	}
	return ""
}

func driveType(opts []string) string {
	joined := strings.ToLower(strings.Join(opts, ","))
	if strings.Contains(joined, "removable") {
		return "removable"
	}
	if strings.Contains(joined, "fixed") {
		return "fixed"
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
