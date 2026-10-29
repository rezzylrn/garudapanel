<?php
// Admin — All servers + create
$page_title = 'All Servers';
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
    if ($r['ok']) { $okMsg = 'Server created.'; } else { $error = $r['error']; }
}

$servers = gp_api('GET', '/api/admin/servers', null, gp_token());
$nodes = gp_api('GET', '/api/admin/nodes', null, gp_token());
$eggs = gp_api('GET', '/api/admin/eggs', null, gp_token());
$users = gp_api('GET', '/api/admin/users', null, gp_token());
$list = $servers['data'] ?? [];

require __DIR__ . '/../../partials/header.php';
?>
<div class="page-head"><div><h1><?= icon('cpu', 20) ?> All Servers</h1></div></div>
<?php if (!empty($okMsg)): ?><div class="alert ok"><?= icon('check', 16) ?> <?= e($okMsg) ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= icon('x', 16) ?> <?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3><?= icon('plus', 15) ?> New server</h3>
  <form method="POST">
    <div class="form-grid">
      <div class="field"><label>Name</label><input type="text" name="name" required></div>
      <div class="field"><label>Owner</label><select name="owner_id" required><?php foreach (($users['data'] ?? []) as $u): ?><option value="<?= $u['id'] ?>"><?= e($u['username']) ?></option><?php endforeach; ?></select></div>
      <div class="field"><label>Node</label><select name="node_id" required><?php foreach (($nodes['data'] ?? []) as $n): ?><option value="<?= $n['id'] ?>"><?= e($n['name']) ?></option><?php endforeach; ?></select></div>
      <div class="field"><label>Egg</label><select name="egg_id" required><?php foreach (($eggs['data'] ?? []) as $eg): ?><option value="<?= $eg['id'] ?>"><?= e($eg['name']) ?></option><?php endforeach; ?></select></div>
      <div class="field"><label>Allocation ID</label><input type="number" name="allocation_id" required></div>
      <div class="field"><label>Memory (MB)</label><input type="number" name="memory_mb" value="512" required></div>
      <div class="field"><label>Disk (MB)</label><input type="number" name="disk_mb" value="1024" required></div>
    </div>
    <button type="submit" name="create" class="btn primary" style="margin-top:12px"><?= icon('plus', 15) ?> Create</button>
  </form>
</div>

<div class="card">
  <?php if (empty($list)): ?>
  <div class="empty"><?= icon('server', 32) ?><p>No servers</p></div>
  <?php else: ?>
  <div class="table-wrap"><table>
    <thead><tr><th>Name</th><th>Owner</th><th>Node</th><th>Status</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($list as $s): ?>
      <tr>
        <td><span class="cell-main"><?= e($s['name']) ?></span></td>
        <td><?= e($s['owner_name'] ?? $s['owner_id'] ?? '-') ?></td>
        <td><?= e($s['node_name'] ?? '-') ?></td>
        <td><?= gp_status_badge($s['status'] ?? 'offline') ?></td>
        <td><div class="row-actions"><a class="act-btn" title="Manage" href="/?p=server/console&id=<?= $s['id'] ?>"><?= icon('chevron', 16) ?></a></div></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>
<?php require __DIR__ . '/../../partials/footer.php'; ?>
