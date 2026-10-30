<?php
// Admin — All servers (list only; create lives on its own page)
$page_title = 'All Servers';
$active_nav = 'admin_servers';
require_once __DIR__ . '/../../pages/server_layout.php';

$servers = gp_api('GET', '/api/admin/servers', null, gp_token());
$list = $servers['data'] ?? [];
$created = isset($_GET['created']);

require __DIR__ . '/../../partials/header.php';
?>
<div class="page-head">
  <div><h1><?= icon('cpu', 20) ?> All Servers</h1><p><?= count($list) ?> total</p></div>
  <div class="page-actions"><a class="btn primary" href="/?p=admin/servers/new"><?= icon('plus', 15) ?> New server</a></div>
</div>
<?php if ($created): ?><div class="alert ok"><?= icon('check', 16) ?> Server created.</div><?php endif; ?>

<div class="card">
  <?php if (empty($list)): ?>
  <div class="empty"><?= icon('server', 32) ?><p>No servers</p></div>
  <?php else: ?>
  <div class="server-rows">
    <?php foreach ($list as $s): ?>
      <?php $st = $s['status'] ?? 'offline'; ?>
      <a class="server-row" href="/?p=server/console&id=<?= $s['id'] ?>">
        <span class="row-dot <?= $st === 'running' ? 'on' : ($st === 'starting' || $st === 'stopping' ? 'warn' : '') ?>"></span>
        <span class="row-main">
          <span class="row-name"><?= e($s['name']) ?></span>
          <span class="row-sub" style="display:block"><?= e($s['node_name'] ?? '-') ?> · owner <?= e($s['owner_name'] ?? $s['owner_id'] ?? '-') ?> · <?= e($s['egg_name'] ?? '-') ?></span>
        </span>
        <span class="row-side"><?= gp_status_badge($st) ?><?= icon('chevron', 16) ?></span>
      </a>
    <?php endforeach; ?>
  </div>
  <?php endif; ?>
</div>
<?php require __DIR__ . '/../../partials/footer.php'; ?>
