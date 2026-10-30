<?php
// Server — Startup page (standalone, multi-page)
require_once __DIR__ . '/server_layout.php';
$id = (int)($_GET['id'] ?? 0);
if ($id <= 0) { die('Invalid server'); }
$s = gp_load_server($id);

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['var_name'])) {
    gp_api('PATCH', "/api/servers/$id/variables/" . urlencode($_POST['var_name']), ['value' => $_POST['var_value'] ?? ''], gp_token());
    header("Location: /?p=server/startup&id=$id");
    exit;
}

$vars = gp_api('GET', "/api/servers/$id/variables", null, gp_token());
$varList = $vars['data']['variables'] ?? [];

gp_server_head($s, $id, 'startup');
?>

<div class="card">
  <h3><?= icon('terminal', 15) ?> Startup command</h3>
  <p class="mono" style="background:var(--bg);padding:12px;border-radius:8px;border:1px solid var(--border-soft);white-space:pre-wrap;overflow-wrap:anywhere"><?= e(!empty($s['startup_cmd']) ? $s['startup_cmd'] : '-') ?></p>
</div>

<div class="card">
  <h3><?= icon('settings', 15) ?> Variables</h3>
  <?php if (empty($varList)): ?>
    <div class="empty"><?= icon('settings', 28) ?><p>No variables for this egg.</p></div>
  <?php else: ?>
    <div class="var-list">
      <?php foreach ($varList as $v): ?>
        <div class="var-item">
          <div class="var-info">
            <span class="mono var-name"><?= e($v['name']) ?></span>
            <?php if (!empty($v['description'])): ?><span class="var-desc"><?= e($v['description']) ?></span><?php endif; ?>
          </div>
          <div class="var-control">
            <?php if (!empty($v['user_editable'])): ?>
            <form method="POST">
              <input type="hidden" name="var_name" value="<?= e($v['name']) ?>">
              <input type="text" name="var_value" value="<?= e($v['value'] ?? $v['default_value'] ?? '') ?>">
              <button class="btn small" type="submit"><?= icon('check', 14) ?> Save</button>
            </form>
            <?php else: ?>
            <span class="mono var-value"><?= e($v['value'] ?? $v['default_value'] ?? '') ?></span>
            <?php endif; ?>
          </div>
        </div>
      <?php endforeach; ?>
    </div>
  <?php endif; ?>
</div>

<?php require __DIR__ . '/../partials/footer.php'; ?>
