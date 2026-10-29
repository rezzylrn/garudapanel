<?php
// Servers list (user's own)
$page_title = 'Servers';
$active_nav = 'servers';
require_once __DIR__ . '/server_layout.php';
$r = gp_api('GET', '/api/servers', null, gp_token());
$list = $r['data'] ?? [];
require __DIR__ . '/partials/header.php';
?>
<div class="page-head">
  <div><h1><?= icon('server', 20) ?> Servers</h1><p><?= count($list) ?> total</p></div>
</div>
<div class="card">
  <?php if (empty($list)): ?>
  <div class="empty"><?= icon('server', 32) ?><p>No servers yet</p></div>
  <?php else: ?>
  <div class="table-wrap"><table>
    <thead><tr><th>Name</th><th>Status</th><th>Egg</th><th>Resources</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($list as $s): ?>
      <tr>
        <td><span class="cell-main"><?= e($s['name']) ?></span><br><span class="mono cell-sub"><?= e(substr($s['uuid'] ?? '', 0, 8)) ?></span></td>
        <td><?= gp_status_badge($s['status'] ?? 'offline') ?></td>
        <td><?= e($s['egg_name'] ?? '-') ?></td>
        <td class="mono" style="font-size:12px"><?= (int)($s['memory_mb'] ?? 0) ?> MB · <?= (int)($s['disk_mb'] ?? 0) ?> MB</td>
        <td><div class="row-actions"><a class="act-btn" title="Manage" href="/?p=server/console&id=<?= $s['id'] ?>"><?= icon('chevron', 16) ?></a></div></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>
<?php require __DIR__ . '/partials/footer.php'; ?>
