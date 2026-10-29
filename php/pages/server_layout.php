<?php
// Shared loader for server sub-pages. Sets $s (server), $id, $st, $badgeCls.
// Renders page head + power actions + subnav. Call gp_server_head($activeTab) after require.

// Returns server array or dies.
function gp_load_server(int $id): array {
    $r = gp_api('GET', "/api/servers/$id", null, gp_token());
    if (!$r['ok']) { http_response_code(404); die('Server not found'); }
    return $r['data'];
}

function gp_status_badge(string $st): string {
    $cls = ['running' => 'green', 'starting' => 'yellow', 'stopping' => 'yellow'][$st] ?? 'gray';
    return '<span class="badge ' . $cls . '"><span class="dot"></span>' . e($st) . '</span>';
}

function gp_server_head(array $s, int $id, string $activeTab): void {
    $st = $s['status'] ?? 'offline';
    $page_title = $s['name'];
    $active_nav = 'servers';
    require __DIR__ . '/../partials/header.php';
    ?>
    <div class="page-head">
      <div>
        <h1><?= icon('server', 20) ?> <?= e($s['name']) ?> <?= gp_status_badge($st) ?></h1>
        <p class="mono"><?= e($s['uuid'] ?? '') ?> · <?= e($s['node_name'] ?? '') ?> · <?= (int)($s['memory_mb'] ?? 0) ?> MB</p>
      </div>
      <div class="page-actions">
        <button class="btn small" hx-post="/?p=server/console&id=<?= $id ?>&action=start" hx-swap="none"><?= icon('play', 14) ?> Start</button>
        <button class="btn small" hx-post="/?p=server/console&id=<?= $id ?>&action=restart" hx-swap="none"><?= icon('restart', 14) ?> Restart</button>
        <button class="btn small danger" hx-post="/?p=server/console&id=<?= $id ?>&action=stop" hx-swap="none"><?= icon('stop', 14) ?> Stop</button>
      </div>
    </div>
    <nav class="subnav">
      <a href="/?p=server/console&id=<?= $id ?>" class="<?= $activeTab === 'console' ? 'active' : '' ?>"><?= icon('terminal', 16) ?> Console</a>
      <a href="/?p=server/files&id=<?= $id ?>" class="<?= $activeTab === 'files' ? 'active' : '' ?>"><?= icon('folder', 16) ?> Files</a>
      <a href="/?p=server/startup&id=<?= $id ?>" class="<?= $activeTab === 'startup' ? 'active' : '' ?>"><?= icon('settings', 16) ?> Startup</a>
    </nav>
    <?php
    // Handle power actions via HTMX (posted to any sub-page)
    if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_GET['action'])) {
        $action = $_GET['action'];
        if (in_array($action, ['start', 'stop', 'restart', 'kill'], true)) {
            gp_api('POST', "/api/servers/$id/power", ['action' => $action], gp_token());
        }
        exit;
    }
}
