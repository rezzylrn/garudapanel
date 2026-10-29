<?php
// Server — Files page (standalone, multi-page)
require_once __DIR__ . '/server_layout.php';
$id = (int)($_GET['id'] ?? 0);
if ($id <= 0) { die('Invalid server'); }
$s = gp_load_server($id);
$path = $_GET['path'] ?? '';

// --- Actions (POST) ---
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    if (isset($_GET['mkdir'])) {
        $dir = $path === '' ? ($_POST['dirname'] ?? '') : $path . '/' . ($_POST['dirname'] ?? '');
        gp_api('POST', "/api/servers/$id/files/mkdir", ['path' => $dir], gp_token());
        header("Location: /?p=server/files&id=$id&path=" . urlencode($path));
        exit;
    }
    if (isset($_GET['upload']) && isset($_FILES['file'])) {
        $tmp = $_FILES['file']['tmp_name'];
        $name = basename($_FILES['file']['name']);
        $target = $path === '' ? $name : $path . '/' . $name;
        $ch = curl_init(GP_API_URL . "/api/servers/$id/files?path=" . urlencode($target));
        curl_setopt_array($ch, [
            CURLOPT_CUSTOMREQUEST => 'PUT',
            CURLOPT_HTTPHEADER => ['Authorization: Bearer ' . gp_token(), 'Content-Type: application/octet-stream'],
            CURLOPT_INFILE => fopen($tmp, 'r'),
            CURLOPT_INFILESIZE => filesize($tmp),
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_TIMEOUT => 120,
        ]);
        curl_exec($ch);
        // curl_close() omitted — deprecated in PHP 8.5, handle freed automatically.
        header("Location: /?p=server/files&id=$id&path=" . urlencode($path));
        exit;
    }
    if (isset($_GET['delete'])) {
        gp_api('DELETE', "/api/servers/$id/files?path=" . urlencode($_GET['delete']), null, gp_token());
        header("Location: /?p=server/files&id=$id&path=" . urlencode($path));
        exit;
    }
}

// --- HTMX partial: file list only ---
if (isset($_GET['partial'])) {
    $r = gp_api('GET', "/api/servers/$id/files?path=" . urlencode($path), null, gp_token());
    if (!$r['ok']) {
        echo '<div class="alert error">' . icon('x', 16) . e($r['error'] ?? 'Failed to load files') . '</div>';
        exit;
    }
    $entries = $r['data']['entries'] ?? [];
    foreach ($entries as &$en) { $en['_is_dir'] = ($en['mode'] ?? '') === 'd'; }
    unset($en);
    usort($entries, fn($a, $b) => ($b['_is_dir'] <=> $a['_is_dir']) ?: strcmp($a['name'], $b['name']));
    if (empty($entries) && $path === '') {
        echo '<div class="empty">' . icon('folder', 32) . '<p>No files yet</p></div>';
        exit;
    }
    ?>
    <div class="table-wrap"><table>
      <thead><tr><th>Name</th><th>Size</th><th>Modified</th><th></th></tr></thead>
      <tbody>
      <?php if ($path !== ''): ?>
        <tr><td colspan="4"><a href="#" hx-get="/?p=server/files&id=<?= $id ?>&partial=1&path=<?= urlencode(dirname($path) === '.' ? '' : dirname($path)) ?>" hx-target="#file-list" style="color:var(--text-dim);text-decoration:none"><?= icon('back', 14) ?> Parent directory</a></td></tr>
      <?php endif; ?>
      <?php foreach ($entries as $en): ?>
        <?php $full = $path === '' ? $en['name'] : $path . '/' . $en['name']; ?>
        <tr>
          <td>
            <?php if ($en['_is_dir']): ?>
              <a href="#" hx-get="/?p=server/files&id=<?= $id ?>&partial=1&path=<?= urlencode($full) ?>" hx-target="#file-list" style="text-decoration:none"><?= icon('folder', 15) ?> <span class="cell-main"><?= e($en['name']) ?></span></a>
            <?php else: ?>
              <?= icon('file', 15) ?> <?= e($en['name']) ?>
            <?php endif; ?>
          </td>
          <td class="mono"><?= $en['_is_dir'] ? '—' : number_format($en['size'] ?? 0) . ' B' ?></td>
          <td class="mono cell-sub"><?= !empty($en['mod_time']) ? date('Y-m-d H:i', (int)$en['mod_time']) : '—' ?></td>
          <td><div class="row-actions">
            <?php if (!$en['_is_dir']): ?>
              <a class="act-btn" title="Download" href="<?= GP_API_URL ?>/api/servers/<?= $id ?>/files/content?path=<?= urlencode($full) ?>&token=<?= urlencode(gp_token()) ?>"><?= icon('download', 15) ?></a>
            <?php endif; ?>
            <a class="act-btn danger" title="Delete" href="/?p=server/files&id=<?= $id ?>&path=<?= urlencode($path) ?>&delete=<?= urlencode($full) ?>" onclick="return confirm('Delete <?= e($en['name']) ?>?')"><?= icon('trash', 15) ?></a>
          </div></td>
        </tr>
      <?php endforeach; ?>
      </tbody>
    </table></div>
    <?php
    exit;
}

gp_server_head($s, $id, 'files');
?>

<div class="card">
  <div class="files-toolbar">
    <span class="mono crumb-path" style="color:var(--text-dim)"><?= icon('folder', 14) ?> /<?= e($path) ?></span>
    <div class="tools">
      <form method="POST" action="/?p=server/files&id=<?= $id ?>&path=<?= urlencode($path) ?>&mkdir=1" style="display:flex;gap:8px">
        <input type="text" name="dirname" placeholder="New folder" required style="padding:7px 10px;border-radius:8px;border:1px solid var(--border);background:var(--bg);color:var(--text);font-size:12px">
        <button class="btn small" type="submit"><?= icon('plus', 14) ?> Folder</button>
      </form>
      <form action="/?p=server/files&id=<?= $id ?>&path=<?= urlencode($path) ?>&upload=1" method="POST" enctype="multipart/form-data" style="display:flex;gap:8px;align-items:center">
        <input type="file" name="file" required style="font-size:12px;color:var(--text-dim);max-width:200px">
        <button class="btn small" type="submit"><?= icon('upload', 14) ?> Upload</button>
      </form>
    </div>
  </div>
  <div id="file-list" hx-get="/?p=server/files&id=<?= $id ?>&partial=1&path=<?= urlencode($path) ?>" hx-trigger="load">
    <p style="color:var(--text-mute)">Loading…</p>
  </div>
</div>

<?php require __DIR__ . '/../partials/footer.php'; ?>
