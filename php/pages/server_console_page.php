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
?>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm@5.3.0/css/xterm.min.css">
<div class="console-bar">
  <span class="status-dot <?= $st === 'running' ? 'on' : 'off' ?>"></span>
  <span id="conn-status" class="status-line">Connecting…</span>
</div>
<div id="terminal"></div>
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
  const ws = new WebSocket(`<?= $wsProto ?>//<?= e(preg_replace('#^https?://#', '', $wsBase)) ?>/api/servers/<?= $id ?>/ws`);
  const sendResize = () => {
    if (fitAddon) { try { fitAddon.fit(); } catch (e) {} }
    if (ws.readyState === 1 && term.cols && term.rows) {
      ws.send(JSON.stringify({ t: 'resize', cols: term.cols, rows: term.rows }));
    }
  };
  if (fitAddon) {
    requestAnimationFrame(sendResize);
    window.addEventListener('resize', sendResize);
  }
  ws.onopen = () => { statusEl.textContent = 'Authenticating…'; ws.send(JSON.stringify({ t: 'auth', token: <?= json_encode($wsToken) ?> })); sendResize(); };
  ws.onmessage = (ev) => {
    try {
      const m = JSON.parse(ev.data);
      if (m.t === 'out') term.write(m.d);
      else if (m.t === 'status') statusEl.textContent = m.d?.s || m.s || 'Connected';
    } catch (e) { term.write(ev.data); }
  };
  ws.onclose = () => { statusEl.textContent = 'Disconnected'; };
  term.onData((d) => { if (ws.readyState === 1) ws.send(JSON.stringify({ t: 'in', d })); });
})();
</script>
<?php require __DIR__ . '/../partials/footer.php'; ?>
