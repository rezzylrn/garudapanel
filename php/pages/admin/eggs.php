<?php
// Admin — Eggs (list + import egg.json)
$page_title = 'Eggs';
$active_nav = 'eggs';

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_FILES['eggfile'])) {
    $nestId = (int)($_POST['nest_id'] ?? 0);
    $content = file_get_contents($_FILES['eggfile']['tmp_name']);
    $egg = json_decode($content, true);
    if (!$egg) {
        $error = 'File is not valid JSON';
    } else {
        $r = gp_api('POST', '/api/admin/eggs/import', ['nest_id' => $nestId, 'egg' => $egg], gp_token());
        if ($r['ok']) {
            $okMsg = 'Imported "' . e($r['data']['name'] ?? '') . '" with ' . ($r['data']['variables'] ?? 0) . ' variables.';
        } else {
            $error = $r['error'];
        }
    }
}

$eggs = gp_api('GET', '/api/admin/eggs', null, gp_token());
$nests = gp_api('GET', '/api/admin/nests', null, gp_token());
$eggList = $eggs['data'] ?? [];
$nestList = $nests['data'] ?? [];

require __DIR__ . '/../../partials/header.php';
?>

<div class="page-head">
  <div><h1><?= icon('egg', 20) ?> Eggs</h1><p>Server templates (Pterodactyl-compatible)</p></div>
</div>

<?php if (!empty($okMsg)): ?><div class="alert ok"><?= icon('check', 16) ?> <?= $okMsg ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= icon('x', 16) ?> <?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3><?= icon('upload', 15) ?> Import egg.json</h3>
  <form method="POST" enctype="multipart/form-data">
    <div class="form-grid">
      <div class="field"><label>Nest</label>
        <select name="nest_id" required>
          <?php foreach ($nestList as $n): ?><option value="<?= $n['id'] ?>"><?= e($n['name']) ?></option><?php endforeach; ?>
        </select>
      </div>
      <div class="field"><label>egg.json file</label><input type="file" name="eggfile" accept=".json" required></div>
    </div>
    <button type="submit" class="btn primary" style="margin-top:12px"><?= icon('upload', 15) ?> Import</button>
  </form>
  <p style="color:var(--text-mute);font-size:12px;margin-top:10px">Upload a Pterodactyl egg export (PTDL_v2). Variables are imported automatically.</p>
</div>

<div class="card">
  <?php if (empty($eggList)): ?>
  <div class="empty"><?= icon('egg', 32) ?><p>No eggs yet — import one above</p></div>
  <?php else: ?>
  <div class="table-wrap"><table>
    <thead><tr><th>Name</th><th>Author</th><th>Image</th><th>Stop</th></tr></thead>
    <tbody>
    <?php foreach ($eggList as $eg): ?>
      <?php $imgs = $eg['docker_images'] ?? []; $first = is_array($imgs) ? reset($imgs) : ''; ?>
      <tr>
        <td><span class="cell-main"><?= e($eg['name']) ?></span><br><span class="cell-sub"><?= e($eg['description'] ?? '') ?></span></td>
        <td><?= e($eg['author'] ?? '-') ?></td>
        <td class="mono" style="max-width:280px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap"><?= e($first) ?></td>
        <td class="mono"><?= e($eg['config_stop'] ?? '^C') ?></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
