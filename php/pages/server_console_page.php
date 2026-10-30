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
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('cpu', 14) ?> CPU Load</span>
    <span class="value" id="st-cpu">—</span>
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('memory', 14) ?> Memory</span>
    <span class="value" id="st-mem">—</span>
    <span class="meter"><span id="st-mem-bar" style="width:0%"></span></span>
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('hdd', 14) ?> Disk</span>
    <span class="value" id="st-disk">—</span>
    <span class="meter"><span id="st-disk-bar" style="width:0%"></span></span>
  </div>
  <div class="stat mini">
    <span class="label"><?= icon('network', 14) ?> Network</span>
    <span class="value mono" id="st-net" style="font-size:13px">—</span>
  </div>
</div>
<div class="console-bar">
  <span class="status-dot <?= $st === 'running' ? 'on' : 'off' ?>" id="conn-dot"></span>
  <span id="conn-status" class="status-line">Connecting…</span>
</div>
<div class="terminal-wrap"><div id="terminal"></div></div>
<script src="https://cdn.jsdelivr.net/npm/xterm@5.3.0/lib/xterm.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/xterm-addon-fit@5.3.0/lib/xterm-addon-fit.min.js"></script>
<script>
(function() {
  const term = new Terminal({ cursorBlink: true, fontSize: 13, theme: { background: '#000000' } });
  term.open(document.getElementById('terminal'));
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
    document.getElementById('st-cpu').textContent =
      (d.cpu_percent == null ? '—' : d.cpu_percent.toFixed(1) + '%');
    const memUsed = d.mem_used_mb || 0, memLimit = d.mem_limit_mb || memLimitMB;
    document.getElementById('st-mem').textContent =
      fmtMB(memUsed) + (memLimit ? ' / ' + fmtMB(memLimit) : '');
    setBar(document.getElementById('st-mem-bar'), memLimit ? memUsed / memLimit * 100 : 0);
    const rx = d.net_rx_kb || 0, tx = d.net_tx_kb || 0;
    document.getElementById('st-net').textContent =
      (rx <= 0 && tx <= 0) ? '—' : '↓ ' + fmtKB(rx) + ' · ↑ ' + fmtKB(tx);
    if (d.state) {
      dotEl.className = 'status-dot ' + (d.state === 'running' ? 'on' : 'off');
      if (window.__gpSetStatus) window.__gpSetStatus(d.state);
    }
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
