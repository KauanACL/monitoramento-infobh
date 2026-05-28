package server

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed web/templates/*.html web/static/*
var webFS embed.FS

type App struct {
	store *Store
	cfg   Config
	log   *slog.Logger
}

func NewApp(store *Store, cfg Config, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.Default()
	}
	if strings.TrimSpace(cfg.ActionPIN) == "" {
		cfg.ActionPIN = DefaultActionPIN
	}
	return &App{store: store, cfg: cfg, log: logger}
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	staticFS, _ := fs.Sub(webFS, "web/static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /", a.handleDashboard)
	mux.HandleFunc("GET /clients", a.handleClients)
	mux.HandleFunc("POST /clients", a.handleCreateClient)
	mux.HandleFunc("GET /machines", a.handleMachines)
	mux.HandleFunc("POST /machines", a.handleCreateMachine)
	mux.HandleFunc("GET /machines/{id}", a.handleMachineDetail)
	mux.HandleFunc("GET /api/machines/{id}/live", a.handleMachineLive)
	mux.HandleFunc("POST /machines/{id}/commands/cache-clean", a.handleCacheCleanCommand)
	mux.HandleFunc("POST /machines/{id}/rotate-token", a.handleRotateToken)
	mux.HandleFunc("GET /settings/alerts", a.handleAlertSettings)
	mux.HandleFunc("POST /settings/alerts", a.handleSaveAlertSettings)

	mux.HandleFunc("POST /api/agent/heartbeat", a.handleAgentHeartbeat)
	mux.HandleFunc("POST /api/agent/metrics", a.handleAgentMetrics)
	mux.HandleFunc("POST /api/agent/devices", a.handleAgentDevices)
	mux.HandleFunc("POST /api/agent/hardware", a.handleAgentHardware)
	mux.HandleFunc("POST /api/agent/temperatures", a.handleAgentTemperatures)
	mux.HandleFunc("POST /api/agent/commands/poll", a.handleAgentCommandPoll)
	mux.HandleFunc("POST /api/agent/commands/{id}/result", a.handleAgentCommandResult)
	mux.HandleFunc("GET /healthz", a.handleHealth)

	return a.securityHeaders(a.requestLog(mux))
}

func (a *App) StartRetentionJob(ctx context.Context) {
	go func() {
		if err := a.store.Cleanup(ctx, a.cfg.RetentionDays); err != nil {
			a.log.Warn("cleanup failed", "error", err)
		}
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.store.Cleanup(ctx, a.cfg.RetentionDays); err != nil {
					a.log.Warn("cleanup failed", "error", err)
				}
			}
		}
	}()
}

func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := a.store.Dashboard(r.Context())
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "dashboard.html", data)
}

func (a *App) handleClients(w http.ResponseWriter, r *http.Request) {
	clients, err := a.store.ListClients(r.Context())
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "clients.html", map[string]any{
		"Clients": clients,
		"Flash":   r.URL.Query().Get("flash"),
	})
}

func (a *App) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.badRequest(w, "Formulario invalido")
		return
	}
	_, err := a.store.CreateClient(r.Context(), r.FormValue("name"), r.FormValue("contact"), r.FormValue("notes"))
	if err != nil {
		a.render(w, "clients.html", map[string]any{
			"Clients": []ClientOverview{},
			"Error":   "Nao foi possivel criar o cliente: " + err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/clients?flash=Cliente criado", http.StatusSeeOther)
}

func (a *App) handleMachines(w http.ResponseWriter, r *http.Request) {
	clientID, _ := strconv.ParseInt(r.URL.Query().Get("client_id"), 10, 64)
	clients, err := a.store.ListClients(r.Context())
	if err != nil {
		a.serverError(w, err)
		return
	}
	machines, err := a.store.ListMachines(r.Context(), clientID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "machines.html", map[string]any{
		"Clients":        clients,
		"Machines":       machines,
		"SelectedClient": clientID,
		"Flash":          r.URL.Query().Get("flash"),
	})
}

func (a *App) handleCreateMachine(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.badRequest(w, "Formulario invalido")
		return
	}
	clientID, _ := strconv.ParseInt(r.FormValue("client_id"), 10, 64)
	machine, token, err := a.store.CreateMachine(r.Context(), clientID, r.FormValue("name"))
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "token.html", map[string]any{
		"Machine": machine,
		"Token":   token,
		"Server":  publicServerURL(r),
	})
}

func (a *App) handleMachineDetail(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	detail, err := a.store.MachineDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		a.serverError(w, err)
		return
	}
	detail.ActionPINEnabled = a.actionPINEnabled()
	detail.Flash = r.URL.Query().Get("flash")
	detail.Error = r.URL.Query().Get("error")
	a.render(w, "machine_detail.html", detail)
}

func (a *App) handleMachineLive(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	detail, err := a.store.MachineDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		a.serverError(w, err)
		return
	}
	detail.ActionPINEnabled = a.actionPINEnabled()
	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339Nano),
		"detail":       detail,
	})
}

func (a *App) handleCacheCleanCommand(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.badRequest(w, "Formulario invalido")
		return
	}
	if !a.checkActionPIN(w, r) {
		return
	}
	if _, err := a.store.GetMachine(r.Context(), id); err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := a.store.CreateCacheCleanCommand(r.Context(), id); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/machines/%d?flash=Limpeza+solicitada", id), http.StatusSeeOther)
}

func (a *App) handleAlertSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.store.AlertSettings(r.Context())
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "settings_alerts.html", map[string]any{
		"Settings":         settings,
		"ActionPINEnabled": a.actionPINEnabled(),
		"Flash":            r.URL.Query().Get("flash"),
	})
}

func (a *App) handleSaveAlertSettings(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.badRequest(w, "Formulario invalido")
		return
	}
	if !a.checkActionPIN(w, r) {
		return
	}
	settings := AlertSettings{
		CPUPercent:     parseFormPercent(r, "cpu_percent", 85),
		RAMPercent:     parseFormPercent(r, "ram_percent", 85),
		StoragePercent: parseFormPercent(r, "storage_percent", 90),
	}
	if _, err := a.store.SaveAlertSettings(r.Context(), settings); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/settings/alerts?flash=Alertas+salvos", http.StatusSeeOther)
}

func (a *App) handleRotateToken(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	machine, err := a.store.GetMachine(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	token, err := a.store.RotateMachineToken(r.Context(), id)
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, "token.html", map[string]any{
		"Machine": machine,
		"Token":   token,
		"Server":  publicServerURL(r),
		"Rotated": true,
	})
}

func (a *App) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	machine, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var payload AgentHeartbeat
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := a.store.RecordHeartbeat(r.Context(), machine.ID, payload); err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "machine_id": machine.ID})
}

func (a *App) handleAgentMetrics(w http.ResponseWriter, r *http.Request) {
	machine, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var payload AgentMetricPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := a.store.RecordMetrics(r.Context(), machine.ID, payload); err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "machine_id": machine.ID})
}

func (a *App) handleAgentDevices(w http.ResponseWriter, r *http.Request) {
	machine, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var payload AgentDevicePayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := a.store.RecordDevices(r.Context(), machine.ID, payload); err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "machine_id": machine.ID})
}

func (a *App) handleAgentHardware(w http.ResponseWriter, r *http.Request) {
	machine, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var payload AgentHardwarePayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := a.store.RecordHardware(r.Context(), machine.ID, payload); err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "machine_id": machine.ID})
}

func (a *App) handleAgentTemperatures(w http.ResponseWriter, r *http.Request) {
	machine, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var payload AgentTemperaturePayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := a.store.RecordTemperatures(r.Context(), machine.ID, payload); err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "machine_id": machine.ID})
}

func (a *App) handleAgentCommandPoll(w http.ResponseWriter, r *http.Request) {
	machine, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	command, err := a.store.ClaimNextCommand(r.Context(), machine.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if command == nil {
		writeJSON(w, http.StatusOK, AgentCommandPollResponse{OK: true})
		return
	}
	payload := json.RawMessage(command.RequestPayload)
	if len(payload) == 0 || !json.Valid(payload) {
		payload = json.RawMessage(`{}`)
	}
	writeJSON(w, http.StatusOK, AgentCommandPollResponse{
		OK: true,
		Command: &AgentCommandPayload{
			ID:        command.ID,
			Type:      command.Type,
			Payload:   payload,
			CreatedAt: command.CreatedAt.Format(time.RFC3339Nano),
		},
	})
}

func (a *App) handleAgentCommandResult(w http.ResponseWriter, r *http.Request) {
	machine, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	commandID, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var payload AgentCommandResultPayload
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := a.store.RecordCommandResult(r.Context(), machine.ID, commandID, payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "machine_id": machine.ID})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) authenticateAgent(w http.ResponseWriter, r *http.Request) (Machine, bool) {
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.Header.Get("X-Agent-Token")
	}
	machine, err := a.store.AuthenticateAgent(r.Context(), token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "token invalido"})
		return Machine{}, false
	}
	return machine, true
}

func (a *App) render(w http.ResponseWriter, page string, data any) {
	tmpl, err := template.New("").Funcs(templateFuncs()).ParseFS(webFS, "web/templates/base.html", "web/templates/"+page)
	if err != nil {
		a.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		a.log.Error("template render failed", "error", err)
	}
}

func (a *App) serverError(w http.ResponseWriter, err error) {
	a.log.Error("request failed", "error", err)
	http.Error(w, "Erro interno", http.StatusInternalServerError)
}

func (a *App) badRequest(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

func (a *App) actionPINEnabled() bool {
	return strings.TrimSpace(a.cfg.ActionPIN) != ""
}

func (a *App) checkActionPIN(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(a.cfg.ActionPIN)
	if expected == "" {
		http.Error(w, "PIN de acao indisponivel", http.StatusForbidden)
		return false
	}
	pin := strings.TrimSpace(r.FormValue("pin"))
	if pin == "" {
		pin = strings.TrimSpace(r.Header.Get("X-Action-PIN"))
	}
	if subtle.ConstantTimeCompare([]byte(pin), []byte(expected)) != 1 {
		http.Error(w, "PIN invalido", http.StatusForbidden)
		return false
	}
	return true
}

func (a *App) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		a.log.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dest); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "json invalido"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func pathID(r *http.Request) (int64, error) {
	value := r.PathValue("id")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func publicServerURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	return scheme + "://" + r.Host
}

func parseFormPercent(r *http.Request, key string, fallback float64) float64 {
	value := strings.ReplaceAll(strings.TrimSpace(r.FormValue(key)), ",", ".")
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return normalizeThreshold(parsed, fallback)
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"bytes":         formatBytes,
		"pct":           formatPercent,
		"pctValue":      formatPercentValue,
		"temp":          formatTemp,
		"timeAgo":       timeAgo,
		"clock":         clock,
		"statusText":    statusText,
		"join":          strings.Join,
		"historyCPU":    historyValues(func(m Metric) float64 { return m.CPUPercent }),
		"historyRAM":    historyValues(func(m Metric) float64 { return m.RAMPercent }),
		"categoryIcon":  categoryIcon,
		"slotLabel":     slotLabel,
		"clockMHz":      clockMHz,
		"tempComponent": temperatureComponent,
		"tempSensor":    temperatureSensor,
		"safeID":        safeID,
		"clientPercent": clientPercent,
	}
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(value)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", f, units[i])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.0f%%", value)
}

func formatPercentValue(value float64) string {
	return fmt.Sprintf("%.0f", value)
}

func formatTemp(value float64) string {
	return fmt.Sprintf("%.1f C", value)
}

func clockMHz(value int) string {
	if value <= 0 {
		return "clock indisponivel"
	}
	return fmt.Sprintf("%d MHz", value)
}

func timeAgo(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "nunca"
	}
	d := time.Since(t.UTC())
	if d < time.Minute {
		return "agora"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func clock(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("02/01 15:04")
}

func statusText(online bool) string {
	if online {
		return "Online"
	}
	return "Offline"
}

func categoryIcon(category string) string {
	switch strings.ToLower(category) {
	case "usb":
		return "usb"
	case "impressora":
		return "printer"
	case "armazenamento":
		return "drive"
	default:
		return "device"
	}
}

func slotLabel(index int) string {
	return fmt.Sprintf("Slot %d", index+1)
}

func temperatureComponent(index int, name, sensorType string) string {
	normalized := strings.ToLower(name + " " + sensorType)
	switch {
	case strings.Contains(normalized, "cpu"), strings.Contains(normalized, "processor"), strings.Contains(normalized, "package"):
		return "CPU"
	case strings.Contains(normalized, "gpu"), strings.Contains(normalized, "video"):
		return "GPU"
	case strings.Contains(normalized, "disk"), strings.Contains(normalized, "ssd"), strings.Contains(normalized, "hdd"), strings.Contains(normalized, "nvme"):
		return "Armazenamento"
	case strings.Contains(normalized, "acpi"), strings.Contains(normalized, "thermalzone"), strings.Contains(normalized, "thermal_zone"), strings.Contains(normalized, "tz"):
		return fmt.Sprintf("Placa-mãe / ACPI %d", index+1)
	default:
		clean := cleanTemperatureName(name)
		if clean == "" {
			return fmt.Sprintf("Sensor %d", index+1)
		}
		return clean
	}
}

func temperatureSensor(name, source string) string {
	clean := cleanTemperatureName(name)
	switch {
	case clean != "" && source != "":
		return fmt.Sprintf("Sensor %s · %s", clean, source)
	case clean != "":
		return fmt.Sprintf("Sensor %s", clean)
	case source != "":
		return source
	default:
		return "Sensor nativo do Windows"
	}
}

func cleanTemperatureName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '\\' || r == '/' || r == ':'
	})
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part != "" {
			return part
		}
	}
	return name
}

func safeID(value string) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-").Replace(value)
	return value
}

func clientPercent(online, total int) string {
	if total <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", (float64(online)/float64(total))*100)
}

func historyValues(pick func(Metric) float64) func([]Metric) string {
	return func(metrics []Metric) string {
		values := make([]string, 0, len(metrics))
		for _, metric := range metrics {
			values = append(values, fmt.Sprintf("%.1f", pick(metric)))
		}
		return strings.Join(values, ",")
	}
}
