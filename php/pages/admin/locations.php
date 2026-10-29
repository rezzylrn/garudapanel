<?php
// Admin — Locations
$page_title = 'Locations';
$active_nav = 'locations';

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['create'])) {
    $r = gp_api('POST', '/api/admin/locations', [
        'short_code' => $_POST['short_code'] ?? '',
        'description' => $_POST['description'] ?? '',
    ], gp_token());
    if ($r['ok']) { $okMsg = 'Location created.'; } else { $error = $r['error']; }
}

$locs = gp_api('GET', '/api/admin/locations', null, gp_token());
$locList = $locs['data'] ?? [];
require __DIR__ . '/../../partials/header.php';
?>

<div class="page-head"><div><h1><?= icon('location', 20) ?> Locations</h1></div></div>
<?php if (!empty($okMsg)): ?><div class="alert ok"><?= icon('check', 16) ?> <?= e($okMsg) ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= icon('x', 16) ?> <?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3><?= icon('plus', 15) ?> New location</h3>
  <form method="POST">
    <div class="form-grid">
      <div class="field"><label>Short code</label><input type="text" name="short_code" required placeholder="eu-west"></div>
      <div class="field"><label>Description</label><input type="text" name="description" placeholder="Europe West"></div>
    </div>
    <button type="submit" name="create" class="btn primary" style="margin-top:12px"><?= icon('plus', 15) ?> Create</button>
  </form>
</div>

<div class="card">
  <?php if (empty($locList)): ?>
  <div class="empty"><?= icon('location', 32) ?><p>No locations yet</p></div>
  <?php else: ?>
  <div class="table-wrap"><table>
    <thead><tr><th>Code</th><th>Description</th></tr></thead>
    <tbody>
    <?php foreach ($locList as $l): ?>
      <tr><td class="mono"><span class="cell-main"><?= e($l['short_code']) ?></span></td><td><?= e($l['description'] ?? '') ?></td></tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
