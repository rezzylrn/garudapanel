<?php
// Server — Console page (standalone, multi-page)
require_once __DIR__ . '/server_layout.php';
$id = (int)($_GET['id'] ?? 0);
if ($id <= 0) { die('Invalid server'); }
$s = gp_load_server($id);
gp_server_head($s, $id, 'console');

$tok = gp_api('POST', "/api/servers/$id/console-token", null, gp_token());
$wsToken = $tok['data']['token'] ?? '';
// WS base: URL publik tunnel Go panel (ditulis keepalive-ws.sh tiap reconnect)
// — browser di luar VM tidak bisa mencapai 127.0.0.1:8080. Fallback: host:port API lokal.
$wsBaseFile = trim(@file_get_contents(__DIR__ . '/../ws_url.txt') ?: '');
if ($wsBaseFile !== '') {
    $wsBase = $wsBaseFile; // https://....serveousercontent.com
    $wsProto = 'wss:';
} else {
    $wsBase = parse_url(GP_API_URL, PHP_URL_HOST) . ':' . (parse_url(GP_API_URL, PHP_URL_PORT) ?: 80);
    $wsProto = (parse_url(GP_API_URL, PHP_URL_SCHEME) === 'https') ? 'wss:' : 'ws:';
}
$st = $s['status'] ?? 'offline';
$addr = ($s['ip'] ?? '') !== '' ? e($s['ip']) . ':' . (int)($s['port'] ?? 0) : '—';
$diskLimitMB = (int)($s['disk_mb'] ?? 0);
$memLimitMB = (int)($s['memory_mb'] ?? 0);
?>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm@5.3.0/css/xterm.min.css">
<div class="stat-strip" aria-label="Server statistics">
  <div class="stat mini">
    <span class="label"><?= icon('nodes', 14) ?> Address</span>
    <span class="value mono" id="st-address" style="font-size:14px"><?= $addr ?></span>
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('clock', 14) ?> Uptime</span>
    <span class="value" id="st-uptime">—</span>
    <canvas class="spark" id="spark-uptime" height="36"></canvas>
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('cpu', 14) ?> CPU Load</span>
    <span class="value" id="st-cpu">—</span>
    <canvas class="spark" id="spark-cpu" height="36"></canvas>
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('memory', 14) ?> Memory</span>
    <span class="value" id="st-mem">—</span>
    <canvas class="spark" id="spark-mem" height="36"></canvas>
    <span class="meter"><span id="st-mem-bar" style="width:0%"></span></span>
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('hdd', 14) ?> Disk</span>
    <span class="value" id="st-disk">—</span>
    <canvas class="spark" id="spark-disk" height="36"></canvas>
    <span class="meter"><span id="st-disk-bar" style="width:0%"></span></span>
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('network', 14) ?> Network</span>
    <span class="value mono" id="st-net" style="font-size:13px">—</span>
    <canvas class="spark" id="spark-net" height="36"></canvas>
  </div>
</div>
<div class="console-bar">
  <span class="status-dot <?= $st === 'running' ? 'on' : 'off' ?>" id="conn-dot"></span>
  <span id="conn-status" class="status-line">Connecting…</span>
  <span class="console-bar-spacer"></span>
  <button class="btn small" id="console-clear" type="button" title="Clear console"><?= icon('eraser', 13) ?> Clear</button>
</div>
<div class="card glass console-card">
  <div class="terminal-wrap"><div id="terminal"></div></div>
</div>
<div class="card" style="margin-top:16px">
  <h3><?= icon('network', 15) ?> Network traffic</h3>
  <div class="traffic-legend">
    <span><span class="traffic-dot" style="background:#60a5fa"></span> Download <strong id="traffic-rx">—</strong></span>
    <span><span class="traffic-dot" style="background:#4ade80"></span> Upload <strong id="traffic-tx">—</strong></span>
    <span class="traffic-note">last 3 min · live from server stats</span>
  </div>
  <div class="traffic-wrap"><canvas id="traffic-chart" height="120"></canvas></div>
</div>
<script src="https://cdn.jsdelivr.net/npm/xterm@5.3.0/lib/xterm.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/xterm-addon-fit@5.3.0/lib/xterm-addon-fit.min.js"></script>
<script>
(function() {
  const term = new Terminal({
    cursorBlink: true, fontSize: 13,
    allowTransparency: true,
    theme: { background: 'rgba(0, 0, 0, 0)', foreground: '#e6eaf0', cursor: '#60a5fa' },
  });
  term.open(document.getElementById('terminal'));
  document.getElementById('console-clear').addEventListener('click', function () { term.clear(); });
  // FitAddon (pola Pterodactyl Console.tsx): terminal menyesuaikan lebar
  // container — tanpa ini, xterm 80 kolom default terpotong / menggeser
  // halaman di layar mobile. Fallback: tetap jalan tanpa fit bila CDN gagal.
  let fitAddon = null;
  try {
    const FA = window.FitAddon && (window.FitAddon.FitAddon || window.FitAddon);
    if (FA) { fitAddon = new FA(); term.loadAddon(fitAddon); }
  } catch (e) { fitAddon = null; }
  const statusEl = document.getElementById('conn-status');
  const dotEl = document.getElementById('conn-dot');

  // --- Statistik live (pola ServerDetailsBlock Pterodactyl) ---
  const memLimitMB = <?= $memLimitMB ?>;
  const diskLimitMB = <?= $diskLimitMB ?>;
  function fmtMB(mb) {
    if (mb == null || isNaN(mb)) return '—';
    return mb >= 1024 ? (mb / 1024).toFixed(1) + ' GB' : Math.round(mb) + ' MB';
  }
  function fmtKB(kb) {
    if (kb == null || isNaN(kb)) return '—';
    return kb >= 1024 ? (kb / 1024).toFixed(1) + ' MB' : Math.round(kb) + ' KB';
  }
  function fmtUptime(sec) {
    sec = Math.floor(sec || 0);
    if (sec <= 0) return '—';
    const d = Math.floor(sec / 86400), h = Math.floor(sec % 86400 / 3600),
          m = Math.floor(sec % 3600 / 60), s = sec % 60;
    if (d > 0) return d + 'd ' + h + 'h';
    if (h > 0) return h + 'h ' + m + 'm';
    if (m > 0) return m + 'm ' + s + 's';
    return s + 's';
  }
  function setBar(el, pct) {
    el.style.width = Math.max(0, Math.min(100, pct || 0)) + '%';
  }
  function applyStats(d) {
    if (!d) return;
    document.getElementById('st-uptime').textContent = fmtUptime(d.uptime_seconds);
    if (sparkUptime) sparkUptime.push(d.uptime_seconds || 0);
    document.getElementById('st-cpu').textContent =
      (d.cpu_percent == null ? '—' : d.cpu_percent.toFixed(1) + '%');
    if (sparkCpu) sparkCpu.push(d.cpu_percent || 0);
    const memUsed = d.mem_used_mb || 0, memLimit = d.mem_limit_mb || memLimitMB;
    if (sparkMem) sparkMem.push(memUsed);
    document.getElementById('st-mem').textContent =
      fmtMB(memUsed) + (memLimit ? ' / ' + fmtMB(memLimit) : '');
    setBar(document.getElementById('st-mem-bar'), memLimit ? memUsed / memLimit * 100 : 0);
    const rx = d.net_rx_kb || 0, tx = d.net_tx_kb || 0;
    document.getElementById('st-net').textContent =
      (rx <= 0 && tx <= 0) ? '—' : '↓ ' + fmtKB(rx) + ' · ↑ ' + fmtKB(tx);
    pushTraffic(d);
    if (d.state) {
      dotEl.className = 'status-dot ' + (d.state === 'running' ? 'on' : 'off');
      if (window.__gpSetStatus) window.__gpSetStatus(d.state);
    }
  }
  // --- Sparkline: angka di atas, grafik di bawah (satu per stat) ---
  // Buffer 60 sampel (~2 menit, frame stat tiap 2 detik). Uptime naik terus
  // (garis miring ke atas = visual runtime); CPU skala tetap 0-100;
  // Memory/Disk skala = limit; Network dua garis (RX/TX).
  function makeSpark(id, color, fixedMax) {
    var el = document.getElementById(id);
    if (!el) return null;
    var ctx = el.getContext('2d');
    var data = [];
    function draw() {
      var dpr = window.devicePixelRatio || 1;
      var w = el.clientWidth, h = 36;
      if (!w) return;
      if (el.width !== w * dpr || el.height !== h * dpr) { el.width = w * dpr; el.height = h * dpr; }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);
      var max = fixedMax || 1;
      if (!fixedMax) { data.forEach(function (v) { if (v > max) max = v; }); max *= 1.15; }
      ctx.strokeStyle = 'rgba(255,255,255,0.10)';
      ctx.beginPath(); ctx.moveTo(0, h - 1.5); ctx.lineTo(w, h - 1.5); ctx.stroke();
      if (data.length < 2) return;
      ctx.strokeStyle = color; ctx.lineWidth = 1.5; ctx.beginPath();
      data.forEach(function (v, i) {
        var x = w * i / 59, y = h - 2 - (h - 6) * Math.min(1, v / max);
        if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
      });
      ctx.stroke();
      ctx.lineTo(w, h); ctx.lineTo(0, h); ctx.closePath();
      ctx.fillStyle = color + '22'; ctx.fill();
    }
    return {
      push: function (v) {
        data.push(v == null || isNaN(v) ? 0 : v);
        if (data.length > 60) data.shift();
        draw();
      },
      draw: draw
    };
  }
  function makeSparkDual(id, colorA, colorB) {
    var el = document.getElementById(id);
    if (!el) return null;
    var ctx = el.getContext('2d');
    var a = [], b = [];
    function draw() {
      var dpr = window.devicePixelRatio || 1;
      var w = el.clientWidth, h = 36;
      if (!w) return;
      if (el.width !== w * dpr || el.height !== h * dpr) { el.width = w * dpr; el.height = h * dpr; }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, w, h);
      var max = 1;
      a.concat(b).forEach(function (v) { if (v > max) max = v; });
      max *= 1.15;
      ctx.strokeStyle = 'rgba(255,255,255,0.10)';
      ctx.beginPath(); ctx.moveTo(0, h - 1.5); ctx.lineTo(w, h - 1.5); ctx.stroke();
      [[a, colorA], [b, colorB]].forEach(function (pair) {
        var data = pair[0];
        if (data.length < 2) return;
        ctx.strokeStyle = pair[1]; ctx.lineWidth = 1.5; ctx.beginPath();
        data.forEach(function (v, i) {
          var x = w * i / 59, y = h - 2 - (h - 6) * Math.min(1, v / max);
          if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
        });
        ctx.stroke();
      });
    }
    return {
      push: function (va, vb) {
        a.push(va || 0); b.push(vb || 0);
        if (a.length > 60) { a.shift(); b.shift(); }
        draw();
      },
      draw: draw
    };
  }
  var sparkUptime = makeSpark('spark-uptime', '#94a3b8', null);
  var sparkCpu = makeSpark('spark-cpu', '#60a5fa', 100);
  var sparkMem = makeSpark('spark-mem', '#4ade80', memLimitMB > 0 ? memLimitMB : null);
  var sparkDisk = makeSpark('spark-disk', '#facc15', diskLimitMB > 0 ? diskLimitMB : null);
  var sparkNet = makeSparkDual('spark-net', '#60a5fa', '#4ade80');
  var allSparks = [sparkUptime, sparkCpu, sparkMem, sparkDisk, sparkNet];
  if (window.ResizeObserver) {
    var strip = document.querySelector('.stat-strip');
    if (strip) new ResizeObserver(function () { allSparks.forEach(function (s) { if (s) s.draw(); }); }).observe(strip);
  }

  // --- Grafik traffic live (canvas, tanpa dependensi) ---
  // Stat frame membawa net_rx_kb/net_tx_kb KUMULATIF sejak proses start,
  // jadi rate = delta antar frame / dt (frame datang tiap ~2 detik).
  const traffic = { rx: [], tx: [], lastRx: null, lastTx: null, lastT: 0, max: 90 };
  const chartEl = document.getElementById('traffic-chart');
  const chartCtx = chartEl ? chartEl.getContext('2d') : null;
  function fmtRate(kbs) {
    if (kbs == null || isNaN(kbs)) return '—';
    return kbs >= 1024 ? (kbs / 1024).toFixed(1) + ' MB/s' : Math.round(kbs) + ' KB/s';
  }
  function pushTraffic(d) {
    if (!d || chartEl == null) return;
    const now = Date.now() / 1000;
    const rx = d.net_rx_kb || 0, tx = d.net_tx_kb || 0;
    if (traffic.lastRx != null && now > traffic.lastT) {
      const dt = now - traffic.lastT;
      const rRx = Math.max(0, (rx - traffic.lastRx) / dt);
      const rTx = Math.max(0, (tx - traffic.lastTx) / dt);
      traffic.rx.push(rRx); traffic.tx.push(rTx);
      if (traffic.rx.length > traffic.max) { traffic.rx.shift(); traffic.tx.shift(); }
      document.getElementById('traffic-rx').textContent = fmtRate(rRx);
      document.getElementById('traffic-tx').textContent = fmtRate(rTx);
      if (sparkNet) sparkNet.push(rRx, rTx);
      drawTraffic();
    }
    traffic.lastRx = rx; traffic.lastTx = tx; traffic.lastT = now;
  }
  function drawTraffic() {
    if (!chartCtx) return;
    const dpr = window.devicePixelRatio || 1;
    const w = chartEl.clientWidth, h = 120;
    if (chartEl.width !== w * dpr || chartEl.height !== h * dpr) {
      chartEl.width = w * dpr; chartEl.height = h * dpr;
    }
    const ctx = chartCtx;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    const padL = 34, padR = 6, padT = 8, padB = 18;
    const iw = w - padL - padR, ih = h - padT - padB;
    let max = 10;
    traffic.rx.concat(traffic.tx).forEach(function (v) { if (v > max) max = v; });
    max = Math.ceil(max * 1.2);
    // grid + label sumbu
    ctx.strokeStyle = 'rgba(255,255,255,0.08)'; ctx.fillStyle = '#6f7d8c';
    ctx.font = '10px sans-serif'; ctx.lineWidth = 1;
    for (let i = 0; i <= 3; i++) {
      const y = padT + ih - ih * i / 3;
      ctx.beginPath(); ctx.moveTo(padL, y); ctx.lineTo(w - padR, y); ctx.stroke();
      ctx.fillText(fmtRate(max * i / 3), 0, y + 3);
    }
    function line(data, color) {
      if (data.length < 2) return;
      ctx.strokeStyle = color; ctx.lineWidth = 1.5; ctx.beginPath();
      data.forEach(function (v, i) {
        const x = padL + iw * i / (traffic.max - 1);
        const y = padT + ih - ih * Math.min(1, v / max);
        if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
      });
      ctx.stroke();
    }
    line(traffic.rx, '#60a5fa');
    line(traffic.tx, '#4ade80');
    ctx.fillStyle = '#6f7d8c';
    ctx.fillText('now', w - padR - 22, h - 5);
    ctx.fillText('−3 min', padL, h - 5);
  }
  if (chartEl) {
    window.addEventListener('resize', drawTraffic);
    if (window.ResizeObserver) new ResizeObserver(drawTraffic).observe(chartEl.parentElement);
  }
  function refreshDisk() {
    fetch('/?p=server/usage&id=<?= $id ?>', { credentials: 'same-origin' })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        if (!d || !d.ok) return;
        const usedMB = (d.used || 0) / 1048576, limitMB = (d.limit || 0) / 1048576 || diskLimitMB;
        document.getElementById('st-disk').textContent =
          fmtMB(usedMB) + (limitMB ? ' / ' + fmtMB(limitMB) : '');
        setBar(document.getElementById('st-disk-bar'), limitMB ? usedMB / limitMB * 100 : 0);
        if (sparkDisk) sparkDisk.push(usedMB);
      })
      .catch(function () {});
  }
  refreshDisk();
  setInterval(refreshDisk, 30000);

  const ws = new WebSocket(`<?= $wsProto ?>//<?= e(preg_replace('#^https?://#', '', $wsBase)) ?>/api/servers/<?= $id ?>/ws`);
  const sendResize = () => {
    if (fitAddon) { try { fitAddon.fit(); } catch (e) {} }
    if (ws.readyState === 1 && term.cols && term.rows) {
      ws.send(JSON.stringify({ t: 'resize', cols: term.cols, rows: term.rows }));
    }
  };
  if (fitAddon) {
    // fit() SEKALI di rAF tidak cukup di mobile: CSS xterm / font sering
    // belum settle saat rAF pertama → kolom kehitung kebanyakan → terminal
    // meluber ke kanan (teks terpotong di tepi layar). Refit di beberapa
    // momen: load (stylesheet+font selesai), fonts.ready, timeout, dan
    // ResizeObserver pada wrapper (rotasi, address bar, dsb).
    requestAnimationFrame(sendResize);
    window.addEventListener('load', sendResize);
    window.addEventListener('resize', sendResize);
    setTimeout(sendResize, 150);
    if (document.fonts && document.fonts.ready) document.fonts.ready.then(sendResize);
    if (window.ResizeObserver) {
      const wrap = document.querySelector('.terminal-wrap');
      if (wrap) new ResizeObserver(function () { sendResize(); }).observe(wrap);
    }
  }
  ws.onopen = () => { statusEl.textContent = 'Authenticating…'; ws.send(JSON.stringify({ t: 'auth', token: <?= json_encode($wsToken) ?> })); sendResize(); };
  ws.onmessage = (ev) => {
    try {
      const m = JSON.parse(ev.data);
      if (m.t === 'out') term.write(m.d);
      else if (m.t === 'stat') applyStats(m.d);
      else if (m.t === 'status') statusEl.textContent = m.d?.s || m.s || 'Connected';
    } catch (e) { term.write(ev.data); }
  };
  ws.onclose = () => { statusEl.textContent = 'Disconnected'; };
  term.onData((d) => { if (ws.readyState === 1) ws.send(JSON.stringify({ t: 'in', d })); });
})();
</script>
<?php require __DIR__ . '/../partials/footer.php'; ?>
