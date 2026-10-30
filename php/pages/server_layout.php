<?php
// Shared loader for server sub-pages. Sets $s (server), $id, $st, $badgeCls.
// Renders page head + power actions + subnav. Call gp_server_head($activeTab) after require.

// Returns server array or dies.
function gp_load_server(int $id): array {
    $r = gp_api('GET', "/api/servers/$id", null, gp_token());
    if (!$r['ok']) { http_response_code(404); die('Server not found'); }
    return $r['data'];
}

function gp_status_badge(string $st): string {
    $cls = ['running' => 'green', 'starting' => 'yellow', 'stopping' => 'yellow'][$st] ?? 'gray';
    return '<span class="badge ' . $cls . '"><span class="dot"></span>' . e($st) . '</span>';
}

function gp_server_head(array $s, int $id, string $activeTab): void {
    $st = $s['status'] ?? 'offline';

    // Power actions: POST ke halaman ini (?action=start|stop|restart|kill).
    // Dijawab JSON supaya JS bisa menampilkan error 409 dari state guard
    // backend (mis. stop saat masih starting) sebagai toast. Ditangani
    // SEBELUM header dirender agar tidak ada HTML yang terlanjur terkirim.
    if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_GET['action'])) {
        $action = $_GET['action'];
        header('Content-Type: application/json');
        if (!in_array($action, ['start', 'stop', 'restart', 'kill'], true)) {
            http_response_code(422);
            echo json_encode(['ok' => false, 'error' => 'Unknown action']);
            exit;
        }
        $r = gp_api('POST', "/api/servers/$id/power", ['action' => $action], gp_token());
        http_response_code($r['ok'] ? 202 : ($r['code'] ?: 500));
        echo json_encode($r['ok']
            ? ['ok' => true, 'action' => $action]
            : ['ok' => false, 'error' => $r['error'] ?? 'Power action failed']);
        exit;
    }

    $page_title = $s['name'];
    $active_nav = 'servers';
    require __DIR__ . '/../partials/header.php';
    ?>
    <div class="page-head">
      <div class="page-head-title">
        <h1><?= icon('server', 20) ?> <?= e($s['name']) ?> <span id="server-badge"><?= gp_status_badge($st) ?></span></h1>
        <p class="mono"><?= e($s['uuid'] ?? '') ?> · <?= e($s['node_name'] ?? '') ?> · <?= (int)($s['memory_mb'] ?? 0) ?> MB</p>
      </div>
      <div class="page-actions" id="power-actions" data-server-id="<?= $id ?>" data-status="<?= e($st) ?>" data-server-name="<?= e($s['name']) ?>">
        <button type="button" class="btn small" data-power-action="start"><?= icon('play', 14) ?> Start</button>
        <button type="button" class="btn small" data-power-action="restart"><?= icon('restart', 14) ?> Restart</button>
        <button type="button" class="btn small danger" data-power-action="stop" id="btn-stop"><?= icon('stop', 14) ?> <span id="btn-stop-label">Stop</span></button>
      </div>
    </div>
    <nav class="subnav">
      <a href="/?p=server/console&id=<?= $id ?>" class="<?= $activeTab === 'console' ? 'active' : '' ?>"><?= icon('terminal', 16) ?> Console</a>
      <a href="/?p=server/files&id=<?= $id ?>" class="<?= $activeTab === 'files' ? 'active' : '' ?>"><?= icon('folder', 16) ?> Files</a>
      <a href="/?p=server/startup&id=<?= $id ?>" class="<?= $activeTab === 'startup' ? 'active' : '' ?>"><?= icon('settings', 16) ?> Startup</a>
    </nav>

    <?php /* Power-action UI: dialog konfirmasi 2 langkah + state guard.
       Aturan status (riset PowerButtons.tsx Pterodactyl, diperketat):
       - Start hanya dari offline. Restart/Stop hanya dari running.
       - Stop/Restart saat "starting" DIBLOKIR + toast error (bukan diam-diam).
       - Saat "stopping", tombol Stop berubah jadi Kill (dialog peringatan
         data corruption, pola upstream). Kill juga muncul sebagai escape
         hatch bila "starting" macet > 60 detik.
       Tombol yang diblokir tetap bisa diklik (aria-disabled, bukan
       disabled) supaya alasannya bisa ditampilkan sebagai error. */ ?>
    <div class="modal-backdrop" id="gp-confirm-backdrop" hidden>
      <div class="modal" role="dialog" aria-modal="true" aria-labelledby="gp-confirm-title">
        <h2 id="gp-confirm-title"></h2>
        <p id="gp-confirm-body"></p>
        <div class="modal-actions">
          <button type="button" class="btn" id="gp-confirm-cancel">Cancel</button>
          <button type="button" class="btn primary" id="gp-confirm-ok">OK</button>
        </div>
      </div>
    </div>
    <div class="toast-stack" id="gp-toasts" aria-live="polite"></div>
    <script>
    (function () {
      var wrap = document.getElementById('power-actions');
      if (!wrap) return;
      var serverId = wrap.getAttribute('data-server-id');
      var serverName = wrap.getAttribute('data-server-name');
      var status = wrap.getAttribute('data-status') || 'offline';
      var startingSince = status === 'starting' ? Date.now() : 0;
      var pollTimer = null;
      var busy = false;

      var badgeEl = document.getElementById('server-badge');
      var stopBtn = document.getElementById('btn-stop');
      var stopLabel = document.getElementById('btn-stop-label');
      var btns = {};
      wrap.querySelectorAll('[data-power-action]').forEach(function (b) {
        btns[b.getAttribute('data-power-action')] = b;
      });

      var CONFIRM = {
        start:   { title: 'Start server?',   body: 'Start ' + serverName + '? The server will launch with its configured startup command.', ok: 'Start',   danger: false },
        restart: { title: 'Restart server?', body: 'Restart ' + serverName + '? The running process will be stopped and started again.', ok: 'Restart', danger: false },
        stop:    { title: 'Stop server?',    body: 'Stop ' + serverName + '? The server process will be stopped.', ok: 'Stop',    danger: true },
        kill:    { title: 'Forcibly stop process?', body: 'Forcibly stopping a server can lead to data corruption. Only use this when the server is stuck stopping or starting.', ok: 'Kill', danger: true }
      };

      function blockedReason(action) {
        if (action === 'start') {
          if (status === 'running') return 'Server is already running.';
          if (status === 'starting') return 'Server is already starting. Please wait until it is running.';
          if (status === 'stopping') return 'Server is stopping. Please wait until it is offline.';
          return null;
        }
        if (action === 'restart') {
          if (status === 'offline') return 'Server is offline. Start it instead.';
          if (status === 'starting') return 'Server is still starting. Wait until it is running before restarting.';
          if (status === 'stopping') return 'Server is stopping. Please wait until it is offline.';
          return null;
        }
        if (action === 'stop' || action === 'kill') {
          if (status === 'offline') return 'Server is already offline.';
          if (status === 'starting' && action === 'stop') return 'Server is still starting. Wait until it is running before stopping.';
          if (status === 'stopping' && action === 'stop') return 'Server is already stopping. Use Kill to force stop.';
          return null;
        }
        return null;
      }

      function render() {
        var cls = { running: 'green', starting: 'yellow', stopping: 'yellow' }[status] || 'gray';
        badgeEl.innerHTML = '<span class="badge ' + cls + '"><span class="dot"></span>' + status + '</span>';
        var killMode = status === 'stopping' || (status === 'starting' && startingSince && Date.now() - startingSince >= 60000);
        stopLabel.textContent = killMode ? 'Kill' : 'Stop';
        stopBtn.setAttribute('data-power-action', killMode ? 'kill' : 'stop');
        // Tombol Start/Restart + tombol Stop/Kill (satu elemen, aksinya
        // berubah) dinilai dari aksi yang sedang ditampilkan.
        [['start', btns.start], ['restart', btns.restart]].forEach(function (pair) {
          if (!pair[1]) return;
          var blocked = !!blockedReason(pair[0]);
          pair[1].classList.toggle('is-blocked', blocked);
          pair[1].setAttribute('aria-disabled', blocked ? 'true' : 'false');
        });
        var stopBlocked = !!blockedReason(stopBtn.getAttribute('data-power-action'));
        stopBtn.classList.toggle('is-blocked', stopBlocked);
        stopBtn.setAttribute('aria-disabled', stopBlocked ? 'true' : 'false');
      }

      function toast(msg, kind) {
        var stack = document.getElementById('gp-toasts');
        var el = document.createElement('div');
        el.className = 'toast ' + (kind || 'error');
        el.textContent = msg;
        stack.appendChild(el);
        setTimeout(function () { el.classList.add('out'); setTimeout(function () { el.remove(); }, 250); }, 4500);
      }

      var backdrop = document.getElementById('gp-confirm-backdrop');
      var okBtn = document.getElementById('gp-confirm-ok');
      var cancelBtn = document.getElementById('gp-confirm-cancel');
      var resolveConfirm = null;
      function confirmDialog(cfg) {
        document.getElementById('gp-confirm-title').textContent = cfg.title;
        document.getElementById('gp-confirm-body').textContent = cfg.body;
        okBtn.textContent = cfg.ok;
        okBtn.className = 'btn ' + (cfg.danger ? 'danger-solid' : 'primary');
        backdrop.hidden = false;
        okBtn.focus();
        return new Promise(function (resolve) { resolveConfirm = resolve; });
      }
      function closeDialog(val) {
        backdrop.hidden = true;
        if (resolveConfirm) { resolveConfirm(val); resolveConfirm = null; }
      }
      okBtn.addEventListener('click', function () { closeDialog(true); });
      cancelBtn.addEventListener('click', function () { closeDialog(false); });
      backdrop.addEventListener('click', function (ev) { if (ev.target === backdrop) closeDialog(false); });
      document.addEventListener('keydown', function (ev) {
        if (ev.key === 'Escape' && !backdrop.hidden) closeDialog(false);
      });

      function setStatus(next) {
        if (next === status) return;
        if (next === 'starting' && status !== 'starting') startingSince = Date.now();
        if (next !== 'starting') startingSince = 0;
        status = next;
        render();
      }
      // Hook untuk halaman console: frame stat WS juga membawa status live.
      window.__gpSetStatus = setStatus;
      window.__gpGetStatus = function () { return status; };

      function pollStatus(left) {
        if (pollTimer) { clearTimeout(pollTimer); pollTimer = null; }
        if (left <= 0) return;
        pollTimer = setTimeout(function () {
          fetch('/?p=server/status&id=' + encodeURIComponent(serverId), { credentials: 'same-origin' })
            .then(function (r) { return r.json(); })
            .then(function (d) {
              if (d && d.status) setStatus(d.status);
              if (status === 'starting' || status === 'stopping') pollStatus(left - 1);
            })
            .catch(function () { if (status === 'starting' || status === 'stopping') pollStatus(left - 1); });
        }, 2000);
      }

      wrap.addEventListener('click', function (ev) {
        var b = ev.target.closest('[data-power-action]');
        if (!b) return;
        var action = b.getAttribute('data-power-action');
        var reason = blockedReason(action);
        if (reason) { toast(reason, 'error'); return; }
        if (busy) return;
        confirmDialog(CONFIRM[action]).then(function (yes) {
          if (!yes) return;
          busy = true;
          var u = new URL(location.href);
          u.searchParams.set('action', action);
          fetch(u.toString(), { method: 'POST', credentials: 'same-origin' })
            .then(function (r) {
              var ct = r.headers.get('content-type') || '';
              if (ct.indexOf('application/json') === -1) throw new Error('bad-response');
              return r.json().then(function (d) { return { http: r.status, body: d }; });
            })
            .then(function (res) {
              if (!res.body.ok) { toast(res.body.error || 'Power action failed.', 'error'); return; }
              toast('Command sent: ' + action + '.', 'ok');
              if (action === 'start' || action === 'restart') setStatus('starting');
              else if (action === 'stop' || action === 'kill') setStatus('stopping');
              pollStatus(30);
            })
            .catch(function () { toast('Could not reach the panel. Please refresh and try again.', 'error'); })
            .finally(function () { busy = false; });
        });
      });

      render();
      if (status === 'starting' || status === 'stopping') pollStatus(30);
    })();
    </script>
    <?php
}
