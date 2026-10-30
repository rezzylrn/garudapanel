<?php
// Server — Files page (standalone, multi-page)
require_once __DIR__ . '/server_layout.php';
$id = (int)($_GET['id'] ?? 0);
if ($id <= 0) { die('Invalid server'); }
$s = gp_load_server($id);
$path = $_GET['path'] ?? '';

// --- Delete (GET dari tombol hapus di daftar + confirm() di browser) ---
// Sengaja di luar blok POST: link hapus di daftar file adalah GET biasa.
// Sebelumnya delete terjebak di dalam blok POST sehingga tidak pernah jalan.
if (isset($_GET['delete'])) {
    gp_api('DELETE', "/api/servers/$id/files?path=" . urlencode($_GET['delete']), null, gp_token());
    header("Location: /?p=server/files&id=$id&path=" . urlencode($path));
    exit;
}

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
}

// --- Save edited / new file (POST) ---
if ($_SERVER['REQUEST_METHOD'] === 'POST' && (isset($_GET['save']) || isset($_GET['create']))) {
    $content = $_POST['content'] ?? '';
    if (strlen($content) > 1048576) {
        die('File too large (max 1 MB for the editor). Use upload instead.');
    }
    if (isset($_GET['create'])) {
        $name = basename($_POST['filename'] ?? '');
        if ($name === '' || $name === '.' || $name === '..') { die('Invalid filename'); }
        $target = $path === '' ? $name : $path . '/' . $name;
    } else {
        $target = $_GET['save'];
    }
    gp_raw('PUT', "/api/servers/$id/files?path=" . urlencode($target), gp_token(), $content, 'application/octet-stream');
    $dir = dirname($target);
    header('Location: /?p=server/files&id=' . $id . '&path=' . urlencode($dir === '.' ? '' : $dir));
    exit;
}

// --- Editor view (edit existing / create new) ---
if (isset($_GET['edit']) || isset($_GET['new'])) {
    $isNew = isset($_GET['new']);
    $target = $isNew ? '' : $_GET['edit'];
    $content = '';
    $editError = null;
    if (!$isNew) {
        $r = gp_raw('GET', "/api/servers/$id/files/content?path=" . urlencode($target), gp_token());
        if (!$r['ok']) {
            $editError = 'Failed to load file: ' . ($r['error'] ?? 'unknown error');
        } elseif (strlen($r['body']) > 524288) {
            $editError = 'File too large to edit here (over 512 KB). Use download + upload to replace it.';
        } elseif (strpos($r['body'], "\0") !== false) {
            $editError = 'Binary file — cannot be edited as text. Use upload to replace it.';
        } else {
            $content = $r['body'];
        }
    }
    $dir = $isNew ? $path : (dirname($target) === '.' ? '' : dirname($target));
    gp_server_head($s, $id, 'files');
    ?>
    <div class="card">
      <div class="editor-bar">
        <span class="mono" style="overflow-wrap:anywhere"><?= icon('edit', 15) ?> <?= $isNew ? 'New file in /' . e($path) : e($target) ?></span>
        <a class="btn small ghost" href="/?p=server/files&id=<?= $id ?>&path=<?= urlencode($dir) ?>">Cancel</a>
      </div>
      <?php if ($editError): ?>
        <div class="alert error"><?= icon('x', 16) ?> <?= e($editError) ?></div>
      <?php else: ?>
        <form method="POST" action="/?p=server/files&id=<?= $id ?>&path=<?= urlencode($dir) ?><?= $isNew ? '&create=1' : '&save=' . urlencode($target) ?>">
          <?php if ($isNew): ?>
            <input type="text" name="filename" placeholder="filename, e.g. config.json" required
                   style="width:100%;margin-bottom:10px;padding:8px 10px;border-radius:8px;border:1px solid var(--border);background:var(--bg);color:var(--text);font-size:13px">
          <?php endif; ?>
          <textarea class="editor" name="content" spellcheck="false" wrap="off"><?= e($content) ?></textarea>
          <div class="editor-bar" style="margin-top:10px">
            <span style="color:var(--text-dim);font-size:12px"><?= number_format(strlen($content)) ?> bytes · max 1 MB</span>
            <button class="btn" type="submit"><?= icon('check', 14) ?> Save</button>
          </div>
        </form>
      <?php endif; ?>
    </div>
    <?php
    require __DIR__ . '/../partials/footer.php';
    exit;
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
              <a class="act-btn" title="Edit" href="/?p=server/files&id=<?= $id ?>&path=<?= urlencode($path) ?>&edit=<?= urlencode($full) ?>"><?= icon('edit', 15) ?></a>
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
      <a class="btn small" href="/?p=server/files&id=<?= $id ?>&path=<?= urlencode($path) ?>&new=1"><?= icon('plus', 14) ?> New file</a>
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
