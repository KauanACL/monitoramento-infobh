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
  const ram = modules.length ? modules.map((module) => `
    <div class="ram-module">
      <strong>${escapeHTML(module.slot || module.bank_label || "-")}</strong>
      <small>${formatBytes(module.capacity_bytes)} · ${escapeHTML(module.memory_type || "-")} · ${escapeHTML(module.speed_mhz || "-")} MHz · ${escapeHTML(module.manufacturer || "-")} · ${escapeHTML(module.part_number || "-")}</small>
    </div>`).join("") : '<div class="empty-line">Sem dados dos slots de RAM.</div>';
  target.innerHTML = `
    <div class="hardware-block">
      <h3>CPU</h3>
      <dl>
        <div><dt>Modelo</dt><dd>${escapeHTML(hardware.CPUName || "-")}</dd></div>
        <div><dt>Fabricante</dt><dd>${escapeHTML(hardware.CPUManufacturer || "-")}</dd></div>
        <div><dt>Núcleos</dt><dd>${escapeHTML(hardware.CPUCores || "-")}</dd></div>
        <div><dt>Threads</dt><dd>${escapeHTML(hardware.CPULogicalProcessors || "-")}</dd></div>
        <div><dt>Clock</dt><dd>${escapeHTML(hardware.CPUMaxClockMHz || "-")} MHz</dd></div>
      </dl>
      <h3>RAM</h3>
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
  target.innerHTML = `<div class="temperature-grid">${temperatures.Readings.map((reading) => `
    <div class="temperature-card">
      <span>${escapeHTML(reading.Name)}</span>
      <strong>${formatTemp(reading.CurrentCelsius)}</strong>
      <small>${escapeHTML(reading.Source)}</small>
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
  setText(root, '[data-live="history-updated"]', `atualizado ${clock(new Date().toISOString())}`);
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
  setText(root, '[data-live="cpu"]', metric ? formatPercent(metric.CPUPercent) : "-");
  setText(root, '[data-live="ram"]', metric ? formatPercent(metric.RAMPercent) : "-");
  setText(root, '[data-live="internet"]', metric && metric.InternetOnline ? "OK" : "Falha");
  setText(root, '[data-live="last-seen"]', timeAgo(detail.LastSeenAt));
  setTile(root, "status", detail.Online ? "good" : "danger");
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
