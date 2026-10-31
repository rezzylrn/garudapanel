<?php
// Admin — Nodes
$page_title = 'Nodes';
$active_nav = 'nodes';

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['create'])) {
    $r = gp_api('POST', '/api/admin/nodes', [
        'name' => $_POST['name'] ?? '',
        'fqdn' => $_POST['fqdn'] ?? '',
        'location_id' => (int)($_POST['location_id'] ?? 0),
    ], gp_token());
    if ($r['ok']) {
        $newToken = $r['data']['join_token'] ?? '';
        $okMsg = 'Node created.' . ($newToken ? ' Join token (shown once): <span class="mono">' . e($newToken) . '</span>' : '');
    } else {
        $error = $r['error'];
    }
}

$nodes = gp_api('GET', '/api/admin/nodes', null, gp_token());
$locations = gp_api('GET', '/api/admin/locations', null, gp_token());
$nodeList = $nodes['data'] ?? [];
$locList = $locations['data'] ?? [];

require __DIR__ . '/../../partials/header.php';
?>

<div class="page-head">
  <div><h1><?= icon('nodes', 20) ?> Nodes</h1><p>Game server worker machines</p></div>
</div>

<?php if (!empty($okMsg)): ?><div class="alert ok"><?= icon('check', 16) ?> <?= $okMsg ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= icon('x', 16) ?> <?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3><?= icon('plus', 15) ?> New node</h3>
  <form method="POST">
    <div class="form-grid">
      <div class="field"><label>Name</label><input type="text" name="name" required placeholder="node-1"></div>
      <div class="field"><label>FQDN / IP</label><input type="text" name="fqdn" required placeholder="192.168.1.10"></div>
      <div class="field"><label>Location</label>
        <select name="location_id" required>
          <?php foreach ($locList as $l): ?><option value="<?= $l['id'] ?>"><?= e($l['short_code']) ?></option><?php endforeach; ?>
        </select>
      </div>
    </div>
    <button type="submit" name="create" class="btn primary" style="margin-top:12px"><?= icon('plus', 15) ?> Create</button>
  </form>
</div>

<div class="card">
  <?php if (empty($nodeList)): ?>
  <div class="empty"><?= icon('nodes', 32) ?><p>No nodes yet</p></div>
  <?php else: ?>
  <div class="table-wrap"><table>
    <thead><tr><th>Name</th><th>FQDN</th><th>Status</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($nodeList as $n): ?>
      <tr>
        <td><span class="cell-main"><?= e($n['name']) ?></span></td>
        <td class="mono"><?= e($n['fqdn'] ?? '') ?></td>
        <td><?= !empty($n['online']) ? '<span class="badge green"><span class="dot"></span>online</span>' : '<span class="badge gray"><span class="dot"></span>offline</span>' ?></td>
        <td><div class="row-actions">
          <?php if (empty($n['online'])): ?>
          <button class="btn small" data-activate-node="<?= $n['id'] ?>" data-node-name="<?= e($n['name']) ?>"><?= icon('play', 13) ?> Activate</button>
          <?php endif; ?>
        </div></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>

<div class="modal-backdrop" id="activate-backdrop" hidden>
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="activate-title" style="max-width:640px">
    <h2 id="activate-title">Activate node</h2>
    <p id="activate-sub">Jalankan salah satu script di bawah ini di mesin node. Token sekali pakai, berlaku <strong>15 menit</strong> — buka ulang tombol Activate kalau kedaluwarsa.</p>
    <div class="field"><label>Join token</label>
      <div style="display:flex;gap:8px">
        <input type="text" id="activate-token" readonly class="mono" style="flex:1">
        <button class="btn small" data-copy-target="activate-token">Copy</button>
      </div>
    </div>
    <div class="field"><label>Docker <span style="text-transform:none;letter-spacing:0">(paste ke terminal Docker host)</span></label>
      <pre class="cmd-box" id="activate-docker"></pre>
      <button class="btn small" data-copy-target="activate-docker" style="margin-top:6px">Copy script</button>
    </div>
    <div class="field"><label>Binary langsung <span style="text-transform:none;letter-spacing:0">(tanpa Docker)</span></label>
      <pre class="cmd-box" id="activate-binary"></pre>
      <button class="btn small" data-copy-target="activate-binary" style="margin-top:6px">Copy script</button>
    </div>
    <div class="modal-actions"><button class="btn" id="activate-close">Close</button></div>
  </div>
</div>
<script>
(function () {
  var backdrop = document.getElementById('activate-backdrop');
  var title = document.getElementById('activate-title');
  function close() { backdrop.hidden = true; }
  document.getElementById('activate-close').addEventListener('click', close);
  backdrop.addEventListener('click', function (ev) { if (ev.target === backdrop) close(); });
  document.addEventListener('keydown', function (ev) { if (ev.key === 'Escape') close(); });

  document.querySelectorAll('[data-copy-target]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var el = document.getElementById(btn.getAttribute('data-copy-target'));
      var text = el.value !== undefined ? el.value : el.textContent;
      function done(ok) { btn.textContent = ok ? 'Copied' : 'Copy failed'; setTimeout(function () { btn.textContent = btn.getAttribute('data-copy-target') === 'activate-token' ? 'Copy' : 'Copy script'; }, 1500); }
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function () { done(true); }, function () { done(false); });
      } else {
        var ta = document.createElement('textarea');
        ta.value = text; document.body.appendChild(ta); ta.select();
        var ok = false; try { ok = document.execCommand('copy'); } catch (e) {}
        document.body.removeChild(ta); done(ok);
      }
    });
  });

  document.querySelectorAll('[data-activate-node]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      if (btn.disabled) return;
      btn.disabled = true;
      var id = btn.getAttribute('data-activate-node');
      var name = btn.getAttribute('data-node-name');
      title.textContent = 'Activate node — ' + name;
      document.getElementById('activate-token').value = 'Loading…';
      document.getElementById('activate-docker').textContent = 'Loading…';
      document.getElementById('activate-binary').textContent = 'Loading…';
      backdrop.hidden = false;
      fetch('/?p=admin/node-join-token&id=' + encodeURIComponent(id), { method: 'POST', credentials: 'same-origin' })
        .then(function (r) { return r.json(); })
        .then(function (d) {
          if (!d || !d.ok || !d.join_token) throw new Error('failed');
          var tok = d.join_token, url = d.panel_url;
          document.getElementById('activate-token').value = tok;
          document.getElementById('activate-docker').textContent =
            'docker run -d --name garuda-daemon --restart always -v garuda-data:/var/lib/garudapanel alpine:3.20 sh -c "apk add --no-cache curl >/dev/null 2>&1 && curl -fsSL https://github.com/rezzylrn/garudapanel/releases/latest/download/garuda -o /usr/local/bin/garuda && chmod +x /usr/local/bin/garuda && exec /usr/local/bin/garuda daemon --panel ' + url + ' --join ' + tok + ' --data /var/lib/garudapanel"';
          document.getElementById('activate-binary').textContent =
            'curl -fsSL https://github.com/rezzylrn/garudapanel/releases/latest/download/garuda -o /usr/local/bin/garuda && chmod +x /usr/local/bin/garuda\n' +
            'garuda daemon --panel ' + url + ' --join ' + tok + ' --data /var/lib/garudapanel';
        })
        .catch(function () {
          document.getElementById('activate-token').value = '';
          document.getElementById('activate-docker').textContent = 'Gagal membuat join token, coba lagi.';
          document.getElementById('activate-binary').textContent = '';
        })
        .finally(function () { btn.disabled = false; });
    });
  });
})();
</script>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
