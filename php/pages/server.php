<?php
// Server detail page with tabs
$id = (int)($_GET['id'] ?? 0);
$tab = $_GET['tab'] ?? 'console';
if ($id <= 0) { die('Invalid server'); }

$r = gp_api('GET', "/api/servers/$id", null, gp_token());
if (!$r['ok']) { die('Server not found'); }
$s = $r['data'];

$page_title = $s['name'];
$active_nav = 'servers';
require __DIR__ . '/../partials/header.php';

$st = $s['status'] ?? 'offline';
$badgeCls = ['running' => 'green', 'starting' => 'yellow', 'stopping' => 'yellow'][$st] ?? 'gray';
?>

<div class="page-head">
  <div>
    <h1><?= e($s['name']) ?> <span class="badge <?= $badgeCls ?>"><?= e($st) ?></span></h1>
    <p class="mono"><?= e($s['uuid'] ?? '') ?> · <?= e($s['node_name'] ?? '') ?> · <?= (int)($s['memory_mb'] ?? 0) ?> MB RAM</p>
  </div>
  <div style="display:flex;gap:8px">
    <button class="btn small" hx-post="/?p=server&id=<?= $id ?>&action=start" hx-swap="none">Start</button>
    <button class="btn small" hx-post="/?p=server&id=<?= $id ?>&action=restart" hx-swap="none">Restart</button>
    <button class="btn small danger" hx-post="/?p=server&id=<?= $id ?>&action=stop" hx-swap="none">Stop</button>
  </div>
</div>

<?php
// Handle power actions via HTMX
if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_GET['action'])) {
    $action = $_GET['action'];
    if (in_array($action, ['start', 'stop', 'restart', 'kill'], true)) {
        gp_api('POST', "/api/servers/$id/power", ['action' => $action], gp_token());
    }
    exit;
}
?>

<div class="tabs">
  <a href="/?p=server&id=<?= $id ?>&tab=console" class="<?= $tab === 'console' ? 'active' : '' ?>">Console</a>
  <a href="/?p=server&id=<?= $id ?>&tab=files" class="<?= $tab === 'files' ? 'active' : '' ?>">Files</a>
  <a href="/?p=server&id=<?= $id ?>&tab=startup" class="<?= $tab === 'startup' ? 'active' : '' ?>">Startup</a>
</div>

<?php if ($tab === 'console'): ?>
<?php require __DIR__ . '/server_console.php'; ?>
<?php elseif ($tab === 'files'): ?>
<?php require __DIR__ . '/server_files.php'; ?>
<?php elseif ($tab === 'startup'): ?>
<?php require __DIR__ . '/server_startup.php'; ?>
<?php endif; ?>

<?php require __DIR__ . '/../partials/footer.php'; ?>
