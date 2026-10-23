<?php
// Login page
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $r = gp_api('POST', '/api/auth/login', [
        'username' => $_POST['username'] ?? '',
        'password' => $_POST['password'] ?? '',
    ]);
    if ($r['ok']) {
        $_SESSION['gp_token'] = $r['data']['token'];
        $_SESSION['gp_user'] = ['username' => $_POST['username'], 'is_admin' => $r['data']['is_admin'] ?? false];
        $_SESSION['gp_validated'] = time();
        header('Location: /');
        exit;
    }
    $error = $r['error'] ?? 'Login failed';
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Login — GarudaPanel</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body>
<div class="login-wrap">
  <div class="card login-card">
    <div class="brand">Garuda<span>Panel</span></div>
    <?php if (!empty($error)): ?>
    <div class="alert error"><?= e($error) ?></div>
    <?php endif; ?>
    <form method="POST">
      <div class="field">
        <label>Username</label>
        <input type="text" name="username" required autofocus autocomplete="username">
      </div>
      <div class="field">
        <label>Password</label>
        <input type="password" name="password" required autocomplete="current-password">
      </div>
      <button type="submit" class="btn primary" style="width:100%">Sign in</button>
    </form>
  </div>
</div>
</body>
</html>
