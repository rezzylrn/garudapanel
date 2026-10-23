<?php
// Layout header + sidebar. Expects $page_title, $active_nav.
$user = gp_user();
?>
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title><?= e($page_title ?? 'GarudaPanel') ?> — GarudaPanel</title>
<link rel="stylesheet" href="/assets/style.css">
<script src="https://unpkg.com/htmx.org@1.9.10"></script>
</head>
<body>
<div class="app">
<aside class="sidebar">
  <div class="brand">Garuda<span>Panel</span></div>
  <nav class="nav">
    <a href="/?p=dashboard" class="<?= ($active_nav ?? '') === 'dashboard' ? 'active' : '' ?>">Dashboard</a>
    <a href="/?p=servers" class="<?= ($active_nav ?? '') === 'servers' ? 'active' : '' ?>">Servers</a>
    <?php if (gp_is_admin()): ?>
    <div class="section">Administration</div>
    <a href="/?p=admin/nodes" class="<?= ($active_nav ?? '') === 'nodes' ? 'active' : '' ?>">Nodes</a>
    <a href="/?p=admin/servers" class="<?= ($active_nav ?? '') === 'admin_servers' ? 'active' : '' ?>">All Servers</a>
    <a href="/?p=admin/eggs" class="<?= ($active_nav ?? '') === 'eggs' ? 'active' : '' ?>">Eggs</a>
    <a href="/?p=admin/users" class="<?= ($active_nav ?? '') === 'users' ? 'active' : '' ?>">Users</a>
    <a href="/?p=admin/locations" class="<?= ($active_nav ?? '') === 'locations' ? 'active' : '' ?>">Locations</a>
    <?php endif; ?>
    <div class="section">Account</div>
    <a href="/?p=logout"><?= e($user['username'] ?? 'user') ?> — Logout</a>
  </nav>
</aside>
<main class="main">
