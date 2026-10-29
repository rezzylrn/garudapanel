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

<div class="page-head"><div><h1><?= icon('users', 20) ?> Users</h1></div></div>
<?php if (!empty($okMsg)): ?><div class="alert ok"><?= icon('check', 16) ?> <?= e($okMsg) ?></div><?php endif; ?>
<?php if (!empty($error)): ?><div class="alert error"><?= icon('x', 16) ?> <?= e($error) ?></div><?php endif; ?>

<div class="card" style="margin-bottom:16px">
  <h3><?= icon('plus', 15) ?> New user</h3>
  <form method="POST">
    <div class="form-grid">
      <div class="field"><label>Username</label><input type="text" name="username" required></div>
      <div class="field"><label>Email</label><input type="email" name="email" required></div>
      <div class="field"><label>Password</label><input type="password" name="password" required></div>
    </div>
    <div style="display:flex;gap:12px;align-items:center;margin-top:12px;flex-wrap:wrap">
      <label style="font-size:13px;display:flex;gap:6px;align-items:center;cursor:pointer"><input type="checkbox" name="is_admin" style="width:auto"> Admin</label>
      <button type="submit" name="create" class="btn primary"><?= icon('plus', 15) ?> Create</button>
    </div>
  </form>
</div>

<div class="card">
  <?php if (empty($userList)): ?>
  <div class="empty"><?= icon('users', 32) ?><p>No users</p></div>
  <?php else: ?>
  <div class="table-wrap"><table>
    <thead><tr><th>Username</th><th>Email</th><th>Role</th></tr></thead>
    <tbody>
    <?php foreach ($userList as $u): ?>
      <tr>
        <td><span class="cell-main"><?= e($u['username']) ?></span></td>
        <td><?= e($u['email'] ?? '') ?></td>
        <td><?= !empty($u['is_admin']) ? '<span class="badge blue">admin</span>' : '<span class="badge gray">user</span>' ?></td>
      </tr>
    <?php endforeach; ?>
    </tbody>
  </table></div>
  <?php endif; ?>
</div>

<?php require __DIR__ . '/../../partials/footer.php'; ?>
