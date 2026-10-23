<?php
// Admin — Users
$page_title = 'Users';
$active_nav = 'users';

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['create'])) {
    $r = gp_api('POST', '/api/admin/users', [
        'username' => $_POST['username'] ?? '',
        'email' => $_POST['email'] ?? '',
        'password' => $_POST['password'] ?? '',
        'is_admin' => isset($_POST['is_admin']),
    ], gp_token());
    if ($r['ok']) { $okMsg = 'User created.'; } else { $error = $r['error']; }
}

$users = gp_api('GET', '/api/admin/users', null, gp_token());
$userList = $users['data'] ?? [];
require __DIR__ . '/../../partials/header.php';
?>

<div class="page-head"><div><h1>Users</h1></div></div>
<?php if (!empty($okMsg)): ?><div class="alert ok"><?= e($okMsg) ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3>New user</h3>
  <form method="POST" style="display:grid;grid-template-columns:1fr 1fr 1fr auto auto;gap:12px;align-items:end">
    <div class="field" style="margin:0"><label>Username</label><input type="text" name="username" required></div>
    <div class="field" style="margin:0"><label>Email</label><input type="email" name="email" required></div>
    <div class="field" style="margin:0"><label>Password</label><input type="password" name="password" required></div>
    <div class="field" style="margin:0"><label>&nbsp;</label><label style="text-transform:none"><input type="checkbox" name="is_admin" style="width:auto"> Admin</label></div>
    <button type="submit" name="create" class="btn primary">Create</button>
  </form>
</div>

<div class="card">
  <table>
    <thead><tr><th>Username</th><th>Email</th><th>Role</th></tr></thead>
    <tbody>
    <?php foreach ($userList as $u): ?>
      <tr>
        <td><strong><?= e($u['username']) ?></strong></td>
        <td style="color:var(--text-dim)"><?= e($u['email'] ?? '') ?></td>
        <td><?= !empty($u['is_admin']) ? '<span class="badge blue">admin</span>' : '<span class="badge gray">user</span>' ?></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
