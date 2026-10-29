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

// Route table — one file per page, no page crams multiple sections
$routes = [
    'dashboard'       => 'pages/dashboard.php',
    'servers'         => 'pages/servers.php',
    'server/console'  => 'pages/server_console_page.php',
    'server/files'    => 'pages/server_files_page.php',
    'server/startup'  => 'pages/server_startup_page.php',
    'admin/nodes'     => 'pages/admin/nodes.php',
    'admin/servers'   => 'pages/admin/servers.php',
    'admin/nests'     => 'pages/admin/nests.php',
    'admin/eggs'      => 'pages/admin/eggs.php',
    'admin/users'     => 'pages/admin/users.php',
    'admin/locations' => 'pages/admin/locations.php',
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
