<?php
// Layout header + icon sidebar + mobile topbar. Expects $page_title, $active_nav.
require_once __DIR__ . '/../lib/icons.php';
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
<div class="topbar">
  <button class="icon-btn" onclick="document.querySelector('.sidebar').classList.toggle('open');document.querySelector('.sidebar-backdrop').classList.toggle('show')" aria-label="Menu"><?= icon('menu', 20) ?></button>
  <div class="brand"><?= icon('layers', 20) ?> Garuda<span>Panel</span></div>
  <span style="width:36px"></span>
</div>
<div class="sidebar-backdrop" onclick="document.querySelector('.sidebar').classList.remove('open');this.classList.remove('show')"></div>
<div class="app">
<aside class="sidebar">
  <div class="brand"><?= icon('layers', 20) ?> Garuda<span>Panel</span></div>
  <nav class="nav">
    <a href="/?p=dashboard" class="<?= ($active_nav ?? '') === 'dashboard' ? 'active' : '' ?>"><?= icon('dashboard', 18) ?> Dashboard</a>
    <a href="/?p=servers" class="<?= ($active_nav ?? '') === 'servers' ? 'active' : '' ?>"><?= icon('server', 18) ?> Servers</a>
    <?php if (gp_is_admin()): ?>
    <div class="section">Administration</div>
    <a href="/?p=admin/nodes" class="<?= ($active_nav ?? '') === 'nodes' ? 'active' : '' ?>"><?= icon('nodes', 18) ?> Nodes</a>
    <a href="/?p=admin/servers" class="<?= ($active_nav ?? '') === 'admin_servers' ? 'active' : '' ?>"><?= icon('cpu', 18) ?> All Servers</a>
    <a href="/?p=admin/eggs" class="<?= ($active_nav ?? '') === 'eggs' ? 'active' : '' ?>"><?= icon('egg', 18) ?> Eggs</a>
    <a href="/?p=admin/users" class="<?= ($active_nav ?? '') === 'users' ? 'active' : '' ?>"><?= icon('users', 18) ?> Users</a>
    <a href="/?p=admin/locations" class="<?= ($active_nav ?? '') === 'locations' ? 'active' : '' ?>"><?= icon('location', 18) ?> Locations</a>
    <?php endif; ?>
    <div class="nav-footer" style="margin-top:auto">
      <a href="/?p=logout"><?= icon('logout', 18) ?> <?= e($user['username'] ?? 'user') ?></a>
    </div>
  </nav>
</aside>
<main class="main">
