<?php
// Dashboard — overview stats + server list
$page_title = 'Dashboard';
$active_nav = 'dashboard';

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
    <h1>Dashboard</h1>
    <p>Welcome back, <?= e(gp_user()['username'] ?? '') ?></p>
  </div>
</div>

<div class="stats">
  <div class="stat">
    <div class="label">Servers</div>
    <div class="value"><?= count($serverList) ?></div>
    <div class="sub"><span class="status-dot <?= $online > 0 ? 'on' : 'off' ?>"></span><?= $online ?> running</div>
  </div>
  <?php if (gp_is_admin()): ?>
  <div class="stat">
    <div class="label">Nodes</div>
    <div class="value"><?= count($nodeList) ?></div>
    <div class="sub"><span class="status-dot <?= $nodesOnline > 0 ? 'on' : 'off' ?>"></span><?= $nodesOnline ?> online</div>
  </div>
  <?php endif; ?>
</div>

<div class="card">
  <h3>Your servers</h3>
  <?php if (empty($serverList)): ?>
  <p style="color:var(--text-dim)">No servers yet.</p>
  <?php else: ?>
  <table>
    <thead><tr><th>Name</th><th>Status</th><th>Node</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($serverList as $s): ?>
      <tr>
        <td><strong><?= e($s['name']) ?></strong><br><span class="mono" style="color:var(--text-mute)"><?= e($s['uuid'] ?? '') ?></span></td>
        <td><?= status_badge($s['status'] ?? 'offline') ?></td>
        <td><?= e($s['node_name'] ?? '-') ?></td>
        <td style="text-align:right"><a class="btn small" href="/?p=server&id=<?= $s['id'] ?>">Manage</a></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table>
  <?php endif; ?>
</div>

<?php
function status_badge(string $s): string {
    $map = ['running' => 'green', 'starting' => 'yellow', 'stopping' => 'yellow', 'offline' => 'gray'];
    $cls = $map[$s] ?? 'gray';
    return '<span class="badge ' . $cls . '">' . e($s) . '</span>';
}
require __DIR__ . '/../partials/footer.php';
