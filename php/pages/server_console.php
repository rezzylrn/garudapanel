<?php
// Console tab — xterm.js + WebSocket (same protocol as before, minimal JS)
$tok = gp_api('POST', "/api/servers/$id/console-token", null, gp_token());
$wsToken = $tok['data']['token'] ?? '';
?>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm@5.3.0/css/xterm.min.css">
<div class="console-bar">
  <span class="status-dot <?= $st === 'running' ? 'on' : 'off' ?>"></span>
  <span id="conn-status" style="color:var(--text-dim);font-size:13px">Connecting…</span>
</div>
<div id="terminal"></div>
<script src="https://cdn.jsdelivr.net/npm/xterm@5.3.0/lib/xterm.min.js"></script>
<script>
(function() {
  const term = new Terminal({ cursorBlink: true, fontSize: 13, theme: { background: '#000000' } });
  term.open(document.getElementById('terminal'));
  const statusEl = document.getElementById('conn-status');
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  // Console WS lives on the Go API host
  const apiHost = <?= json_encode(parse_url(GP_API_URL, PHP_URL_HOST) . ':' . (parse_url(GP_API_URL, PHP_URL_PORT) ?: 80)) ?>;
  const ws = new WebSocket(`${proto}//${apiHost}/api/servers/<?= $id ?>/ws`);
  ws.onopen = () => { statusEl.textContent = 'Authenticating…'; ws.send(JSON.stringify({ t: 'auth', token: <?= json_encode($wsToken) ?> })); };
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
