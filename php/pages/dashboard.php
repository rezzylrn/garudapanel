<?php
// Dashboard — overview stats + server list
$page_title = 'Dashboard';
$active_nav = 'dashboard';
require_once __DIR__ . '/server_layout.php';

$servers = gp_api('GET', '/api/servers', null, gp_token());
$nodes = gp_is_admin() ? gp_api('GET', '/api/admin/nodes', null, gp_token()) : null;

$serverList = $servers['data'] ?? [];
$nodeList = $nodes['data'] ?? [];

$online = count(array_filter($serverList, fn($s) => ($s['status'] ?? '') === 'running'));
$nodesOnline = count(array_filter($nodeList, fn($n) => !empty($n['online'])));

require __DIR__ . '/../partials/header.php';
?>

<div class="page-head">
  <div>
    <h1><?= icon('dashboard', 20) ?> Dashboard</h1>
    <p>Welcome back, <?= e(gp_user()['username'] ?? '') ?></p>
  </div>
  <div class="page-actions">
    <?php if (gp_is_admin()): ?><a class="btn primary" href="/?p=admin/servers"><?= icon('plus', 15) ?> New server</a><?php endif; ?>
  </div>
</div>

<div class="stats">
  <div class="stat">
    <div class="label"><?= icon('server', 13) ?> Servers</div>
    <div class="value"><?= count($serverList) ?></div>
    <div class="sub"><span class="status-dot <?= $online > 0 ? 'on' : 'off' ?>"></span><?= $online ?> running</div>
  </div>
  <?php if (gp_is_admin()): ?>
  <div class="stat">
    <div class="label"><?= icon('nodes', 13) ?> Nodes</div>
    <div class="value"><?= count($nodeList) ?></div>
    <div class="sub"><span class="status-dot <?= $nodesOnline > 0 ? 'on' : 'off' ?>"></span><?= $nodesOnline ?> online</div>
  </div>
  <?php endif; ?>
</div>

<div class="card">
  <h3><?= icon('server', 15) ?> Your servers</h3>
  <?php if (empty($serverList)): ?>
  <div class="empty"><?= icon('server', 32) ?><p>No servers yet.</p></div>
  <?php else: ?>
  <div class="table-wrap"><table>
    <thead><tr><th>Name</th><th>Status</th><th>Node</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($serverList as $s): ?>
      <tr>
        <td><span class="cell-main"><?= e($s['name']) ?></span><br><span class="mono cell-sub"><?= e($s['uuid'] ?? '') ?></span></td>
        <td><?= gp_status_badge($s['status'] ?? 'offline') ?></td>
        <td><?= e($s['node_name'] ?? '-') ?></td>
        <td><div class="row-actions"><a class="act-btn" title="Manage" href="/?p=server/console&id=<?= $s['id'] ?>"><?= icon('chevron', 16) ?></a></div></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>

<?php
require __DIR__ . '/../partials/footer.php';
