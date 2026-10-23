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
  <div><h1>Nodes</h1><p>Game server worker machines</p></div>
</div>

<?php if (!empty($okMsg)): ?><div class="alert ok"><?= $okMsg ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3>New node</h3>
  <form method="POST" style="display:grid;grid-template-columns:1fr 1fr 1fr auto;gap:12px;align-items:end">
    <div class="field" style="margin:0"><label>Name</label><input type="text" name="name" required placeholder="node-1"></div>
    <div class="field" style="margin:0"><label>FQDN / IP</label><input type="text" name="fqdn" required placeholder="192.168.1.10"></div>
    <div class="field" style="margin:0"><label>Location</label>
      <select name="location_id" required>
        <?php foreach ($locList as $l): ?><option value="<?= $l['id'] ?>"><?= e($l['short_code']) ?></option><?php endforeach; ?>
      </select>
    </div>
    <button type="submit" name="create" class="btn primary">Create</button>
  </form>
</div>

<div class="card">
  <table>
    <thead><tr><th>Name</th><th>FQDN</th><th>Status</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($nodeList as $n): ?>
      <tr>
        <td><strong><?= e($n['name']) ?></strong></td>
        <td class="mono"><?= e($n['fqdn'] ?? '') ?></td>
        <td><?= !empty($n['online']) ? '<span class="badge green">online</span>' : '<span class="badge gray">offline</span>' ?></td>
        <td></td>
      </tr>
    <?php endforeach; ?>
    <?php if (empty($nodeList)): ?><tr><td colspan="4" style="color:var(--text-mute);text-align:center">No nodes yet</td></tr><?php endif; ?>
    </tbody>
  </table>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
