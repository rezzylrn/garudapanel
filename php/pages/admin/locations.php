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

<div class="page-head"><div><h1>Locations</h1></div></div>
<?php if (!empty($okMsg)): ?><div class="alert ok"><?= e($okMsg) ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3>New location</h3>
  <form method="POST" style="display:grid;grid-template-columns:200px 1fr auto;gap:12px;align-items:end">
    <div class="field" style="margin:0"><label>Short code</label><input type="text" name="short_code" required placeholder="eu-west"></div>
    <div class="field" style="margin:0"><label>Description</label><input type="text" name="description" placeholder="Europe West"></div>
    <button type="submit" name="create" class="btn primary">Create</button>
  </form>
</div>

<div class="card">
  <table>
    <thead><tr><th>Code</th><th>Description</th></tr></thead>
    <tbody>
    <?php foreach ($locList as $l): ?>
      <tr><td class="mono"><strong><?= e($l['short_code']) ?></strong></td><td><?= e($l['description'] ?? '') ?></td></tr>
    <?php endforeach; ?>
    <?php if (empty($locList)): ?><tr><td colspan="2" style="color:var(--text-mute);text-align:center">No locations yet</td></tr><?php endif; ?>
    </tbody>
  </table>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
