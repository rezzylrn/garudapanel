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
    <?php if (gp_is_admin()): ?><a class="btn primary" href="/?p=admin/servers/new"><?= icon('plus', 15) ?> New server</a><?php endif; ?>
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
  <div class="server-rows">
    <?php foreach ($serverList as $s): ?>
      <?php $st = $s['status'] ?? 'offline'; ?>
      <a class="server-row" href="/?p=server/console&id=<?= $s['id'] ?>">
        <span class="row-dot <?= $st === 'running' ? 'on' : ($st === 'starting' || $st === 'stopping' ? 'warn' : '') ?>"></span>
        <span class="row-main">
          <span class="row-name"><?= e($s['name']) ?></span>
          <span class="row-sub" style="display:block"><?= e($s['node_name'] ?? '-') ?> · <span class="mono"><?= e(substr($s['uuid'] ?? '', 0, 8)) ?></span></span>
        </span>
        <span class="row-side"><?= gp_status_badge($st) ?><?= icon('chevron', 16) ?></span>
      </a>
    <?php endforeach; ?>
  </div>
  <?php endif; ?>
</div>

<?php
require __DIR__ . '/../partials/footer.php';
