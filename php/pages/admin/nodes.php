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
    <thead><tr><th>Name</th><th>FQDN</th><th>Status</th></tr></thead>
    <tbody>
    <?php foreach ($nodeList as $n): ?>
      <tr>
        <td><span class="cell-main"><?= e($n['name']) ?></span></td>
        <td class="mono"><?= e($n['fqdn'] ?? '') ?></td>
        <td><?= !empty($n['online']) ? '<span class="badge green"><span class="dot"></span>online</span>' : '<span class="badge gray"><span class="dot"></span>offline</span>' ?></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
