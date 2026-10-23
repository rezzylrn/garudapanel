<script>
  import { onMount } from 'svelte';
  import { apiFetch, token } from '../../stores.js';

  export let server;

  let currentPath = '';
  let entries = [];
  let loading = true;
  let errorMsg = '';
  let diskUsage = null;

  // Editor states
  let isEditing = false;
  let editingFilePath = '';
  let fileContent = '';
  let savingFile = false;

  // New folder dialog
  let showMkdir = false;
  let newDirName = '';

  // Rename dialog
  let showRename = false;
  let renameFrom = '';
  let renameTo = '';

  // Upload state
  let isDragging = false;
  let uploading = false;
  let uploadProgressText = '';

  async function loadFiles(p = '') {
    loading = true;
    errorMsg = '';
    currentPath = p;
    try {
      const res = await apiFetch(`/api/servers/${server.id}/files?path=${encodeURIComponent(p)}`);
      if (res.ok) {
        const data = await res.json();
        // data.entries: [{name, size, is_dir, mode, mod_time}]
        entries = (data.entries || []).sort((a, b) => {
          if (a.is_dir && !b.is_dir) return -1;
          if (!a.is_dir && b.is_dir) return 1;
          return a.name.localeCompare(b.name);
        });
      } else {
        let detail = '';
        try {
          const err = await res.json();
          detail = err.error || err.message || '';
        } catch {}
        errorMsg = detail ? `Gagal memuat file: ${detail}` : 'Gagal memuat file';
      }
    } catch (e) {
      errorMsg = e.message || 'Koneksi gagal';
    } finally {
      loading = false;
    }
  }

  async function loadDiskUsage() {
    try {
      const res = await apiFetch(`/api/servers/${server.id}/files/usage`);
      if (res.ok) {
        diskUsage = await res.json();
      }
    } catch (e) {}
  }

  function navigateTo(p) {
    isEditing = false;
    loadFiles(p);
  }

  function handleBreadcrumbClick(index, parts) {
    if (index === -1) {
      navigateTo('');
      return;
    }
    const newP = parts.slice(0, index + 1).join('/');
    navigateTo(newP);
  }

  async function openFile(entry) {
    const filePath = currentPath ? `${currentPath}/${entry.name}` : entry.name;
    if (entry.is_dir) {
      navigateTo(filePath);
      return;
    }

    loading = true;
    try {
      const res = await apiFetch(`/api/servers/${server.id}/files/content?path=${encodeURIComponent(filePath)}`);
      if (res.ok) {
        fileContent = await res.text();
        editingFilePath = filePath;
        isEditing = true;
      } else {
        alert('Gagal membaca isi file');
      }
    } catch (e) {
      alert('Error membaca file: ' + e.message);
    } finally {
      loading = false;
    }
  }

  async function saveFile() {
    savingFile = true;
    try {
      let authToken = '';
      token.subscribe((t) => (authToken = t))();

      const res = await fetch(`/api/servers/${server.id}/files?path=${encodeURIComponent(editingFilePath)}`, {
        method: 'PUT',
        headers: {
          Authorization: `Bearer ${authToken}`,
          'Content-Type': 'application/octet-stream',
        },
        body: fileContent,
      });

      if (res.ok) {
        alert('File berhasil disimpan!');
      } else {
        alert('Gagal menyimpan file');
      }
    } catch (e) {
      alert('Error menyimpan file: ' + e.message);
    } finally {
      savingFile = false;
    }
  }

  async function deleteEntry(entry) {
    const p = currentPath ? `${currentPath}/${entry.name}` : entry.name;
    if (!confirm(`Hapus ${entry.is_dir ? 'folder' : 'file'} "${entry.name}"?`)) return;

    try {
      const res = await apiFetch(`/api/servers/${server.id}/files?path=${encodeURIComponent(p)}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        loadFiles(currentPath);
        loadDiskUsage();
      } else {
        alert('Gagal menghapus file');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  async function createFolder() {
    if (!newDirName.trim()) return;
    const p = currentPath ? `${currentPath}/${newDirName.trim()}` : newDirName.trim();
    try {
      const res = await apiFetch(`/api/servers/${server.id}/files/mkdir`, {
        method: 'POST',
        body: { path: p },
      });
      if (res.ok) {
        showMkdir = false;
        newDirName = '';
        loadFiles(currentPath);
      } else {
        alert('Gagal membuat folder');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  function startRename(entry) {
    renameFrom = currentPath ? `${currentPath}/${entry.name}` : entry.name;
    renameTo = renameFrom;
    showRename = true;
  }

  async function doRename() {
    if (!renameTo.trim() || renameTo === renameFrom) return;
    try {
      const res = await apiFetch(`/api/servers/${server.id}/files/rename`, {
        method: 'POST',
        body: { from: renameFrom, to: renameTo.trim() },
      });
      if (res.ok) {
        showRename = false;
        loadFiles(currentPath);
      } else {
        alert('Gagal mengganti nama');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  async function extractArchive(entry) {
    const p = currentPath ? `${currentPath}/${entry.name}` : entry.name;
    if (!confirm(`Ekstrak archive "${entry.name}" di direktori ini?`)) return;
    try {
      const res = await apiFetch(`/api/servers/${server.id}/files/extract`, {
        method: 'POST',
        body: { path: p, to: currentPath },
      });
      if (res.ok) {
        alert('Ekstrak selesai!');
        loadFiles(currentPath);
        loadDiskUsage();
      } else {
        alert('Gagal mengekstrak archive');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  function downloadEntry(entry) {
    const p = currentPath ? `${currentPath}/${entry.name}` : entry.name;
    let authToken = '';
    token.subscribe((t) => (authToken = t))();

    // Trigger download streaming via fetch blob
    apiFetch(`/api/servers/${server.id}/files/content?path=${encodeURIComponent(p)}`)
      .then((res) => res.blob())
      .then((blob) => {
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = entry.name;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        a.remove();
      })
      .catch((e) => alert('Download gagal: ' + e.message));
  }

  async function handleDrop(e) {
    e.preventDefault();
    isDragging = false;
    const files = e.dataTransfer?.files;
    if (files && files.length > 0) {
      uploadFilesList(files);
    }
  }

  function handleFileInput(e) {
    const files = e.target.files;
    if (files && files.length > 0) {
      uploadFilesList(files);
    }
  }

  async function uploadFilesList(files) {
    uploading = true;
    let authToken = '';
    token.subscribe((t) => (authToken = t))();

    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      uploadProgressText = `Mengupload ${i + 1}/${files.length}: ${file.name}...`;
      const targetPath = currentPath ? `${currentPath}/${file.name}` : file.name;

      try {
        await fetch(`/api/servers/${server.id}/files?path=${encodeURIComponent(targetPath)}`, {
          method: 'PUT',
          headers: {
            Authorization: `Bearer ${authToken}`,
            'Content-Type': 'application/octet-stream',
          },
          body: file,
        });
      } catch (e) {
        console.error('Upload error:', e);
      }
    }

    uploading = false;
    uploadProgressText = '';
    loadFiles(currentPath);
    loadDiskUsage();
  }

  function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  onMount(() => {
    loadFiles('');
    loadDiskUsage();
  });

  $: pathParts = currentPath ? currentPath.split('/').filter(Boolean) : [];
</script>

<div class="space-y-3 font-mono text-xs">
  <!-- Top Bar -->
  <div class="border border-garuda-border bg-garuda-surface p-2.5 flex flex-col md:flex-row md:items-center justify-between gap-3">
    <!-- Breadcrumb -->
    <div class="flex items-center gap-1 overflow-x-auto">
      <button
        on:click={() => handleBreadcrumbClick(-1)}
        class="text-slate-200 hover:text-white px-1.5 py-0.5 hover:bg-garuda-hover"
      >
        /root
      </button>
      {#each pathParts as part, idx}
        <span class="text-garuda-muted">/</span>
        <button
          on:click={() => handleBreadcrumbClick(idx, pathParts)}
          class="text-slate-200 hover:text-white px-1.5 py-0.5 hover:bg-garuda-hover whitespace-nowrap"
        >
          {part}
        </button>
      {/each}
    </div>

    <!-- Actions & Quota -->
    <div class="flex items-center gap-2">
      {#if diskUsage}
        <div class="text-[11px] text-garuda-muted hidden sm:block px-2 py-0.5 border border-garuda-border bg-garuda-card">
          Disk: <span class="text-slate-200 font-semibold">{diskUsage.used_mb}</span> / {server.disk_mb} MB
        </div>
      {/if}

      <button
        on:click={() => (showMkdir = true)}
        class="px-2.5 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200"
      >
        + Direktori
      </button>

      <label class="px-2.5 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 cursor-pointer">
        <span>Upload</span>
        <input type="file" multiple on:change={handleFileInput} class="hidden" />
      </label>
    </div>
  </div>

  {#if uploading}
    <div class="p-2 border border-garuda-border bg-garuda-surface text-garuda-muted">
      {uploadProgressText}
    </div>
  {/if}

  <!-- Prompt Modal Buat Folder -->
  {#if showMkdir}
    <div class="p-3 border border-garuda-border bg-garuda-surface flex items-center gap-2">
      <input
        type="text"
        bind:value={newDirName}
        placeholder="Nama direktori baru..."
        class="flex-1 bg-garuda-bg border border-garuda-border px-2.5 py-1 text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
      />
      <button on:click={createFolder} class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200">
        Buat
      </button>
      <button on:click={() => (showMkdir = false)} class="px-3 py-1 border border-garuda-border text-garuda-muted hover:bg-garuda-hover">
        Batal
      </button>
    </div>
  {/if}

  <!-- Prompt Modal Rename -->
  {#if showRename}
    <div class="p-3 border border-garuda-border bg-garuda-surface flex items-center gap-2">
      <input
        type="text"
        bind:value={renameTo}
        placeholder="Target path..."
        class="flex-1 bg-garuda-bg border border-garuda-border px-2.5 py-1 text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
      />
      <button on:click={doRename} class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200">
        Simpan
      </button>
      <button on:click={() => (showRename = false)} class="px-3 py-1 border border-garuda-border text-garuda-muted hover:bg-garuda-hover">
        Batal
      </button>
    </div>
  {/if}

  <!-- File Editor / File List -->
  {#if isEditing}
    <div class="border border-garuda-border bg-garuda-surface flex flex-col">
      <div class="p-2.5 border-b border-garuda-border bg-garuda-card flex items-center justify-between">
        <span class="text-slate-200 font-semibold">{editingFilePath}</span>
        <div class="flex items-center gap-2">
          <button
            on:click={saveFile}
            disabled={savingFile}
            class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-100 disabled:opacity-50"
          >
            {savingFile ? 'Menyimpan...' : 'Simpan'}
          </button>
          <button
            on:click={() => (isEditing = false)}
            class="px-2.5 py-1 border border-garuda-border text-garuda-muted hover:bg-garuda-hover"
          >
            Tutup
          </button>
        </div>
      </div>
      <textarea
        bind:value={fileContent}
        rows="22"
        class="w-full bg-[#090b0e] p-3 text-xs text-slate-200 focus:outline-none resize-none font-mono"
      ></textarea>
    </div>
  {:else}
    <!-- Drop area & Table -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
      on:dragover|preventDefault={() => (isDragging = true)}
      on:dragleave|preventDefault={() => (isDragging = false)}
      on:drop={handleDrop}
      class={`border bg-garuda-surface ${isDragging ? 'border-dashed border-garuda-muted bg-garuda-card' : 'border-garuda-border'}`}
    >
      {#if isDragging}
        <div class="p-6 text-center text-xs text-garuda-muted">
          Lepaskan file untuk mengupload ke {currentPath || '/root'}
        </div>
      {/if}

      {#if loading}
        <div class="py-16 text-center text-xs text-garuda-muted">Memuat file...</div>
      {:else if errorMsg}
        <div class="p-4 text-center text-xs text-rose-400">{errorMsg}</div>
      {:else if entries.length === 0}
        <div class="py-12 text-center text-xs text-garuda-muted">Direktori kosong.</div>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full text-left border-collapse text-xs">
            <thead>
              <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
                <th class="py-2 px-3">Nama</th>
                <th class="py-2 px-3 w-28">Ukuran</th>
                <th class="py-2 px-3 w-40">Dimodifikasi</th>
                <th class="py-2 px-3 text-right w-44">Aksi</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-garuda-border">
              {#if currentPath}
                <tr
                  on:click={() => {
                    const parts = currentPath.split('/');
                    parts.pop();
                    navigateTo(parts.join('/'));
                  }}
                  class="hover:bg-garuda-hover cursor-pointer text-garuda-muted"
                >
                  <td class="py-1.5 px-3" colspan="4">.. [Kembali]</td>
                </tr>
              {/if}

              {#each entries as entry}
                <tr class="hover:bg-garuda-hover group">
                  <td class="py-1.5 px-3">
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <div
                      on:click={() => openFile(entry)}
                      class="flex items-center gap-2 cursor-pointer text-slate-200 hover:text-white"
                    >
                      {#if entry.is_dir}
                        <span class="text-garuda-muted">[DIR]</span>
                      {:else}
                        <span class="text-garuda-border">[FILE]</span>
                      {/if}
                      <span>{entry.name}</span>
                    </div>
                  </td>
                  <td class="py-1.5 px-3 text-garuda-muted">
                    {entry.is_dir ? '-' : formatBytes(entry.size)}
                  </td>
                  <td class="py-1.5 px-3 text-garuda-muted text-[11px]">
                    {entry.mod_time ? new Date(entry.mod_time).toLocaleString('id-ID') : '-'}
                  </td>
                  <td class="py-1.5 px-3 text-right">
                    <div class="flex items-center justify-end gap-1">
                      {#if !entry.is_dir}
                        <button
                          on:click={() => downloadEntry(entry)}
                          class="px-1.5 py-0.5 border border-garuda-border hover:bg-garuda-card text-garuda-muted hover:text-slate-200 text-[10px]"
                        >
                          Unduh
                        </button>
                      {/if}

                      {#if !entry.is_dir && (entry.name.endsWith('.zip') || entry.name.endsWith('.tar.gz'))}
                        <button
                          on:click={() => extractArchive(entry)}
                          class="px-1.5 py-0.5 border border-garuda-border hover:bg-garuda-card text-garuda-muted hover:text-slate-200 text-[10px]"
                        >
                          Ekstrak
                        </button>
                      {/if}

                      <button
                        on:click={() => startRename(entry)}
                        class="px-1.5 py-0.5 border border-garuda-border hover:bg-garuda-card text-garuda-muted hover:text-slate-200 text-[10px]"
                      >
                        Rename
                      </button>

                      <button
                        on:click={() => deleteEntry(entry)}
                        class="px-1.5 py-0.5 border border-garuda-border hover:bg-garuda-card text-rose-400 text-[10px]"
                      >
                        Hapus
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}
</div>
