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

if (location.pathname === "/") {
  setTimeout(() => {
    location.reload();
  }, 60000);
}
