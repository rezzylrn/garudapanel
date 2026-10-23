<?php
// Admin — Eggs (list + import egg.json)
$page_title = 'Eggs';
$active_nav = 'eggs';

// Handle egg.json import
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
  <div><h1>Eggs</h1><p>Server templates (Pterodactyl-compatible)</p></div>
</div>

<?php if (!empty($okMsg)): ?><div class="alert ok"><?= $okMsg ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3>Import egg.json</h3>
  <form method="POST" enctype="multipart/form-data" style="display:flex;gap:12px;align-items:end">
    <div class="field" style="margin:0"><label>Nest</label>
      <select name="nest_id" required>
        <?php foreach ($nestList as $n): ?><option value="<?= $n['id'] ?>"><?= e($n['name']) ?></option><?php endforeach; ?>
      </select>
    </div>
    <div class="field" style="margin:0"><label>egg.json file</label><input type="file" name="eggfile" accept=".json" required></div>
    <button type="submit" class="btn primary">Import</button>
  </form>
  <p style="color:var(--text-mute);font-size:12px;margin-top:10px">Upload a Pterodactyl egg export (PTDL_v2). Variables are imported automatically.</p>
</div>

<div class="card">
  <table>
    <thead><tr><th>Name</th><th>Author</th><th>Image</th><th>Stop</th></tr></thead>
    <tbody>
    <?php foreach ($eggList as $eg): ?>
      <?php $imgs = $eg['docker_images'] ?? []; $first = is_array($imgs) ? reset($imgs) : ''; ?>
      <tr>
        <td><strong><?= e($eg['name']) ?></strong><br><span style="color:var(--text-mute);font-size:12px"><?= e($eg['description'] ?? '') ?></span></td>
        <td style="color:var(--text-dim)"><?= e($eg['author'] ?? '-') ?></td>
        <td class="mono" style="max-width:280px;overflow:hidden;text-overflow:ellipsis"><?= e($first) ?></td>
        <td class="mono"><?= e($eg['config_stop'] ?? '^C') ?></td>
      </tr>
    <?php endforeach; ?>
    <?php if (empty($eggList)): ?><tr><td colspan="4" style="color:var(--text-mute);text-align:center">No eggs yet — import one above</td></tr><?php endif; ?>
    </tbody>
  </table>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
