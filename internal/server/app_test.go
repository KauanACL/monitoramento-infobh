package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentFlowStoresMetricsDevicesAndDashboard(t *testing.T) {
	store := testStore(t)
	clientID, err := store.CreateClient(context.Background(), "Cliente A", "suporte@cliente.test", "")
	if err != nil {
		t.Fatal(err)
	}
	machine, token, err := store.CreateMachine(context.Background(), clientID, "Recepcao-01")
	if err != nil {
		t.Fatal(err)
	}

	app := NewApp(store, Config{RetentionDays: 30}, slog.New(slog.NewTextHandler(os.Stdout, nil)))
	server := httptest.NewServer(app.Routes())
	defer server.Close()

	postAgent(t, server.URL+"/api/agent/heartbeat", token, AgentHeartbeat{
		Hostname:       "WIN-REC-01",
		OSName:         "Windows 11",
		AgentVersion:   "test",
		IPAddress:      "10.0.0.10",
		InternetOnline: true,
		CollectedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	})
	postAgent(t, server.URL+"/api/agent/metrics", token, AgentMetricPayload{
		CollectedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		CPUPercent:     42,
		RAMTotalBytes:  16 << 30,
		RAMUsedBytes:   8 << 30,
		RAMPercent:     50,
		InternetOnline: true,
		UptimeSeconds:  3600,
		Disks: []AgentDisk{{
			Name:        "C:",
			MountPoint:  "C:\\",
			FileSystem:  "NTFS",
			DriveType:   "fixed",
			TotalBytes:  512 << 30,
			UsedBytes:   256 << 30,
			FreeBytes:   256 << 30,
			UsedPercent: 50,
		}},
	})
	postAgent(t, server.URL+"/api/agent/devices", token, AgentDevicePayload{
		CollectedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Categories:  []string{"USB", "Impressora"},
		Devices: []AgentDevice{
			{Category: "USB", Identifier: "usb-1", Name: "Leitor USB", Status: "OK", Connected: true},
			{Category: "Impressora", Identifier: "printer-1", Name: "HP LaserJet", Status: "OK", Connected: true},
		},
	})

	detail, err := store.MachineDetail(context.Background(), machine.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.Online {
		t.Fatal("machine should be online after heartbeat")
	}
	if detail.LastMetric == nil || detail.LastMetric.CPUPercent != 42 {
		t.Fatalf("unexpected metric: %#v", detail.LastMetric)
	}
	if len(detail.Disks) != 1 || detail.Disks[0].Name != "C:" {
		t.Fatalf("unexpected disks: %#v", detail.Disks)
	}
	if len(detail.Devices) != 2 {
		t.Fatalf("unexpected devices: %#v", detail.Devices)
	}

	postAgent(t, server.URL+"/api/agent/devices", token, AgentDevicePayload{
		CollectedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Categories:  []string{"USB", "Impressora"},
		Devices: []AgentDevice{
			{Category: "Impressora", Identifier: "printer-1", Name: "HP LaserJet", Status: "OK", Connected: true},
		},
	})
	detail, err = store.MachineDetail(context.Background(), machine.ID)
	if err != nil {
		t.Fatal(err)
	}
	var usbDisconnected bool
	for _, device := range detail.Devices {
		if device.Identifier == "usb-1" && !device.Connected {
			usbDisconnected = true
		}
	}
	if !usbDisconnected {
		t.Fatalf("expected missing USB to be marked disconnected: %#v", detail.Devices)
	}

	resp, err := http.Get(server.URL + "/machines/" + "1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected dashboard detail 200, got %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	if !strings.Contains(buf.String(), "Recepcao-01") {
		t.Fatal("detail page should contain machine name")
	}
}

func TestOfflineStatusAndRetention(t *testing.T) {
	store := testStore(t)
	clientID, err := store.CreateClient(context.Background(), "Cliente B", "", "")
	if err != nil {
		t.Fatal(err)
	}
	machine, _, err := store.CreateMachine(context.Background(), clientID, "Servidor-01")
	if err != nil {
		t.Fatal(err)
	}

	old := time.Now().UTC().Add(-3 * time.Minute).Format(time.RFC3339Nano)
	if _, err := store.db.Exec("UPDATE machines SET last_seen_at = ? WHERE id = ?", old, machine.ID); err != nil {
		t.Fatal(err)
	}
	machines, err := store.ListMachines(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(machines) != 1 || machines[0].Online {
		t.Fatalf("expected offline machine, got %#v", machines)
	}

	if err := store.RecordMetrics(context.Background(), machine.ID, AgentMetricPayload{
		CollectedAt:    time.Now().UTC().AddDate(0, 0, -31).Format(time.RFC3339Nano),
		CPUPercent:     10,
		RAMPercent:     10,
		InternetOnline: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE metric_snapshots SET created_at = ? WHERE machine_id = ?",
		time.Now().UTC().AddDate(0, 0, -31).Format(time.RFC3339Nano), machine.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Cleanup(context.Background(), 30); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM metric_snapshots WHERE machine_id = ?", machine.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected old metrics to be removed, got %d", count)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "monitoramento.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func postAgent(t *testing.T, url, token string, payload any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}
}
