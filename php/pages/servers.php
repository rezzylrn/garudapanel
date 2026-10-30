<?php
// Servers list (user's own)
$page_title = 'Servers';
$active_nav = 'servers';
require_once __DIR__ . '/server_layout.php';
$r = gp_api('GET', '/api/servers', null, gp_token());
$list = $r['data'] ?? [];
require __DIR__ . '/../partials/header.php';
?>
<div class="page-head">
  <div><h1><?= icon('server', 20) ?> Servers</h1><p><?= count($list) ?> total</p></div>
</div>
<div class="card">
  <?php if (empty($list)): ?>
  <div class="empty"><?= icon('server', 32) ?><p>No servers yet</p></div>
  <?php else: ?>
  <div class="server-rows">
    <?php foreach ($list as $s): ?>
      <?php $st = $s['status'] ?? 'offline'; ?>
      <a class="server-row" href="/?p=server/console&id=<?= $s['id'] ?>">
        <span class="row-dot <?= $st === 'running' ? 'on' : ($st === 'starting' || $st === 'stopping' ? 'warn' : '') ?>"></span>
        <span class="row-main">
          <span class="row-name"><?= e($s['name']) ?></span>
          <span class="row-sub" style="display:block"><?= e($s['egg_name'] ?? '-') ?> · <?= (int)($s['memory_mb'] ?? 0) ?> MB · <?= (int)($s['disk_mb'] ?? 0) ?> MB · <span class="mono"><?= e(substr($s['uuid'] ?? '', 0, 8)) ?></span></span>
        </span>
        <span class="row-side"><?= gp_status_badge($st) ?><?= icon('chevron', 16) ?></span>
      </a>
    <?php endforeach; ?>
  </div>
  <?php endif; ?>
</div>
<?php require __DIR__ . '/../partials/footer.php'; ?>
