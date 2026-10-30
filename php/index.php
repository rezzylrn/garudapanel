<?php
// GarudaPanel PHP frontend — front controller (multi-page)

// php -S router: serve static files (assets/) directly instead of routing them
if (php_sapi_name() === 'cli-server') {
    $uriPath = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH);
    $staticFile = __DIR__ . $uriPath;
    if ($uriPath !== '/' && is_file($staticFile)) {
        return false;
    }
}

session_start();
require_once __DIR__ . '/config.php';
require_once __DIR__ . '/lib/api.php';
require_once __DIR__ . '/lib/icons.php';

$page = $_GET['p'] ?? 'dashboard';

// Public pages
if ($page === 'login') {
    require __DIR__ . '/pages/login.php';
    exit;
}
if ($page === 'logout') {
    session_destroy();
    header('Location: /?p=login');
    exit;
}

// Require auth
if (!gp_token()) {
    header('Location: /?p=login');
    exit;
}

// Validate token still works (light check, cached per session)
if (!isset($_SESSION['gp_validated'])) {
    $r = gp_api('GET', '/api/auth/me', null, gp_token());
    if (!$r['ok']) {
        session_destroy();
        header('Location: /?p=login');
        exit;
    }
    $_SESSION['gp_user'] = $r['data'];
    $_SESSION['gp_validated'] = time();
}

// JSON status endpoint untuk polling power-state di halaman server
// (dipakai JS konfirmasi power action; token sesi tidak pernah ke browser
// selain lewat endpoint ini).
if ($page === 'server/status') {
    $id = (int)($_GET['id'] ?? 0);
    $r = $id > 0 ? gp_api('GET', "/api/servers/$id", null, gp_token()) : ['ok' => false];
    header('Content-Type: application/json');
    if (!$r['ok']) { http_response_code(404); echo json_encode(['ok' => false]); exit; }
    echo json_encode(['ok' => true, 'status' => $r['data']['status'] ?? 'offline']);
    exit;
}

// JSON disk usage untuk strip statistik halaman console.
if ($page === 'server/usage') {
    $id = (int)($_GET['id'] ?? 0);
    $r = $id > 0 ? gp_api('GET', "/api/servers/$id/files/usage", null, gp_token()) : ['ok' => false];
    header('Content-Type: application/json');
    if (!$r['ok']) { http_response_code(404); echo json_encode(['ok' => false]); exit; }
    echo json_encode(['ok' => true, 'used' => (int)($r['data']['used'] ?? 0), 'limit' => (int)($r['data']['limit'] ?? 0)]);
    exit;
}

// JSON join token segar untuk tombol Activate node (sekali pakai, 15 menit).
if ($page === 'admin/node-join-token') {
    if (!gp_is_admin()) { http_response_code(403); echo json_encode(['ok' => false]); exit; }
    $nid = (int)($_GET['id'] ?? 0);
    $r = $nid > 0 ? gp_api('POST', "/api/admin/nodes/$nid/join-token", null, gp_token()) : ['ok' => false];
    header('Content-Type: application/json');
    if (empty($r['ok'])) { http_response_code(500); echo json_encode(['ok' => false, 'error' => $r['error'] ?? 'failed']); exit; }
    // URL panel (API Go) yang harus dijangkau daemon: tunnel WS bila ada.
    $panelUrl = '';
    $wsFile = __DIR__ . '/ws_url.txt';
    if (is_readable($wsFile)) { $panelUrl = trim((string)file_get_contents($wsFile)); }
    if ($panelUrl === '') { $panelUrl = 'http://' . ($_SERVER['HTTP_HOST'] ?? '127.0.0.1:8080'); }
    echo json_encode(['ok' => true,
        'join_token' => $r['data']['join_token'] ?? '',
        'expires_in' => (int)($r['data']['join_token_expires_in'] ?? 900),
        'panel_url' => $panelUrl]);
    exit;
}

// JSON alokasi bebas per node untuk dropdown form New server.
if ($page === 'admin/node-allocations') {
    if (!gp_is_admin()) { http_response_code(403); echo json_encode(['ok' => false]); exit; }
    $nid = (int)($_GET['node_id'] ?? 0);
    $r = $nid > 0 ? gp_api('GET', "/api/admin/allocations?node_id=$nid", null, gp_token()) : ['ok' => false, 'data' => []];
    header('Content-Type: application/json');
    echo json_encode(['ok' => (bool)($r['ok'] ?? false), 'data' => $r['data'] ?? []]);
    exit;
}

// Route table — one file per page, no page crams multiple sections
$routes = [
    'dashboard'         => 'pages/dashboard.php',
    'servers'           => 'pages/servers.php',
    'server/console'    => 'pages/server_console_page.php',
    'server/files'      => 'pages/server_files_page.php',
    'server/startup'    => 'pages/server_startup_page.php',
    'admin/nodes'       => 'pages/admin/nodes.php',
    'admin/servers'     => 'pages/admin/servers.php',
    'admin/servers/new' => 'pages/admin/servers_new.php',
    'admin/nests'       => 'pages/admin/nests.php',
    'admin/eggs'        => 'pages/admin/eggs.php',
    'admin/users'       => 'pages/admin/users.php',
    'admin/locations'   => 'pages/admin/locations.php',
];

// Legacy ?p=server&id=N&tab=X redirects to the dedicated page
if ($page === 'server') {
    $id = (int)($_GET['id'] ?? 0);
    $tab = $_GET['tab'] ?? 'console';
    $tab = in_array($tab, ['console', 'files', 'startup'], true) ? $tab : 'console';
    header("Location: /?p=server/$tab&id=$id");
    exit;
}

if (str_starts_with($page, 'admin/') && !gp_is_admin()) {
    http_response_code(403);
    die('Forbidden');
}

$file = $routes[$page] ?? null;
if (!$file || !file_exists(__DIR__ . '/' . $file)) {
    http_response_code(404);
    die('Page not found');
}

require __DIR__ . '/' . $file;
