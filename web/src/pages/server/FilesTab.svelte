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
        errorMsg = 'Gagal memuat file';
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

<div class="space-y-4">
  <!-- Top Bar: Breadcrumb + Quota + Actions -->
  <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-lg">
    <!-- Breadcrumb -->
    <div class="flex items-center gap-1.5 text-xs font-mono overflow-x-auto py-1">
      <button
        on:click={() => handleBreadcrumbClick(-1)}
        class="text-garuda-primary hover:underline font-bold px-1.5 py-0.5 rounded hover:bg-garuda-hover"
      >
        /root
      </button>
      {#each pathParts as part, idx}
        <span class="text-garuda-muted">/</span>
        <button
          on:click={() => handleBreadcrumbClick(idx, pathParts)}
          class="text-slate-200 hover:text-garuda-primary hover:underline px-1.5 py-0.5 rounded hover:bg-garuda-hover whitespace-nowrap"
        >
          {part}
        </button>
      {/each}
    </div>

    <!-- Actions & Disk Quota info -->
    <div class="flex items-center gap-3">
      {#if diskUsage}
        <div class="text-[11px] font-mono text-garuda-muted hidden sm:block bg-garuda-bg px-2.5 py-1 rounded border border-garuda-border/60">
          Disk: <span class="text-slate-200 font-bold">{diskUsage.used_mb}</span> / {server.disk_mb} MB
        </div>
      {/if}

      <button
        on:click={() => (showMkdir = true)}
        class="px-3 py-1.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border rounded-lg text-xs font-medium text-slate-200 transition"
      >
        + Buat Folder
      </button>

      <label class="px-3 py-1.5 bg-sky-600/20 hover:bg-sky-600/30 text-sky-400 border border-sky-500/40 rounded-lg text-xs font-medium cursor-pointer transition">
        <span>Upload File</span>
        <input type="file" multiple on:change={handleFileInput} class="hidden" />
      </label>
    </div>
  </div>

  {#if uploading}
    <div class="p-3 bg-sky-950/40 border border-sky-800/60 rounded-xl text-xs text-sky-300 font-mono flex items-center gap-3 animate-pulse">
      <div class="w-3.5 h-3.5 border-2 border-sky-400 border-t-transparent rounded-full animate-spin"></div>
      <span>{uploadProgressText}</span>
    </div>
  {/if}

  <!-- Modal Buat Folder -->
  {#if showMkdir}
    <div class="p-4 bg-garuda-surface border border-garuda-border rounded-xl shadow-xl flex items-center gap-3">
      <input
        type="text"
        bind:value={newDirName}
        placeholder="Nama folder baru..."
        class="flex-1 bg-garuda-bg border border-garuda-border rounded-lg px-3 py-1.5 text-xs font-mono text-slate-100 focus:outline-none focus:border-garuda-primary"
      />
      <button on:click={createFolder} class="px-3 py-1.5 bg-garuda-primary text-white rounded-lg text-xs font-medium">Buat</button>
      <button on:click={() => (showMkdir = false)} class="px-3 py-1.5 bg-garuda-card text-slate-400 rounded-lg text-xs">Batal</button>
    </div>
  {/if}

  <!-- Modal Rename -->
  {#if showRename}
    <div class="p-4 bg-garuda-surface border border-garuda-border rounded-xl shadow-xl flex items-center gap-3">
      <input
        type="text"
        bind:value={renameTo}
        placeholder="Nama path baru..."
        class="flex-1 bg-garuda-bg border border-garuda-border rounded-lg px-3 py-1.5 text-xs font-mono text-slate-100 focus:outline-none focus:border-garuda-primary"
      />
      <button on:click={doRename} class="px-3 py-1.5 bg-garuda-primary text-white rounded-lg text-xs font-medium">Simpan</button>
      <button on:click={() => (showRename = false)} class="px-3 py-1.5 bg-garuda-card text-slate-400 rounded-lg text-xs">Batal</button>
    </div>
  {/if}

  <!-- Editor View or File List -->
  {#if isEditing}
    <!-- File Editor -->
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl overflow-hidden shadow-2xl flex flex-col">
      <div class="p-3 bg-garuda-card/80 border-b border-garuda-border flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="text-xs font-mono text-garuda-primary font-bold">{editingFilePath}</span>
        </div>
        <div class="flex items-center gap-2">
          <button
            on:click={saveFile}
            disabled={savingFile}
            class="px-4 py-1.5 bg-emerald-600/30 hover:bg-emerald-600/50 text-emerald-400 border border-emerald-500/50 rounded-lg text-xs font-medium transition"
          >
            {savingFile ? 'Menyimpan...' : 'Simpan Perubahan'}
          </button>
          <button
            on:click={() => (isEditing = false)}
            class="px-3 py-1.5 bg-garuda-card hover:bg-garuda-hover text-slate-400 rounded-lg text-xs transition"
          >
            Tutup
          </button>
        </div>
      </div>
      <textarea
        bind:value={fileContent}
        rows="22"
        class="w-full bg-[#070a0f] p-4 text-xs font-mono text-slate-200 focus:outline-none resize-none selection:bg-sky-500/30"
      ></textarea>
    </div>
  {:else}
    <!-- Drag & Drop container area -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
      on:dragover|preventDefault={() => (isDragging = true)}
      on:dragleave|preventDefault={() => (isDragging = false)}
      on:drop={handleDrop}
      class={`bg-garuda-surface border rounded-xl overflow-hidden shadow-xl transition-all ${
        isDragging ? 'border-dashed border-garuda-primary bg-sky-950/20' : 'border-garuda-border/80'
      }`}
    >
      {#if isDragging}
        <div class="p-8 text-center font-mono text-xs text-sky-400 animate-pulse">
          Lepaskan file di sini untuk mengupload ke {currentPath || '/root'}
        </div>
      {/if}

      <!-- File Table -->
      {#if loading}
        <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat direktori...</div>
      {:else if errorMsg}
        <div class="p-6 text-center text-xs text-rose-400 font-mono">{errorMsg}</div>
      {:else if entries.length === 0}
        <div class="py-16 text-center text-xs text-garuda-muted font-mono">Direktori ini kosong.</div>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full text-left border-collapse text-xs">
            <thead>
              <tr class="border-b border-garuda-border/60 bg-garuda-card/40 text-garuda-muted font-mono uppercase text-[10px]">
                <th class="py-3 px-4">Nama</th>
                <th class="py-3 px-4 w-28">Ukuran</th>
                <th class="py-3 px-4 w-44">Dimodifikasi</th>
                <th class="py-3 px-4 text-right w-48">Aksi</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-garuda-border/40 font-mono">
              {#if currentPath}
                <tr
                  on:click={() => {
                    const parts = currentPath.split('/');
                    parts.pop();
                    navigateTo(parts.join('/'));
                  }}
                  class="hover:bg-garuda-card/40 cursor-pointer text-garuda-primary"
                >
                  <td class="py-2.5 px-4 font-bold flex items-center gap-2" colspan="4">
                    <span>.. (Kembali ke folder atas)</span>
                  </td>
                </tr>
              {/if}

              {#each entries as entry}
                <tr class="hover:bg-garuda-card/40 group transition-colors">
                  <td class="py-2.5 px-4">
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <div
                      on:click={() => openFile(entry)}
                      class="flex items-center gap-2 cursor-pointer font-medium text-slate-200 group-hover:text-garuda-primary transition"
                    >
                      {#if entry.is_dir}
                        <span class="text-amber-400 font-bold">[DIR]</span>
                      {:else}
                        <span class="text-slate-500">[FILE]</span>
                      {/if}
                      <span>{entry.name}</span>
                    </div>
                  </td>
                  <td class="py-2.5 px-4 text-garuda-muted">
                    {entry.is_dir ? '-' : formatBytes(entry.size)}
                  </td>
                  <td class="py-2.5 px-4 text-garuda-muted text-[11px]">
                    {entry.mod_time ? new Date(entry.mod_time).toLocaleString('id-ID') : '-'}
                  </td>
                  <td class="py-2.5 px-4 text-right">
                    <div class="flex items-center justify-end gap-1.5 opacity-80 group-hover:opacity-100 transition">
                      {#if !entry.is_dir}
                        <button
                          on:click={() => downloadEntry(entry)}
                          class="px-2 py-1 rounded bg-garuda-card hover:bg-garuda-hover text-slate-300 text-[10px]"
                          title="Download"
                        >
                          Unduh
                        </button>
                      {/if}

                      {#if !entry.is_dir && (entry.name.endsWith('.zip') || entry.name.endsWith('.tar.gz'))}
                        <button
                          on:click={() => extractArchive(entry)}
                          class="px-2 py-1 rounded bg-purple-950/40 hover:bg-purple-900/50 text-purple-300 text-[10px]"
                          title="Ekstrak"
                        >
                          Ekstrak
                        </button>
                      {/if}

                      <button
                        on:click={() => startRename(entry)}
                        class="px-2 py-1 rounded bg-garuda-card hover:bg-garuda-hover text-slate-300 text-[10px]"
                        title="Rename"
                      >
                        Rename
                      </button>

                      <button
                        on:click={() => deleteEntry(entry)}
                        class="px-2 py-1 rounded bg-rose-950/40 hover:bg-rose-900/50 text-rose-300 text-[10px]"
                        title="Hapus"
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
