<?php
// Startup tab — egg startup command + variables
$vars = gp_api('GET', "/api/servers/$id/variables", null, gp_token());
$varList = $vars['data'] ?? [];

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['var_name'])) {
    gp_api('PATCH', "/api/servers/$id/variables/" . urlencode($_POST['var_name']), ['value' => $_POST['var_value'] ?? ''], gp_token());
    header("Location: /?p=server&id=$id&tab=startup");
    exit;
}
?>
<div class="card">
  <h3>Startup command</h3>
  <p class="mono" style="background:var(--bg);padding:12px;border-radius:8px;border:1px solid var(--border-soft)"><?= e($s['startup'] ?? $s['egg_startup'] ?? '-') ?></p>
</div>
<div class="card">
  <h3>Variables</h3>
  <?php if (empty($varList)): ?>
  <p style="color:var(--text-mute)">No variables for this egg.</p>
  <?php else: ?>
  <table>
    <thead><tr><th>Variable</th><th>Value</th><th></th></tr></thead>
    <tbody>
    <?php foreach ($varList as $v): ?>
      <tr>
        <td><strong class="mono"><?= e($v['name']) ?></strong><br><span style="color:var(--text-mute);font-size:12px"><?= e($v['description'] ?? '') ?></span></td>
        <td>
          <?php if (!empty($v['user_editable'])): ?>
          <form method="POST" style="display:flex;gap:8px">
            <input type="hidden" name="var_name" value="<?= e($v['name']) ?>">
            <input type="text" name="var_value" value="<?= e($v['value'] ?? $v['default_value'] ?? '') ?>" style="padding:6px 10px;border-radius:8px;border:1px solid var(--border);background:var(--bg);color:var(--text);font-size:13px;flex:1">
            <button class="btn small" type="submit">Save</button>
          </form>
          <?php else: ?>
          <span class="mono"><?= e($v['value'] ?? $v['default_value'] ?? '') ?></span>
          <?php endif; ?>
        </td>
        <td></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table>
  <?php endif; ?>
</div>
