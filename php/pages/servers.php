<?php
// Servers list (user's own)
$page_title = 'Servers';
$active_nav = 'servers';
$r = gp_api('GET', '/api/servers', null, gp_token());
$list = $r['data'] ?? [];
require __DIR__ . '/partials/header.php';
?>
<div class="page-head"><div><h1>Servers</h1><p><?= count($list) ?> total</p></div></div>
<div class="card">
  <table>
    <thead><tr><th>Name</th><th>Status</th><th>Egg</th><th>Resources</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($list as $s): ?>
      <?php $st = $s['status'] ?? 'offline'; $cls = ['running' => 'green', 'starting' => 'yellow', 'stopping' => 'yellow'][$st] ?? 'gray'; ?>
      <tr>
        <td><strong><?= e($s['name']) ?></strong><br><span class="mono" style="color:var(--text-mute);font-size:11px"><?= e(substr($s['uuid'] ?? '', 0, 8)) ?></span></td>
        <td><span class="badge <?= $cls ?>"><?= e($st) ?></span></td>
        <td style="color:var(--text-dim)"><?= e($s['egg_name'] ?? '-') ?></td>
        <td class="mono" style="font-size:12px"><?= (int)($s['memory_mb'] ?? 0) ?> MB · <?= (int)($s['disk_mb'] ?? 0) ?> MB</td>
        <td style="text-align:right"><a class="btn small" href="/?p=server&id=<?= $s['id'] ?>">Manage</a></td>
      </tr>
    <?php endforeach; ?>
    <?php if (empty($list)): ?><tr><td colspan="5" style="color:var(--text-mute);text-align:center;padding:24px">No servers yet</td></tr><?php endif; ?>
    </tbody>
  </table>
</div>
<?php require __DIR__ . '/partials/footer.php'; ?>
