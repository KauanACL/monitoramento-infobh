function drawSparkline(canvas) {
  const values = (canvas.dataset.values || "")
    .split(",")
    .map((value) => Number.parseFloat(value))
    .filter((value) => Number.isFinite(value));

  const ratio = window.devicePixelRatio || 1;
  const width = canvas.clientWidth;
  const height = canvas.clientHeight;
  canvas.width = Math.max(1, Math.floor(width * ratio));
  canvas.height = Math.max(1, Math.floor(height * ratio));

  const ctx = canvas.getContext("2d");
  ctx.scale(ratio, ratio);
  ctx.clearRect(0, 0, width, height);

  ctx.fillStyle = "#fbfcfd";
  ctx.fillRect(0, 0, width, height);

  ctx.strokeStyle = "#dfe4e8";
  ctx.lineWidth = 1;
  for (let i = 1; i <= 4; i += 1) {
    const y = (height / 5) * i;
    ctx.beginPath();
    ctx.moveTo(12, y);
    ctx.lineTo(width - 12, y);
    ctx.stroke();
  }

  ctx.fillStyle = "#68707a";
  ctx.font = "12px Inter, system-ui, sans-serif";
  ctx.fillText(canvas.dataset.label || "Uso", 14, 22);

  if (values.length === 0) {
    ctx.fillStyle = "#68707a";
    ctx.fillText("Sem dados", 14, height / 2);
    return;
  }

  const pad = 16;
  const plotW = width - pad * 2;
  const plotH = height - pad * 2;
  const step = values.length > 1 ? plotW / (values.length - 1) : plotW;

  ctx.beginPath();
  values.forEach((value, index) => {
    const x = pad + index * step;
    const y = pad + plotH - (Math.max(0, Math.min(100, value)) / 100) * plotH;
    if (index === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  });
  ctx.strokeStyle = canvas.classList.contains("ram") ? "#c77c02" : "#0f8b8d";
  ctx.lineWidth = 2.5;
  ctx.lineJoin = "round";
  ctx.lineCap = "round";
  ctx.stroke();

  if (values.length === 1) {
    const y = pad + plotH - (Math.max(0, Math.min(100, values[0])) / 100) * plotH;
    ctx.beginPath();
    ctx.arc(width / 2, y, 4, 0, Math.PI * 2);
    ctx.fillStyle = ctx.strokeStyle;
    ctx.fill();
  }
}

document.querySelectorAll(".sparkline").forEach(drawSparkline);

window.addEventListener("resize", () => {
  document.querySelectorAll(".sparkline").forEach(drawSparkline);
});

document.querySelectorAll(".copyable").forEach((item) => {
  item.addEventListener("click", async () => {
    const text = item.dataset.copy || item.textContent || "";
    try {
      await navigator.clipboard.writeText(text.trim());
      item.dataset.copied = "true";
      setTimeout(() => {
        delete item.dataset.copied;
      }, 1400);
    } catch {
      // Clipboard can be unavailable over plain HTTP on some browsers.
    }
  });
});

const bytesUnits = ["B", "KB", "MB", "GB", "TB"];

function formatBytes(value) {
  let current = Number(value || 0);
  let index = 0;
  while (current >= 1024 && index < bytesUnits.length - 1) {
    current /= 1024;
    index += 1;
  }
  return `${index === 0 ? current.toFixed(0) : current.toFixed(1)} ${bytesUnits[index]}`;
}

function formatPercent(value) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? `${Math.round(parsed)}%` : "-";
}

function formatTemp(value) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? `${parsed.toFixed(1)} C` : "-";
}

function formatMHz(value) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? `${Math.round(parsed)} MHz` : "";
}

function timeAgo(value) {
  if (!value) return "nunca";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "nunca";
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
  if (seconds < 60) return "agora";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

function clock(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function iconClass(category) {
  const normalized = String(category || "").toLowerCase();
  if (normalized === "usb") return "usb";
  if (normalized === "impressora") return "printer";
  if (normalized === "armazenamento") return "drive";
  return "device";
}

function ramSlotName(index) {
  return `Slot ${index + 1}`;
}

function cleanTemperatureName(name) {
  const parts = String(name || "").split(/[\\/:]/).map((item) => item.trim()).filter(Boolean);
  return parts.length ? parts[parts.length - 1] : "";
}

function temperatureComponent(reading, index) {
  const normalized = `${reading?.Name || ""} ${reading?.SensorType || ""}`.toLowerCase();
  if (normalized.includes("cpu") || normalized.includes("processor") || normalized.includes("package")) return "CPU";
  if (normalized.includes("gpu") || normalized.includes("video")) return "GPU";
  if (normalized.includes("disk") || normalized.includes("ssd") || normalized.includes("hdd") || normalized.includes("nvme")) return "Armazenamento";
  if (normalized.includes("acpi") || normalized.includes("thermalzone") || normalized.includes("thermal_zone") || normalized.includes("tz")) {
    return `Placa-mãe / ACPI ${index + 1}`;
  }
  return cleanTemperatureName(reading?.Name) || `Sensor ${index + 1}`;
}

function temperatureSensor(reading) {
  const clean = cleanTemperatureName(reading?.Name);
  const source = reading?.Source || "";
  if (clean && source) return `Sensor ${clean} · ${source}`;
  if (clean) return `Sensor ${clean}`;
  return source || "Sensor nativo do Windows";
}

function setText(root, selector, value) {
  const item = root.querySelector(selector);
  if (item) item.textContent = value;
}

function setTile(root, name, state) {
  const tile = root.querySelector(`[data-tile="${name}"]`);
  if (!tile) return;
  tile.classList.remove("good", "warn", "danger");
  if (state) tile.classList.add(state);
}

function setStatusPill(root, online) {
  const pill = root.querySelector("[data-status-pill]");
  if (!pill) return;
  pill.classList.toggle("online", online);
  pill.classList.toggle("offline", !online);
}

function setBar(root, name, value) {
  const bar = root.querySelector(`[data-live-bar="${name}"]`);
  if (!bar) return;
  bar.style.width = formatPercent(value);
}

function renderAlerts(root, alerts) {
  const target = root.querySelector('[data-live="alerts"]');
  if (!target) return;
  if (!alerts || alerts.length === 0) {
    target.innerHTML = '<span class="ok">OK</span>';
    return;
  }
  target.innerHTML = alerts.map((alert) => `<span>${escapeHTML(alert)}</span>`).join("");
}

function renderDisks(root, disks) {
  const target = root.querySelector('[data-live-list="disks"]');
  if (!target) return;
  if (!disks || disks.length === 0) {
    target.innerHTML = '<div class="empty-line">Sem dados de disco.</div>';
    return;
  }
  target.innerHTML = disks.map((disk) => {
    const used = formatPercent(disk.UsedPercent);
    return `
      <div class="disk-row">
        <div>
          <strong>${escapeHTML(disk.Name)}</strong>
          <small>${escapeHTML(disk.MountPoint)} · ${escapeHTML(disk.FileSystem)} · ${formatBytes(disk.UsedBytes)} usados de ${formatBytes(disk.TotalBytes)} · ${formatBytes(disk.FreeBytes)} livres</small>
        </div>
        <span>${used}</span>
        <div class="bar"><i style="width: ${used}"></i></div>
      </div>`;
  }).join("");
}

function renderHardware(root, hardware) {
  const target = root.querySelector('[data-live-list="hardware"]');
  if (!target) return;
  if (!hardware) {
    target.innerHTML = '<div class="empty-line">Sem inventário de hardware.</div>';
    return;
  }
  const modules = hardware.RAMModules || [];
  const ram = modules.length ? modules.map((module, index) => {
    const speed = formatMHz(module.speed_mhz);
    const summary = [formatBytes(module.capacity_bytes), module.memory_type || "tipo indisponível", speed].filter(Boolean).join(" · ");
    const details = [module.manufacturer || "fabricante indisponível", module.part_number, speed ? "" : "velocidade não informada"].filter(Boolean).join(" · ");
    return `
      <div class="ram-module">
        <strong>${ramSlotName(index)}</strong>
        <div>
          <span>${escapeHTML(summary)}</span>
          <small>${escapeHTML(details)}</small>
        </div>
      </div>`;
  }).join("") : '<div class="empty-line">Sem dados dos slots de RAM.</div>';
  target.innerHTML = `
    <div class="hardware-block">
      <div class="hardware-summary">
        <div>
          <span>CPU</span>
          <strong>${escapeHTML(hardware.CPUName || "-")}</strong>
          <small>${escapeHTML(hardware.CPUManufacturer || "-")} · ${escapeHTML(hardware.CPUCores || "-")} núcleos · ${escapeHTML(hardware.CPULogicalProcessors || "-")} threads · ${escapeHTML(hardware.CPUMaxClockMHz || "-")} MHz</small>
        </div>
        <div>
          <span>Equipamento</span>
          <strong>${escapeHTML(hardware.SystemModel || "-")}</strong>
          <small>${escapeHTML(hardware.SystemManufacturer || "-")} · ${escapeHTML(hardware.BaseboardProduct || "-")} · BIOS ${escapeHTML(hardware.BIOSVersion || "-")}</small>
        </div>
      </div>
      <h3>Memória instalada</h3>
      <div class="ram-module-list">${ram}</div>
    </div>`;
}

function renderTemperatures(root, temperatures) {
  const target = root.querySelector('[data-live-list="temperatures"]');
  if (!target) return;
  if (!temperatures || !temperatures.Available || !temperatures.Readings || temperatures.Readings.length === 0) {
    target.innerHTML = `<div class="empty-line">${escapeHTML((temperatures && temperatures.Message) || "Aguardando coleta de temperatura")}</div>`;
    return;
  }
  target.innerHTML = `<div class="temperature-grid">${temperatures.Readings.map((reading, index) => `
    <div class="temperature-card">
      <span>${escapeHTML(temperatureComponent(reading, index))}</span>
      <strong>${formatTemp(reading.CurrentCelsius)}</strong>
      <small>${escapeHTML(temperatureSensor(reading))}</small>
    </div>`).join("")}</div>`;
}

function renderDevices(root, devices) {
  const target = root.querySelector('[data-live-list="devices"]');
  if (!target) return;
  if (!devices || devices.length === 0) {
    target.innerHTML = '<div class="empty-line">Nenhum dispositivo registrado.</div>';
    return;
  }
  target.innerHTML = devices.map((device) => `
    <article class="device-card ${device.Connected ? "" : "muted"}" data-device-category="${escapeHTML(device.Category)}">
      <span class="device-icon ${iconClass(device.Category)}"></span>
      <div>
        <strong>${escapeHTML(device.Name)}</strong>
        <small>${escapeHTML(device.Category)} · ${device.Connected ? "conectado" : "desconectado"} · ${escapeHTML(device.Status)} · visto ${clock(device.LastSeen)}</small>
      </div>
    </article>`).join("");
  applyDeviceFilter(root);
}

function renderCommands(root, commands) {
  const target = root.querySelector('[data-live-list="commands"]');
  if (!target) return;
  if (!commands || commands.length === 0) {
    target.innerHTML = '<div class="empty-line">Sem comandos recentes.</div>';
    return;
  }
  target.innerHTML = commands.map((command) => `
    <div class="command-row ${escapeHTML(command.Status)}">
      <strong>${escapeHTML(command.Type)}</strong>
      <span>${escapeHTML(command.Status)}</span>
      <small>${clock(command.CreatedAt)}${command.ResultMessage ? ` · ${escapeHTML(command.ResultMessage)}` : ""}${command.ErrorMessage ? ` · ${escapeHTML(command.ErrorMessage)}` : ""}</small>
    </div>`).join("");
}

function renderEvents(root, events) {
  const target = root.querySelector('[data-live-list="events"]');
  if (!target) return;
  if (!events || events.length === 0) {
    target.innerHTML = '<div class="empty-line">Sem eventos recentes.</div>';
    return;
  }
  target.innerHTML = events.map((event) => `
    <div class="event-row">
      <span class="event-dot ${escapeHTML(event.Severity)}"></span>
      <strong>${escapeHTML(event.Message)}</strong>
      <small>${clock(event.CreatedAt)}</small>
    </div>`).join("");
}

function updateHistory(root, history) {
  const cpu = root.querySelector('[data-chart="cpu"]');
  const ram = root.querySelector('[data-chart="ram"]');
  const cpuValues = (history || []).map((metric) => Number(metric.CPUPercent || 0).toFixed(1)).join(",");
  const ramValues = (history || []).map((metric) => Number(metric.RAMPercent || 0).toFixed(1)).join(",");
  if (cpu) {
    cpu.dataset.values = cpuValues;
    drawSparkline(cpu);
  }
  if (ram) {
    ram.dataset.values = ramValues;
    drawSparkline(ram);
  }
  setText(root, '[data-live="history-updated"]', "ao vivo");
}

function applyDeviceFilter(root) {
  const machineID = root.dataset.machineId;
  const selected = localStorage.getItem(`machine:${machineID}:device-filter`) || "all";
  root.querySelectorAll("[data-device-filter]").forEach((button) => {
    button.classList.toggle("active", button.dataset.deviceFilter === selected);
  });
  root.querySelectorAll("[data-device-category]").forEach((card) => {
    const category = card.dataset.deviceCategory || "";
    const known = ["USB", "Impressora", "Armazenamento"].includes(category);
    const visible = selected === "all" || category === selected || (selected === "other" && !known);
    card.hidden = !visible;
  });
}

function initDeviceFilters(root) {
  root.querySelectorAll("[data-device-filter]").forEach((button) => {
    button.addEventListener("click", () => {
      localStorage.setItem(`machine:${root.dataset.machineId}:device-filter`, button.dataset.deviceFilter);
      applyDeviceFilter(root);
    });
  });
  applyDeviceFilter(root);
}

function updateMachineDetail(root, detail) {
  const metric = detail.LastMetric;
  setText(root, '[data-live="client-host"]', `${detail.Client.Name} · ${detail.Hostname || "aguardando primeiro heartbeat"}`);
  setText(root, '[data-live="status"]', detail.Online ? "Online" : "Offline");
  setText(root, '[data-live="ip"]', detail.IPAddress || "-");
  setText(root, '[data-live="os"]', detail.OSName || "-");
  setText(root, '[data-live="agent"]', detail.AgentVersion || "-");
  setText(root, '[data-live="cpu"]', metric ? formatPercent(metric.CPUPercent) : "-");
  setText(root, '[data-live="ram"]', metric ? formatPercent(metric.RAMPercent) : "-");
  setText(root, '[data-live="internet"]', metric && metric.InternetOnline ? "OK" : "Falha");
  setText(root, '[data-live="connection-note"]', metric && metric.InternetOnline ? "Conectividade validada" : "Sem conectividade reportada");
  setText(root, '[data-live="last-seen"]', timeAgo(detail.LastSeenAt));
  setStatusPill(root, detail.Online);
  setBar(root, "cpu", metric ? metric.CPUPercent : 0);
  setBar(root, "ram", metric ? metric.RAMPercent : 0);
  setTile(root, "status", detail.Online ? "good" : "danger");
  setTile(root, "cpu", metric && detail.Settings && metric.CPUPercent >= detail.Settings.CPUPercent ? "warn" : "");
  setTile(root, "ram", metric && detail.Settings && metric.RAMPercent >= detail.Settings.RAMPercent ? "warn" : "");
  setTile(root, "internet", metric && metric.InternetOnline ? "good" : "warn");
  renderAlerts(root, detail.Alerts);
  renderDisks(root, detail.Disks);
  renderHardware(root, detail.Hardware);
  renderTemperatures(root, detail.Temperatures);
  renderDevices(root, detail.Devices);
  renderCommands(root, detail.Commands);
  renderEvents(root, detail.Events);
  updateHistory(root, detail.History);
}

function initMachineLive() {
  const root = document.querySelector("[data-machine-detail]");
  if (!root) return;
  initDeviceFilters(root);
  const machineID = root.dataset.machineId;
  const load = async () => {
    try {
      const response = await fetch(`/api/machines/${machineID}/live`, { cache: "no-store" });
      if (!response.ok) return;
      const data = await response.json();
      updateMachineDetail(root, data.detail);
    } catch {
      // The next polling cycle will try again.
    }
  };
  setInterval(load, 5000);
}

initMachineLive();

if (location.pathname === "/") {
  setTimeout(() => {
    location.reload();
  }, 60000);
}
