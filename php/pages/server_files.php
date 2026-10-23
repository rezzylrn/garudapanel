<?php
// Files tab — HTMX-powered browser + upload
$path = $_GET['path'] ?? '';

// HTMX partial: return only the file list
if (isset($_GET['partial'])) {
    $r = gp_api('GET', "/api/servers/$id/files?path=" . urlencode($path), null, gp_token());
    if (!$r['ok']) {
        echo '<div class="alert error">' . e($r['error'] ?? 'Failed to load files') . '</div>';
        exit;
    }
    $entries = $r['data']['entries'] ?? [];
    usort($entries, fn($a, $b) => ($b['is_dir'] <=> $a['is_dir']) ?: strcmp($a['name'], $b['name']));
    ?>
    <table>
      <thead><tr><th>Name</th><th>Size</th><th>Modified</th><th></th></tr></thead>
      <tbody>
      <?php if ($path !== ''): ?>
        <tr><td colspan="4"><a href="#" hx-get="/?p=server&id=<?= $id ?>&tab=files&partial=1&path=<?= urlencode(dirname($path) === '.' ? '' : dirname($path)) ?>" hx-target="#file-list" style="color:var(--text-dim)">← ..</a></td></tr>
      <?php endif; ?>
      <?php foreach ($entries as $en): ?>
        <?php $full = $path === '' ? $en['name'] : $path . '/' . $en['name']; ?>
        <tr>
          <td>
            <?php if ($en['is_dir']): ?>
              <a href="#" hx-get="/?p=server&id=<?= $id ?>&tab=files&partial=1&path=<?= urlencode($full) ?>" hx-target="#file-list">📁 <?= e($en['name']) ?></a>
            <?php else: ?>
              📄 <?= e($en['name']) ?>
            <?php endif; ?>
          </td>
          <td class="mono"><?= $en['is_dir'] ? '—' : number_format($en['size'] ?? 0) . ' B' ?></td>
          <td class="mono" style="color:var(--text-mute)"><?= e($en['mod_time'] ?? '') ?></td>
          <td style="text-align:right">
            <?php if (!$en['is_dir']): ?>
              <a class="btn small" href="<?= GP_API_URL ?>/api/servers/<?= $id ?>/files/content?path=<?= urlencode($full) ?>&token=<?= urlencode(gp_token()) ?>">Download</a>
            <?php endif; ?>
          </td>
        </tr>
      <?php endforeach; ?>
      <?php if (empty($entries)): ?>
        <tr><td colspan="4" style="color:var(--text-mute);text-align:center;padding:24px">Empty directory</td></tr>
      <?php endif; ?>
      </tbody>
    </table>
    <?php
    exit;
}
?>

<div class="card">
  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:14px">
    <span class="mono" style="color:var(--text-dim)">/<?= e($path) ?></span>
    <div style="display:flex;gap:8px">
      <form hx-post="/?p=server&id=<?= $id ?>&tab=files&partial=1&mkdir=1&path=<?= urlencode($path) ?>" hx-target="#file-list" style="display:flex;gap:8px">
        <input type="text" name="dirname" placeholder="New folder" required style="padding:6px 10px;border-radius:8px;border:1px solid var(--border);background:var(--bg);color:var(--text);font-size:12px">
        <button class="btn small" type="submit">+ Folder</button>
      </form>
      <form action="/?p=server&id=<?= $id ?>&tab=files&upload=1&path=<?= urlencode($path) ?>" method="POST" enctype="multipart/form-data" style="display:flex;gap:8px">
        <input type="file" name="file" required style="font-size:12px;color:var(--text-dim)">
        <button class="btn small" type="submit">Upload</button>
      </form>
    </div>
  </div>
  <div id="file-list" hx-get="/?p=server&id=<?= $id ?>&tab=files&partial=1&path=<?= urlencode($path) ?>" hx-trigger="load" hx-target="#file-list">
    <p style="color:var(--text-mute)">Loading…</p>
  </div>
</div>

<?php
// Handle mkdir / upload (non-HTMX POST for upload)
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    if (isset($_GET['mkdir'])) {
        $dir = $path === '' ? ($_POST['dirname'] ?? '') : $path . '/' . ($_POST['dirname'] ?? '');
        gp_api('POST', "/api/servers/$id/files/mkdir", ['path' => $dir], gp_token());
        // Return refreshed list
        $_GET['partial'] = 1;
        // (falls through to partial render above on next HTMX request)
        exit;
    }
    if (isset($_GET['upload']) && isset($_FILES['file'])) {
        $tmp = $_FILES['file']['tmp_name'];
        $name = basename($_FILES['file']['name']);
        $target = $path === '' ? $name : $path . '/' . $name;
        // Stream upload to Go API via PUT
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
        curl_close($ch);
        header("Location: /?p=server&id=$id&tab=files&path=" . urlencode($path));
        exit;
    }
}
