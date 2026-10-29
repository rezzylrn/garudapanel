<?php
// Admin — Nests (dedicated page: list + create, separate from Eggs)
$page_title = 'Nests';
$active_nav = 'nests';

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['create'])) {
    $r = gp_api('POST', '/api/admin/nests', [
        'name' => $_POST['name'] ?? '',
    ], gp_token());
    if ($r['ok']) { $okMsg = 'Nest created.'; } else { $error = $r['error']; }
}

$nests = gp_api('GET', '/api/admin/nests', null, gp_token());
$nestList = $nests['data'] ?? [];
require __DIR__ . '/../../partials/header.php';
?>

<div class="page-head">
  <div><h1><?= icon('layers', 20) ?> Nests</h1><p>Egg categories — create a nest first, then add eggs to it</p></div>
</div>
<?php if (!empty($okMsg)): ?><div class="alert ok"><?= icon('check', 16) ?> <?= e($okMsg) ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= icon('x', 16) ?> <?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3><?= icon('plus', 15) ?> New nest</h3>
  <form method="POST">
    <div class="form-grid">
      <div class="field"><label>Name</label><input type="text" name="name" required placeholder="Minecraft"></div>
    </div>
    <button type="submit" name="create" class="btn primary" style="margin-top:12px"><?= icon('plus', 15) ?> Create</button>
  </form>
</div>

<div class="card">
  <?php if (empty($nestList)): ?>
  <div class="empty"><?= icon('layers', 32) ?><p>No nests yet — create one above, then add eggs under it.</p></div>
  <?php else: ?>
  <div class="table-wrap"><table>
    <thead><tr><th>Name</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($nestList as $n): ?>
      <tr>
        <td><span class="cell-main"><?= e($n['name']) ?></span></td>
        <td><div class="row-actions"><a class="act-btn" title="View eggs in this nest" href="/?p=admin/eggs"><?= icon('chevron', 16) ?></a></div></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
