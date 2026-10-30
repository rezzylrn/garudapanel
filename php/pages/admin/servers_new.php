<?php
// Admin — New server (dedicated page, pola Pterodactyl: list → Create New → form)
$page_title = 'New Server';
$active_nav = 'admin_servers';
require_once __DIR__ . '/../../pages/server_layout.php';

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['create'])) {
    $r = gp_api('POST', '/api/admin/servers', [
        'name' => $_POST['name'] ?? '',
        'owner_id' => (int)($_POST['owner_id'] ?? 0),
        'node_id' => (int)($_POST['node_id'] ?? 0),
        'egg_id' => (int)($_POST['egg_id'] ?? 0),
        'allocation_id' => (int)($_POST['allocation_id'] ?? 0),
        'memory_mb' => (int)($_POST['memory_mb'] ?? 512),
        'disk_mb' => (int)($_POST['disk_mb'] ?? 1024),
    ], gp_token());
    if ($r['ok']) {
        header('Location: /?p=admin/servers&created=1');
        exit;
    }
    $error = $r['error'];
}

$nodes = gp_api('GET', '/api/admin/nodes', null, gp_token());
$eggs = gp_api('GET', '/api/admin/eggs', null, gp_token());
$users = gp_api('GET', '/api/admin/users', null, gp_token());
$nodeList = $nodes['data'] ?? [];
// Node default = node pertama yang punya alokasi bebas, supaya dropdown
// alokasi tidak kosong saat halaman pertama dibuka.
$firstNode = 0;
$allocList = [];
foreach ($nodeList as $n) {
    $nid = (int)$n['id'];
    $a = gp_api('GET', "/api/admin/allocations?node_id=$nid", null, gp_token());
    $free = array_values(array_filter($a['data'] ?? [], fn($x) => empty($x['server_id'])));
    if (!empty($free)) { $firstNode = $nid; $allocList = $free; break; }
}
if ($firstNode === 0 && !empty($nodeList)) {
    $firstNode = (int)$nodeList[0]['id'];
    $a = gp_api('GET', "/api/admin/allocations?node_id=$firstNode", null, gp_token());
    $allocList = array_values(array_filter($a['data'] ?? [], fn($x) => empty($x['server_id'])));
}

require __DIR__ . '/../../partials/header.php';
?>
<div class="page-head">
  <div><h1><?= icon('plus', 20) ?> New server</h1><p>Create a server on a node</p></div>
  <div class="page-actions"><a class="btn" href="/?p=admin/servers"><?= icon('back', 15) ?> All servers</a></div>
</div>
<?php if (!empty($error)): ?><div class="alert error"><?= icon('x', 16) ?> <?= e($error) ?></div><?php endif; ?>

<div class="card">
  <form method="POST" id="new-server-form">
    <div class="form-grid">
      <div class="field"><label>Name</label><input type="text" name="name" required></div>
      <div class="field"><label>Owner</label><select name="owner_id" required><?php foreach (($users['data'] ?? []) as $u): ?><option value="<?= $u['id'] ?>"><?= e($u['username']) ?></option><?php endforeach; ?></select></div>
      <div class="field"><label>Node</label><select name="node_id" id="node-select" required><?php foreach ($nodeList as $n): ?><option value="<?= $n['id'] ?>"<?= (int)$n['id'] === $firstNode ? ' selected' : '' ?>><?= e($n['name']) ?></option><?php endforeach; ?></select></div>
      <div class="field"><label>Egg</label><select name="egg_id" required><?php foreach (($eggs['data'] ?? []) as $eg): ?><option value="<?= $eg['id'] ?>"><?= e($eg['name']) ?></option><?php endforeach; ?></select></div>
      <div class="field"><label>Allocation</label>
        <select name="allocation_id" id="alloc-select" required>
          <?php if (empty($allocList)): ?><option value="">No free allocation on this node</option>
          <?php else: foreach ($allocList as $a): ?><option value="<?= $a['id'] ?>"><?= e($a['ip']) ?>:<?= (int)$a['port'] ?></option><?php endforeach; endif; ?>
        </select>
      </div>
      <div class="field"><label>Memory (MB)</label><input type="number" name="memory_mb" value="512" required></div>
      <div class="field"><label>Disk (MB)</label><input type="number" name="disk_mb" value="1024" required></div>
    </div>
    <button type="submit" name="create" class="btn primary" style="margin-top:12px"><?= icon('plus', 15) ?> Create server</button>
  </form>
</div>
<script>
(function () {
  var nodeSel = document.getElementById('node-select');
  var allocSel = document.getElementById('alloc-select');
  nodeSel.addEventListener('change', function () {
    var nid = nodeSel.value;
    allocSel.innerHTML = '<option value="">Loading…</option>';
    fetch('/?p=admin/node-allocations&node_id=' + encodeURIComponent(nid), { credentials: 'same-origin' })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        var list = (d && d.data) || [];
        var free = list.filter(function (a) { return !a.server_id; });
        if (!free.length) {
          allocSel.innerHTML = '<option value="">No free allocation on this node</option>';
          return;
        }
        allocSel.innerHTML = free.map(function (a) {
          return '<option value="' + a.id + '">' + a.ip + ':' + a.port + '</option>';
        }).join('');
      })
      .catch(function () {
        allocSel.innerHTML = '<option value="">Failed to load allocations</option>';
      });
  });
})();
</script>
<?php require __DIR__ . '/../../partials/footer.php'; ?>
