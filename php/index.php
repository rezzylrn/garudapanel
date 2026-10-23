<?php
// GarudaPanel PHP frontend — front controller
session_start();
require_once __DIR__ . '/config.php';
require_once __DIR__ . '/lib/api.php';

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

// Route
$routes = [
    'dashboard'       => 'pages/dashboard.php',
    'servers'         => 'pages/servers.php',
    'server'          => 'pages/server.php',
    'admin/nodes'     => 'pages/admin/nodes.php',
    'admin/servers'   => 'pages/admin/servers.php',
    'admin/eggs'      => 'pages/admin/eggs.php',
    'admin/users'     => 'pages/admin/users.php',
    'admin/locations' => 'pages/admin/locations.php',
];

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
