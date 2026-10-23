<script>
  import { onMount } from 'svelte';
  import { apiFetch } from '../../stores.js';

  let nodes = [];
  let locations = [];
  let loading = true;
  let errorMsg = '';
  let showCreateModal = false;

  // New Node Form
  let name = '';
  let locationId = 1;
  let fqdn = '';
  let memoryLimit = 1024;
  let diskLimit = 10240;
  let submitting = false;

  // Join Token Dialog setelah sukses create node
  let createdNodeResult = null;
  let copied = false;

  // Allocation Modal
  let showAllocModal = false;
  let targetNodeForAlloc = null;
  let allocIP = '0.0.0.0';
  let allocPort = 25565;
  let addingAlloc = false;
  let allocList = [];
  let loadingAllocs = false;

  async function loadData() {
    loading = true;
    errorMsg = '';
    try {
      const [resN, resL] = await Promise.all([
        apiFetch('/api/admin/nodes'),
        apiFetch('/api/admin/locations'),
      ]);
      if (resN.ok && resL.ok) {
        nodes = await resN.json();
        locations = await resL.json();
        if (locations.length > 0 && !locationId) {
          locationId = locations[0].id;
        }
      } else {
        errorMsg = 'Gagal memuat data nodes atau locations';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      loading = false;
    }
  }

  async function createNode(e) {
    e.preventDefault();
    submitting = true;
    errorMsg = '';
    try {
      const res = await apiFetch('/api/admin/nodes', {
        method: 'POST',
        body: {
          name,
          location_id: Number(locationId),
          fqdn,
          memory_limit_mb: Number(memoryLimit),
          disk_limit_mb: Number(diskLimit),
        },
      });
      const data = await res.json();
      if (res.ok) {
        createdNodeResult = data;
        showCreateModal = false;
        name = '';
        fqdn = '';
        loadData();
      } else {
        errorMsg = data.error || 'Gagal membuat node';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      submitting = false;
    }
  }

  async function deleteNode(n) {
    if (!confirm(`Hapus node "${n.name}"? Pastikan tidak ada server di node ini.`)) return;
    try {
      const res = await apiFetch(`/api/admin/nodes/${n.id}`, { method: 'DELETE' });
      if (res.ok) {
        loadData();
      } else {
        const err = await res.json();
        alert('Gagal menghapus node: ' + (err.error || 'Unknown error'));
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  async function revokeToken(n) {
    if (!confirm(`Revoke token daemon untuk node "${n.name}"? Daemon akan terputus.`)) return;
    try {
      const res = await apiFetch(`/api/admin/nodes/${n.id}/revoke-token`, { method: 'POST' });
      if (res.ok) {
        alert('Token node berhasil direvoke');
      } else {
        alert('Gagal revoke token');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  async function openAllocations(n) {
    targetNodeForAlloc = n;
    showAllocModal = true;
    loadAllocations(n.id);
  }

  async function loadAllocations(nodeId) {
    loadingAllocs = true;
    try {
      const res = await apiFetch(`/api/admin/allocations?node_id=${nodeId}`);
      if (res.ok) {
        allocList = await res.json();
      }
    } catch (e) {
    } finally {
      loadingAllocs = false;
    }
  }

  async function addAllocation(e) {
    e.preventDefault();
    addingAlloc = true;
    try {
      const res = await apiFetch('/api/admin/allocations', {
        method: 'POST',
        body: {
          node_id: targetNodeForAlloc.id,
          ip: allocIP.trim(),
          port: Number(allocPort),
        },
      });
      if (res.ok) {
        loadAllocations(targetNodeForAlloc.id);
        allocPort = Number(allocPort) + 1;
      } else {
        const err = await res.json();
        alert('Gagal menambah allocation: ' + (err.error || 'Error'));
      }
    } catch (e) {
      alert('Error: ' + e.message);
    } finally {
      addingAlloc = false;
    }
  }

  function copyJoinCommand(token) {
    const cmd = `./garuda daemon --panel ${window.location.origin} --join ${token}`;
    navigator.clipboard.writeText(cmd);
    copied = true;
    setTimeout(() => (copied = false), 2500);
  }

  onMount(() => {
    loadData();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-garuda-border/50">
    <div>
      <h1 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <span>Nodes Management</span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-garuda-card border border-garuda-border font-mono text-garuda-muted">
          {nodes.length}
        </span>
      </h1>
      <p class="text-xs text-garuda-muted mt-0.5">Kelola daemon worker nodes yang menjalankan container server game.</p>
    </div>

    <button
      on:click={() => (showCreateModal = true)}
      class="px-4 py-2 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg text-xs font-bold uppercase tracking-wider transition shadow-lg shadow-sky-600/20"
    >
      + Tambah Node Baru
    </button>
  </div>

  {#if createdNodeResult}
    <!-- Join Token Notice Dialog -->
    <div class="bg-gradient-to-r from-sky-950/70 via-garuda-surface to-sky-950/70 border-2 border-sky-500/80 rounded-xl p-6 shadow-2xl relative">
      <div class="flex items-start justify-between gap-4">
        <div>
          <span class="text-[10px] font-mono uppercase tracking-widest text-garuda-primary font-bold px-2 py-0.5 rounded bg-sky-950 border border-sky-800">
            Node Created Successfully
          </span>
          <h2 class="text-base font-bold text-slate-100 mt-2">
            Join Token untuk Node: <span class="text-sky-400">#{createdNodeResult.id}</span>
          </h2>
          <p class="text-xs text-garuda-muted mt-1 max-w-xl">
            Token ini hanya ditampilkan <span class="text-amber-400 font-semibold">sekali saja</span> dan berlaku selama 15 menit. Jalankan perintah daemon berikut pada VPS/server target:
          </p>

          <div class="mt-4 p-3 bg-black/80 rounded-lg border border-garuda-border font-mono text-xs text-emerald-400 flex items-center justify-between gap-3 overflow-x-auto">
            <code>./garuda daemon --panel {window.location.origin} --join {createdNodeResult.join_token}</code>
            <button
              on:click={() => copyJoinCommand(createdNodeResult.join_token)}
              class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover text-slate-200 border border-garuda-border rounded text-[11px] whitespace-nowrap transition"
            >
              {copied ? 'Tersalin!' : 'Copy'}
            </button>
          </div>
        </div>
        <button
          on:click={() => (createdNodeResult = null)}
          class="text-slate-400 hover:text-white text-lg font-bold p-1"
        >
          &times;
        </button>
      </div>
    </div>
  {/if}

  {#if errorMsg}
    <div class="p-4 rounded-lg bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs font-mono">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat daftar nodes...</div>
  {:else if nodes.length === 0}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono bg-garuda-surface border border-garuda-border/60 rounded-xl">
      Belum ada node yang terdaftar. Tambahkan node baru untuk mulai meng-host game server.
    </div>
  {:else}
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-garuda-border/60 bg-garuda-card/40 text-garuda-muted font-mono uppercase text-[10px]">
              <th class="py-3 px-4 w-16">ID</th>
              <th class="py-3 px-4">Nama Node</th>
              <th class="py-3 px-4">FQDN / Host</th>
              <th class="py-3 px-4">Location</th>
              <th class="py-3 px-4">Limits (RAM / Disk)</th>
              <th class="py-3 px-4 text-right">Aksi</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-garuda-border/40 font-mono">
            {#each nodes as n}
              <tr class="hover:bg-garuda-card/40 transition">
                <td class="py-3 px-4 text-garuda-muted">#{n.id}</td>
                <td class="py-3 px-4 font-bold text-slate-200">
                  {n.name}
                </td>
                <td class="py-3 px-4 text-sky-400">{n.fqdn || '127.0.0.1'}</td>
                <td class="py-3 px-4 text-slate-300">
                  {locations.find((l) => l.id === n.location_id)?.short_code || `Loc #${n.location_id}`}
                </td>
                <td class="py-3 px-4 text-garuda-muted">
                  {n.memory_limit_mb || 0} MB / {n.disk_limit_mb || 0} MB
                </td>
                <td class="py-3 px-4 text-right">
                  <div class="flex items-center justify-end gap-2">
                    <button
                      on:click={() => openAllocations(n)}
                      class="px-2.5 py-1 bg-sky-950/40 hover:bg-sky-900/50 text-sky-300 border border-sky-800/50 rounded text-[11px]"
                    >
                      Allocations
                    </button>
                    <button
                      on:click={() => revokeToken(n)}
                      class="px-2.5 py-1 bg-amber-950/40 hover:bg-amber-900/50 text-amber-300 border border-amber-800/50 rounded text-[11px]"
                      title="Putuskan sambungan token daemon"
                    >
                      Revoke
                    </button>
                    <button
                      on:click={() => deleteNode(n)}
                      class="px-2.5 py-1 bg-rose-950/40 hover:bg-rose-900/50 text-rose-300 border border-rose-800/50 rounded text-[11px]"
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
    </div>
  {/if}

  <!-- Modal Tambah Node Baru -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border rounded-xl max-w-lg w-full p-6 shadow-2xl relative">
        <h3 class="text-base font-bold text-slate-100 mb-1">Tambah Node Worker Baru</h3>
        <p class="text-xs text-garuda-muted mb-5">
          Setelah node dibuat, sistem akan menghasilkan satu kali pakai token registrasi untuk daemon.
        </p>

        <form on:submit={createNode} class="space-y-4">
          <div>
            <label for="node-name" class="block text-xs font-mono text-slate-300 mb-1">Nama Node</label>
            <input
              id="node-name"
              type="text"
              bind:value={name}
              required
              placeholder="contoh: node-sg-01"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-xs font-mono text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="node-loc" class="block text-xs font-mono text-slate-300 mb-1">Location</label>
              <select
                id="node-loc"
                bind:value={locationId}
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-xs font-mono text-slate-100 focus:outline-none focus:border-garuda-primary"
              >
                {#each locations as loc}
                  <option value={loc.id}>{loc.short_code} ({loc.description || 'No desc'})</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="node-fqdn" class="block text-xs font-mono text-slate-300 mb-1">FQDN / Host</label>
              <input
                id="node-fqdn"
                type="text"
                bind:value={fqdn}
                placeholder="node1.garuda.io"
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-xs font-mono text-slate-100 focus:outline-none focus:border-garuda-primary"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="node-ram" class="block text-xs font-mono text-slate-300 mb-1">RAM Limit (MB)</label>
              <input
                id="node-ram"
                type="number"
                bind:value={memoryLimit}
                required
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-xs font-mono text-slate-100 focus:outline-none focus:border-garuda-primary"
              />
            </div>

            <div>
              <label for="node-disk" class="block text-xs font-mono text-slate-300 mb-1">Disk Limit (MB)</label>
              <input
                id="node-disk"
                type="number"
                bind:value={diskLimit}
                required
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-xs font-mono text-slate-100 focus:outline-none focus:border-garuda-primary"
              />
            </div>
          </div>

          <div class="pt-4 border-t border-garuda-border/60 flex items-center justify-end gap-2">
            <button
              type="button"
              on:click={() => (showCreateModal = false)}
              class="px-4 py-2 bg-garuda-card hover:bg-garuda-hover rounded-lg text-xs font-medium text-slate-400"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={submitting}
              class="px-5 py-2 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg text-xs font-bold transition disabled:opacity-50"
            >
              {submitting ? 'Menyimpan...' : 'Buat Node'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- Modal Allocations for Node -->
  {#if showAllocModal && targetNodeForAlloc}
    <div class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border rounded-xl max-w-xl w-full p-6 shadow-2xl relative max-h-[90vh] flex flex-col">
        <div class="flex items-center justify-between pb-3 border-b border-garuda-border/60">
          <div>
            <h3 class="text-base font-bold text-slate-100">Allocations: {targetNodeForAlloc.name}</h3>
            <p class="text-xs text-garuda-muted">Daftar port IP yang dapat digunakan oleh server di node ini.</p>
          </div>
          <button on:click={() => (showAllocModal = false)} class="text-slate-400 hover:text-white text-lg font-bold">
            &times;
          </button>
        </div>

        <!-- Add Allocation Form -->
        <form on:submit={addAllocation} class="py-4 border-b border-garuda-border/40 flex items-center gap-3">
          <input
            type="text"
            bind:value={allocIP}
            placeholder="IP Address (0.0.0.0)"
            required
            class="flex-1 px-3 py-1.5 bg-garuda-bg border border-garuda-border rounded-lg text-xs font-mono text-slate-100"
          />
          <input
            type="number"
            bind:value={allocPort}
            placeholder="Port (25565)"
            required
            class="w-28 px-3 py-1.5 bg-garuda-bg border border-garuda-border rounded-lg text-xs font-mono text-slate-100"
          />
          <button
            type="submit"
            disabled={addingAlloc}
            class="px-4 py-1.5 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg text-xs font-bold transition disabled:opacity-50"
          >
            + Tambah
          </button>
        </form>

        <!-- Allocations List -->
        <div class="flex-1 overflow-y-auto py-2">
          {#if loadingAllocs}
            <div class="py-8 text-center text-xs text-garuda-muted font-mono">Memuat...</div>
          {:else if allocList.length === 0}
            <div class="py-8 text-center text-xs text-garuda-muted font-mono">Belum ada port allocation.</div>
          {:else}
            <div class="grid grid-cols-2 gap-2 text-xs font-mono">
              {#each allocList as a}
                <div class="p-2.5 rounded-lg bg-garuda-bg border border-garuda-border/60 flex items-center justify-between">
                  <span>{a.ip}:{a.port}</span>
                  {#if a.server_id}
                    <span class="text-[9px] px-1.5 py-0.5 rounded bg-sky-950 text-sky-400 border border-sky-800">
                      Server #{a.server_id}
                    </span>
                  {:else}
                    <span class="text-[9px] px-1.5 py-0.5 rounded bg-emerald-950 text-emerald-400 border border-emerald-800">
                      FREE
                    </span>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>
