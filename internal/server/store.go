package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	store := &Store{db: db}
	if err := store.configure(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) configure(ctx context.Context) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, pragma := range pragmas {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS clients (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	contact TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS machines (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	client_id INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	hostname TEXT NOT NULL DEFAULT '',
	token_hash TEXT NOT NULL UNIQUE,
	token_hint TEXT NOT NULL,
	agent_version TEXT NOT NULL DEFAULT '',
	os_name TEXT NOT NULL DEFAULT '',
	ip_address TEXT NOT NULL DEFAULT '',
	internet_online INTEGER NOT NULL DEFAULT 0,
	last_seen_at TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_machines_client ON machines(client_id);
CREATE INDEX IF NOT EXISTS idx_machines_last_seen ON machines(last_seen_at);

CREATE TABLE IF NOT EXISTS metric_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	machine_id INTEGER NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
	collected_at TEXT NOT NULL,
	cpu_percent REAL NOT NULL DEFAULT 0,
	ram_total_bytes INTEGER NOT NULL DEFAULT 0,
	ram_used_bytes INTEGER NOT NULL DEFAULT 0,
	ram_percent REAL NOT NULL DEFAULT 0,
	internet_online INTEGER NOT NULL DEFAULT 0,
	uptime_seconds INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_metrics_machine_collected ON metric_snapshots(machine_id, collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_metrics_created ON metric_snapshots(created_at);

CREATE TABLE IF NOT EXISTS disk_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	metric_id INTEGER NOT NULL REFERENCES metric_snapshots(id) ON DELETE CASCADE,
	machine_id INTEGER NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
	collected_at TEXT NOT NULL,
	name TEXT NOT NULL,
	mount_point TEXT NOT NULL DEFAULT '',
	file_system TEXT NOT NULL DEFAULT '',
	drive_type TEXT NOT NULL DEFAULT '',
	total_bytes INTEGER NOT NULL DEFAULT 0,
	used_bytes INTEGER NOT NULL DEFAULT 0,
	free_bytes INTEGER NOT NULL DEFAULT 0,
	used_percent REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_disks_metric ON disk_snapshots(metric_id);
CREATE INDEX IF NOT EXISTS idx_disks_machine_collected ON disk_snapshots(machine_id, collected_at DESC);

CREATE TABLE IF NOT EXISTS device_states (
	machine_id INTEGER NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
	category TEXT NOT NULL,
	identifier TEXT NOT NULL,
	name TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT '',
	connected INTEGER NOT NULL DEFAULT 1,
	details TEXT NOT NULL DEFAULT '',
	first_seen_at TEXT NOT NULL,
	last_seen_at TEXT NOT NULL,
	PRIMARY KEY (machine_id, category, identifier)
);
CREATE INDEX IF NOT EXISTS idx_devices_machine ON device_states(machine_id, category, name);

CREATE TABLE IF NOT EXISTS device_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	machine_id INTEGER NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
	category TEXT NOT NULL,
	identifier TEXT NOT NULL,
	name TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT '',
	event_type TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_device_events_machine_created ON device_events(machine_id, created_at DESC);

CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	machine_id INTEGER NOT NULL REFERENCES machines(id) ON DELETE CASCADE,
	severity TEXT NOT NULL,
	type TEXT NOT NULL,
	message TEXT NOT NULL,
	metadata TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_machine_created ON events(machine_id, created_at DESC);
`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) Cleanup(ctx context.Context, retentionDays int) error {
	if retentionDays <= 0 {
		retentionDays = DefaultRetentionDays
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339Nano)
	queries := []string{
		"DELETE FROM metric_snapshots WHERE created_at < ?",
		"DELETE FROM device_events WHERE created_at < ?",
		"DELETE FROM events WHERE created_at < ?",
	}
	for _, query := range queries {
		if _, err := s.db.ExecContext(ctx, query, cutoff); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateClient(ctx context.Context, name, contact, notes string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("nome do cliente e obrigatorio")
	}
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO clients (name, contact, notes) VALUES (?, ?, ?)",
		name, strings.TrimSpace(contact), strings.TrimSpace(notes),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CreateMachine(ctx context.Context, clientID int64, name string) (Machine, string, error) {
	name = strings.TrimSpace(name)
	if clientID <= 0 {
		return Machine{}, "", errors.New("cliente e obrigatorio")
	}
	if name == "" {
		return Machine{}, "", errors.New("nome da maquina e obrigatorio")
	}
	token, err := newToken()
	if err != nil {
		return Machine{}, "", err
	}
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO machines (client_id, name, token_hash, token_hint) VALUES (?, ?, ?, ?)",
		clientID, name, hashToken(token), tokenHint(token),
	)
	if err != nil {
		return Machine{}, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Machine{}, "", err
	}
	machine, err := s.GetMachine(ctx, id)
	if err != nil {
		return Machine{}, "", err
	}
	return machine, token, nil
}

func (s *Store) RotateMachineToken(ctx context.Context, machineID int64) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	res, err := s.db.ExecContext(ctx,
		"UPDATE machines SET token_hash = ?, token_hint = ?, updated_at = ? WHERE id = ?",
		hashToken(token), tokenHint(token), nowString(), machineID,
	)
	if err != nil {
		return "", err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	if affected == 0 {
		return "", sql.ErrNoRows
	}
	return token, nil
}

func (s *Store) GetMachine(ctx context.Context, id int64) (Machine, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT m.id, m.client_id, c.name, m.name, m.hostname, m.token_hint, m.agent_version, m.os_name,
       m.ip_address, m.internet_online, m.last_seen_at, m.created_at, m.updated_at
FROM machines m
JOIN clients c ON c.id = m.client_id
WHERE m.id = ?`, id)
	return scanMachine(row)
}

func (s *Store) GetClient(ctx context.Context, id int64) (Client, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, name, contact, notes, created_at FROM clients WHERE id = ?", id)
	var c Client
	var created string
	if err := row.Scan(&c.ID, &c.Name, &c.Contact, &c.Notes, &created); err != nil {
		return Client{}, err
	}
	c.CreatedAt = parseDBTime(created)
	return c, nil
}

func (s *Store) AuthenticateAgent(ctx context.Context, token string) (Machine, error) {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return Machine{}, sql.ErrNoRows
	}
	row := s.db.QueryRowContext(ctx, `
SELECT m.id, m.client_id, c.name, m.name, m.hostname, m.token_hint, m.agent_version, m.os_name,
       m.ip_address, m.internet_online, m.last_seen_at, m.created_at, m.updated_at
FROM machines m
JOIN clients c ON c.id = m.client_id
WHERE m.token_hash = ?`, hashToken(token))
	return scanMachine(row)
}

func (s *Store) RecordHeartbeat(ctx context.Context, machineID int64, hb AgentHeartbeat) error {
	at := parseAgentTime(hb.CollectedAt)
	machine, _ := s.GetMachine(ctx, machineID)
	wasOffline := machine.LastSeenAt == nil || time.Since(machine.LastSeenAt.UTC()) > OfflineAfter

	_, err := s.db.ExecContext(ctx, `
UPDATE machines
SET hostname = ?, agent_version = ?, os_name = ?, ip_address = ?, internet_online = ?,
    last_seen_at = ?, updated_at = ?
WHERE id = ?`,
		strings.TrimSpace(hb.Hostname), strings.TrimSpace(hb.AgentVersion), strings.TrimSpace(hb.OSName),
		strings.TrimSpace(hb.IPAddress), boolInt(hb.InternetOnline), at.Format(time.RFC3339Nano),
		nowString(), machineID,
	)
	if err != nil {
		return err
	}
	if wasOffline {
		_ = s.addEvent(ctx, machineID, "info", "online", "Maquina voltou a ficar online", "")
	}
	return nil
}

func (s *Store) RecordMetrics(ctx context.Context, machineID int64, payload AgentMetricPayload) error {
	collectedAt := parseAgentTime(payload.CollectedAt)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
UPDATE machines
SET internet_online = ?, last_seen_at = ?, updated_at = ?
WHERE id = ?`, boolInt(payload.InternetOnline), collectedAt.Format(time.RFC3339Nano), nowString(), machineID)
	if err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `
INSERT INTO metric_snapshots
	(machine_id, collected_at, cpu_percent, ram_total_bytes, ram_used_bytes, ram_percent, internet_online, uptime_seconds)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		machineID, collectedAt.Format(time.RFC3339Nano), clampPercent(payload.CPUPercent),
		payload.RAMTotalBytes, payload.RAMUsedBytes, clampPercent(payload.RAMPercent),
		boolInt(payload.InternetOnline), payload.UptimeSeconds,
	)
	if err != nil {
		return err
	}
	metricID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, disk := range payload.Disks {
		if strings.TrimSpace(disk.Name) == "" && strings.TrimSpace(disk.MountPoint) == "" {
			continue
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO disk_snapshots
	(metric_id, machine_id, collected_at, name, mount_point, file_system, drive_type, total_bytes, used_bytes, free_bytes, used_percent)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			metricID, machineID, collectedAt.Format(time.RFC3339Nano), firstNonEmpty(disk.Name, disk.MountPoint),
			disk.MountPoint, disk.FileSystem, disk.DriveType, disk.TotalBytes, disk.UsedBytes, disk.FreeBytes,
			clampPercent(disk.UsedPercent),
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) RecordDevices(ctx context.Context, machineID int64, payload AgentDevicePayload) error {
	collectedAt := parseAgentTime(payload.CollectedAt)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	seenByCategory := map[string]map[string]bool{}
	for _, category := range payload.Categories {
		normalized := normalizeDeviceCategory(category)
		if normalized != "" {
			seenByCategory[normalized] = map[string]bool{}
		}
	}

	for _, device := range payload.Devices {
		category := normalizeDeviceCategory(device.Category)
		identifier := strings.TrimSpace(device.Identifier)
		name := strings.TrimSpace(device.Name)
		if identifier == "" {
			identifier = name
		}
		if category == "" || identifier == "" || name == "" {
			continue
		}
		if seenByCategory[category] == nil {
			seenByCategory[category] = map[string]bool{}
		}
		seenByCategory[category][identifier] = true

		var previousStatus string
		var previousConnected int
		err := tx.QueryRowContext(ctx, `
SELECT status, connected
FROM device_states
WHERE machine_id = ? AND category = ? AND identifier = ?`, machineID, category, identifier).
			Scan(&previousStatus, &previousConnected)
		eventType := "seen"
		if errors.Is(err, sql.ErrNoRows) {
			eventType = "connected"
		} else if err != nil {
			return err
		} else if previousStatus != device.Status || previousConnected != boolInt(device.Connected) {
			eventType = "changed"
			if previousConnected == 1 && !device.Connected {
				eventType = "disconnected"
			}
			if previousConnected == 0 && device.Connected {
				eventType = "connected"
			}
		}

		_, err = tx.ExecContext(ctx, `
INSERT INTO device_states
	(machine_id, category, identifier, name, status, connected, details, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(machine_id, category, identifier) DO UPDATE SET
	name = excluded.name,
	status = excluded.status,
	connected = excluded.connected,
	details = excluded.details,
	last_seen_at = excluded.last_seen_at`,
			machineID, category, identifier, name, strings.TrimSpace(device.Status), boolInt(device.Connected),
			strings.TrimSpace(device.Details), collectedAt.Format(time.RFC3339Nano), collectedAt.Format(time.RFC3339Nano),
		)
		if err != nil {
			return err
		}
		if eventType != "seen" {
			_, err = tx.ExecContext(ctx, `
INSERT INTO device_events (machine_id, category, identifier, name, status, event_type, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
				machineID, category, identifier, name, strings.TrimSpace(device.Status), eventType,
				collectedAt.Format(time.RFC3339Nano),
			)
			if err != nil {
				return err
			}
		}
	}

	for category, seen := range seenByCategory {
		rows, err := tx.QueryContext(ctx, `
SELECT identifier, name, status
FROM device_states
WHERE machine_id = ? AND category = ? AND connected = 1`, machineID, category)
		if err != nil {
			return err
		}
		type staleDevice struct {
			identifier string
			name       string
			status     string
		}
		var stale []staleDevice
		for rows.Next() {
			var item staleDevice
			if err := rows.Scan(&item.identifier, &item.name, &item.status); err != nil {
				_ = rows.Close()
				return err
			}
			if !seen[item.identifier] {
				stale = append(stale, item)
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, item := range stale {
			_, err = tx.ExecContext(ctx, `
UPDATE device_states
SET connected = 0, status = 'Desconectado', last_seen_at = ?
WHERE machine_id = ? AND category = ? AND identifier = ?`,
				collectedAt.Format(time.RFC3339Nano), machineID, category, item.identifier)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `
INSERT INTO device_events (machine_id, category, identifier, name, status, event_type, created_at)
VALUES (?, ?, ?, ?, ?, 'disconnected', ?)`,
				machineID, category, item.identifier, item.name, firstNonEmpty(item.status, "Desconectado"),
				collectedAt.Format(time.RFC3339Nano))
			if err != nil {
				return err
			}
		}
	}

	_, err = tx.ExecContext(ctx, "UPDATE machines SET last_seen_at = ?, updated_at = ? WHERE id = ?",
		collectedAt.Format(time.RFC3339Nano), nowString(), machineID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Dashboard(ctx context.Context) (DashboardData, error) {
	clients, err := s.ListClients(ctx)
	if err != nil {
		return DashboardData{}, err
	}
	machines, err := s.ListMachines(ctx, 0)
	if err != nil {
		return DashboardData{}, err
	}
	data := DashboardData{
		GeneratedAt:  time.Now(),
		Clients:      clients,
		Machines:     machines,
		TotalClients: len(clients),
	}
	for _, machine := range machines {
		data.TotalMachines++
		if machine.Online {
			data.OnlineMachines++
		} else {
			data.OfflineMachines++
		}
		data.AlertCount += len(machine.Alerts)
	}
	return data, nil
}

func (s *Store) ListClients(ctx context.Context) ([]ClientOverview, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, contact, notes, created_at FROM clients ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []ClientOverview
	for rows.Next() {
		var c Client
		var created string
		if err := rows.Scan(&c.ID, &c.Name, &c.Contact, &c.Notes, &created); err != nil {
			return nil, err
		}
		c.CreatedAt = parseDBTime(created)
		clients = append(clients, ClientOverview{Client: c})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	machines, err := s.ListMachines(ctx, 0)
	if err != nil {
		return nil, err
	}
	byClient := map[int64]*ClientOverview{}
	for i := range clients {
		byClient[clients[i].ID] = &clients[i]
	}
	for _, machine := range machines {
		client := byClient[machine.ClientID]
		if client == nil {
			continue
		}
		client.TotalMachines++
		if machine.Online {
			client.OnlineMachines++
		} else {
			client.OfflineMachines++
		}
		client.AlertCount += len(machine.Alerts)
	}
	return clients, nil
}

func (s *Store) ListMachines(ctx context.Context, clientID int64) ([]MachineOverview, error) {
	query := `
SELECT m.id, m.client_id, c.name, m.name, m.hostname, m.token_hint, m.agent_version, m.os_name,
       m.ip_address, m.internet_online, m.last_seen_at, m.created_at, m.updated_at,
       lm.id, lm.machine_id, lm.collected_at, lm.cpu_percent, lm.ram_total_bytes, lm.ram_used_bytes,
       lm.ram_percent, lm.internet_online, lm.uptime_seconds, lm.created_at
FROM machines m
JOIN clients c ON c.id = m.client_id
LEFT JOIN metric_snapshots lm ON lm.id = (
	SELECT id FROM metric_snapshots WHERE machine_id = m.id ORDER BY collected_at DESC LIMIT 1
)
WHERE (? = 0 OR m.client_id = ?)
ORDER BY c.name, m.name`
	rows, err := s.db.QueryContext(ctx, query, clientID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var machines []MachineOverview
	for rows.Next() {
		machine, metric, err := scanMachineWithMetric(rows)
		if err != nil {
			return nil, err
		}
		overview := MachineOverview{Machine: machine, LastMetric: metric}
		overview.Online = machine.LastSeenAt != nil && time.Since(machine.LastSeenAt.UTC()) <= OfflineAfter
		if metric != nil {
			disks, err := s.listDisksForMetric(ctx, metric.ID)
			if err != nil {
				return nil, err
			}
			overview.Disks = disks
		}
		overview.Alerts = buildAlerts(overview)
		overview.AlertLevel = alertLevel(overview.Alerts)
		machines = append(machines, overview)
	}
	return machines, rows.Err()
}

func (s *Store) MachineDetail(ctx context.Context, id int64) (MachineDetail, error) {
	machine, err := s.GetMachine(ctx, id)
	if err != nil {
		return MachineDetail{}, err
	}
	client, err := s.GetClient(ctx, machine.ClientID)
	if err != nil {
		return MachineDetail{}, err
	}
	machines, err := s.ListMachines(ctx, machine.ClientID)
	if err != nil {
		return MachineDetail{}, err
	}
	var overview MachineOverview
	for _, item := range machines {
		if item.ID == id {
			overview = item
			break
		}
	}
	history, err := s.metricHistory(ctx, id, 60)
	if err != nil {
		return MachineDetail{}, err
	}
	devices, err := s.deviceStates(ctx, id)
	if err != nil {
		return MachineDetail{}, err
	}
	events, err := s.events(ctx, id, 30)
	if err != nil {
		return MachineDetail{}, err
	}
	return MachineDetail{
		MachineOverview: overview,
		Client:          client,
		History:         history,
		Devices:         devices,
		Events:          events,
	}, nil
}

func (s *Store) metricHistory(ctx context.Context, machineID int64, limit int) ([]Metric, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, machine_id, collected_at, cpu_percent, ram_total_bytes, ram_used_bytes, ram_percent,
       internet_online, uptime_seconds, created_at
FROM (
	SELECT * FROM metric_snapshots
	WHERE machine_id = ?
	ORDER BY collected_at DESC
	LIMIT ?
) ORDER BY collected_at ASC`, machineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var metrics []Metric
	for rows.Next() {
		metric, err := scanMetric(rows)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	return metrics, rows.Err()
}

func (s *Store) deviceStates(ctx context.Context, machineID int64) ([]DeviceState, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT machine_id, category, identifier, name, status, connected, details, first_seen_at, last_seen_at
FROM device_states
WHERE machine_id = ?
ORDER BY category, name`, machineID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []DeviceState
	for rows.Next() {
		var d DeviceState
		var connected int
		var firstSeen, lastSeen string
		if err := rows.Scan(&d.MachineID, &d.Category, &d.Identifier, &d.Name, &d.Status, &connected,
			&d.Details, &firstSeen, &lastSeen); err != nil {
			return nil, err
		}
		d.Connected = connected == 1
		d.FirstSeen = parseDBTime(firstSeen)
		d.LastSeen = parseDBTime(lastSeen)
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Store) events(ctx context.Context, machineID int64, limit int) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, machine_id, severity, type, message, metadata, created_at
FROM events
WHERE machine_id = ?
UNION ALL
SELECT id, machine_id, 'info' AS severity, event_type AS type,
       category || ': ' || name AS message, identifier AS metadata, created_at
FROM device_events
WHERE machine_id = ?
ORDER BY created_at DESC
LIMIT ?`, machineID, machineID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var e Event
		var created string
		if err := rows.Scan(&e.ID, &e.MachineID, &e.Severity, &e.Type, &e.Message, &e.Metadata, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseDBTime(created)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) listDisksForMetric(ctx context.Context, metricID int64) ([]Disk, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, metric_id, machine_id, collected_at, name, mount_point, file_system, drive_type,
       total_bytes, used_bytes, free_bytes, used_percent
FROM disk_snapshots
WHERE metric_id = ?
ORDER BY used_percent DESC`, metricID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var disks []Disk
	for rows.Next() {
		var d Disk
		var collected string
		if err := rows.Scan(&d.ID, &d.MetricID, &d.MachineID, &collected, &d.Name, &d.MountPoint,
			&d.FileSystem, &d.DriveType, &d.TotalBytes, &d.UsedBytes, &d.FreeBytes, &d.UsedPercent); err != nil {
			return nil, err
		}
		d.CollectedAt = parseDBTime(collected)
		disks = append(disks, d)
	}
	return disks, rows.Err()
}

func (s *Store) addEvent(ctx context.Context, machineID int64, severity, eventType, message, metadata string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO events (machine_id, severity, type, message, metadata, created_at)
VALUES (?, ?, ?, ?, ?, ?)`, machineID, severity, eventType, message, metadata, nowString())
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMachine(row rowScanner) (Machine, error) {
	var m Machine
	var internet int
	var lastSeen sql.NullString
	var created, updated string
	err := row.Scan(&m.ID, &m.ClientID, &m.ClientName, &m.Name, &m.Hostname, &m.TokenHint,
		&m.AgentVersion, &m.OSName, &m.IPAddress, &internet, &lastSeen, &created, &updated)
	if err != nil {
		return Machine{}, err
	}
	m.InternetOnline = internet == 1
	if lastSeen.Valid && lastSeen.String != "" {
		t := parseDBTime(lastSeen.String)
		m.LastSeenAt = &t
	}
	m.CreatedAt = parseDBTime(created)
	m.UpdatedAt = parseDBTime(updated)
	return m, nil
}

func scanMachineWithMetric(row rowScanner) (Machine, *Metric, error) {
	var m Machine
	var internet int
	var lastSeen sql.NullString
	var created, updated string
	var metric Metric
	var metricID, metricMachineID, ramTotal, ramUsed, uptime sql.NullInt64
	var metricCollected, metricCreated sql.NullString
	var cpu, ramPercent sql.NullFloat64
	var metricInternet sql.NullInt64
	err := row.Scan(&m.ID, &m.ClientID, &m.ClientName, &m.Name, &m.Hostname, &m.TokenHint,
		&m.AgentVersion, &m.OSName, &m.IPAddress, &internet, &lastSeen, &created, &updated,
		&metricID, &metricMachineID, &metricCollected, &cpu, &ramTotal, &ramUsed,
		&ramPercent, &metricInternet, &uptime, &metricCreated)
	if err != nil {
		return Machine{}, nil, err
	}
	m.InternetOnline = internet == 1
	if lastSeen.Valid && lastSeen.String != "" {
		t := parseDBTime(lastSeen.String)
		m.LastSeenAt = &t
	}
	m.CreatedAt = parseDBTime(created)
	m.UpdatedAt = parseDBTime(updated)
	if !metricID.Valid {
		return m, nil, nil
	}
	metric.ID = metricID.Int64
	metric.MachineID = metricMachineID.Int64
	metric.CollectedAt = parseDBTime(metricCollected.String)
	metric.CPUPercent = cpu.Float64
	metric.RAMTotalBytes = ramTotal.Int64
	metric.RAMUsedBytes = ramUsed.Int64
	metric.RAMPercent = ramPercent.Float64
	metric.InternetOnline = metricInternet.Int64 == 1
	metric.UptimeSeconds = uptime.Int64
	metric.CreatedAt = parseDBTime(metricCreated.String)
	return m, &metric, nil
}

func scanMetric(row rowScanner) (Metric, error) {
	var metric Metric
	var collected, created string
	var internet int
	err := row.Scan(&metric.ID, &metric.MachineID, &collected, &metric.CPUPercent, &metric.RAMTotalBytes,
		&metric.RAMUsedBytes, &metric.RAMPercent, &internet, &metric.UptimeSeconds, &created)
	if err != nil {
		return Metric{}, err
	}
	metric.CollectedAt = parseDBTime(collected)
	metric.InternetOnline = internet == 1
	metric.CreatedAt = parseDBTime(created)
	return metric, nil
}

func buildAlerts(machine MachineOverview) []string {
	var alerts []string
	if !machine.Online {
		alerts = append(alerts, "offline")
	}
	if machine.LastMetric != nil {
		if machine.LastMetric.CPUPercent >= 85 {
			alerts = append(alerts, "CPU alta")
		}
		if machine.LastMetric.RAMPercent >= 90 {
			alerts = append(alerts, "RAM alta")
		}
		if !machine.LastMetric.InternetOnline {
			alerts = append(alerts, "internet")
		}
	}
	for _, disk := range machine.Disks {
		if disk.UsedPercent >= 90 {
			alerts = append(alerts, "disco cheio")
			break
		}
	}
	return alerts
}

func alertLevel(alerts []string) string {
	if len(alerts) == 0 {
		return "ok"
	}
	for _, alert := range alerts {
		if alert == "offline" || alert == "disco cheio" {
			return "critical"
		}
	}
	return "warning"
}

func parseAgentTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Now().UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC()
	}
	return time.Now().UTC()
}

func parseDBTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "infobh_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func tokenHint(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 8 {
		return token
	}
	return token[:10] + "..." + token[len(token)-6:]
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeDeviceCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "usb":
		return "USB"
	case "printer", "impressora", "impressoras":
		return "Impressora"
	case "storage", "armazenamento", "disk":
		return "Armazenamento"
	default:
		return strings.TrimSpace(value)
	}
}
