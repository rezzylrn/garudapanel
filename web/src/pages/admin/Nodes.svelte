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

<div class="space-y-4 font-mono text-xs">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-garuda-border">
    <div class="flex items-baseline gap-3">
      <h1 class="text-sm font-bold uppercase tracking-wider text-slate-100">
        Nodes
      </h1>
      <span class="text-garuda-muted text-xs">Total: {nodes.length}</span>
    </div>

    <button
      on:click={() => (showCreateModal = true)}
      class="self-start sm:self-auto px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 uppercase tracking-wider"
    >
      + Tambah Node
    </button>
  </div>

  {#if createdNodeResult}
    <!-- Join Token Notice Dialog -->
    <div class="border border-garuda-border bg-garuda-surface p-4">
      <div class="flex items-start justify-between gap-4">
        <div>
          <div class="text-[11px] uppercase tracking-wider text-garuda-muted font-bold">
            Node Ditambahkan
          </div>
          <div class="text-xs font-bold text-slate-100 mt-1">
            Join Token Node #{createdNodeResult.id} (Berlaku 15 menit)
          </div>
          <p class="text-[11px] text-garuda-muted mt-1">
            Jalankan perintah ini sekali pada VPS worker target:
          </p>

          <div class="mt-2.5 p-2 bg-garuda-bg border border-garuda-border text-[11px] text-slate-200 flex items-center justify-between gap-2 overflow-x-auto">
            <code>./garuda daemon --panel {window.location.origin} --join {createdNodeResult.join_token}</code>
            <button
              on:click={() => copyJoinCommand(createdNodeResult.join_token)}
              class="px-2 py-0.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 text-[10px] uppercase whitespace-nowrap"
            >
              {copied ? 'Tersalin' : 'Copy'}
            </button>
          </div>
        </div>
        <button
          on:click={() => (createdNodeResult = null)}
          class="text-garuda-muted hover:text-white text-sm"
        >
          &times;
        </button>
      </div>
    </div>
  {/if}

  {#if errorMsg}
    <div class="p-2 border-l-2 border-garuda-danger bg-garuda-surface text-rose-400">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-16 text-center text-garuda-muted">Memuat daftar nodes...</div>
  {:else if nodes.length === 0}
    <div class="py-12 text-center text-garuda-muted border border-garuda-border bg-garuda-surface">
      Belum ada node yang terdaftar.
    </div>
  {:else}
    <div class="border border-garuda-border bg-garuda-surface overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
            <th class="py-2 px-3 w-16">ID</th>
            <th class="py-2 px-3">Nama Node</th>
            <th class="py-2 px-3">Host / FQDN</th>
            <th class="py-2 px-3">Location</th>
            <th class="py-2 px-3">Limits (RAM / Disk)</th>
            <th class="py-2 px-3 text-right">Aksi</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-garuda-border">
          {#each nodes as n}
            <tr class="hover:bg-garuda-hover">
              <td class="py-2 px-3 text-garuda-muted">#{n.id}</td>
              <td class="py-2 px-3 font-semibold text-slate-100">{n.name}</td>
              <td class="py-2 px-3 text-slate-300">{n.fqdn || '127.0.0.1'}</td>
              <td class="py-2 px-3 text-slate-300">
                {locations.find((l) => l.id === n.location_id)?.short_code || `Loc #${n.location_id}`}
              </td>
              <td class="py-2 px-3 text-garuda-muted">
                {n.memory_limit_mb || 0} MB / {n.disk_limit_mb || 0} MB
              </td>
              <td class="py-2 px-3 text-right">
                <div class="flex items-center justify-end gap-1">
                  <button
                    on:click={() => openAllocations(n)}
                    class="px-2 py-0.5 border border-garuda-border bg-garuda-card hover:bg-garuda-hover text-slate-200 text-[11px]"
                  >
                    Allocations
                  </button>
                  <button
                    on:click={() => revokeToken(n)}
                    class="px-2 py-0.5 border border-garuda-border bg-garuda-card hover:bg-garuda-hover text-amber-400 text-[11px]"
                    title="Putuskan sambungan token daemon"
                  >
                    Revoke
                  </button>
                  <button
                    on:click={() => deleteNode(n)}
                    class="px-2 py-0.5 border border-garuda-border bg-garuda-card hover:bg-garuda-hover text-rose-400 text-[11px]"
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

  <!-- Modal Tambah Node Baru -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border max-w-lg w-full p-5">
        <div class="flex items-center justify-between pb-2 mb-3 border-b border-garuda-border">
          <h3 class="font-bold text-slate-100 uppercase tracking-wider">Tambah Worker Node</h3>
          <button on:click={() => (showCreateModal = false)} class="text-garuda-muted hover:text-white">&times;</button>
        </div>

        <form on:submit={createNode} class="space-y-3">
          <div>
            <label for="node-name" class="block text-[11px] uppercase text-garuda-muted mb-1">Nama Node</label>
            <input
              id="node-name"
              type="text"
              bind:value={name}
              required
              placeholder="node-01"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="node-loc" class="block text-[11px] uppercase text-garuda-muted mb-1">Location</label>
              <select
                id="node-loc"
                bind:value={locationId}
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              >
                {#each locations as loc}
                  <option value={loc.id}>{loc.short_code} ({loc.description || '-'})</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="node-fqdn" class="block text-[11px] uppercase text-garuda-muted mb-1">FQDN / Host</label>
              <input
                id="node-fqdn"
                type="text"
                bind:value={fqdn}
                placeholder="node1.garuda.io"
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="node-ram" class="block text-[11px] uppercase text-garuda-muted mb-1">RAM Limit (MB)</label>
              <input
                id="node-ram"
                type="number"
                bind:value={memoryLimit}
                required
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              />
            </div>

            <div>
              <label for="node-disk" class="block text-[11px] uppercase text-garuda-muted mb-1">Disk Limit (MB)</label>
              <input
                id="node-disk"
                type="number"
                bind:value={diskLimit}
                required
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              />
            </div>
          </div>

          <div class="pt-3 border-t border-garuda-border flex items-center justify-end gap-2">
            <button
              type="button"
              on:click={() => (showCreateModal = false)}
              class="px-3 py-1.5 border border-garuda-border text-garuda-muted hover:bg-garuda-hover"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={submitting}
              class="px-4 py-1.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-100 uppercase tracking-wider font-semibold disabled:opacity-50"
            >
              {submitting ? 'Menyimpan...' : 'Simpan'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- Modal Allocations for Node -->
  {#if showAllocModal && targetNodeForAlloc}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border max-w-xl w-full p-5 max-h-[90vh] flex flex-col">
        <div class="flex items-center justify-between pb-2 border-b border-garuda-border">
          <div>
            <h3 class="font-bold text-slate-100 uppercase tracking-wider">Allocations: {targetNodeForAlloc.name}</h3>
            <p class="text-[11px] text-garuda-muted">Daftar endpoint IP & Port untuk node ini.</p>
          </div>
          <button on:click={() => (showAllocModal = false)} class="text-garuda-muted hover:text-white">&times;</button>
        </div>

        <!-- Add Allocation Form -->
        <form on:submit={addAllocation} class="py-3 border-b border-garuda-border flex items-center gap-2">
          <input
            type="text"
            bind:value={allocIP}
            placeholder="IP Address (0.0.0.0)"
            required
            class="flex-1 px-2.5 py-1 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border"
          />
          <input
            type="number"
            bind:value={allocPort}
            placeholder="Port"
            required
            class="w-24 px-2.5 py-1 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border"
          />
          <button
            type="submit"
            disabled={addingAlloc}
            class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 uppercase tracking-wider whitespace-nowrap disabled:opacity-50"
          >
            + Tambah
          </button>
        </form>

        <!-- Allocations List -->
        <div class="flex-1 overflow-y-auto py-2">
          {#if loadingAllocs}
            <div class="py-6 text-center text-garuda-muted">Memuat...</div>
          {:else if allocList.length === 0}
            <div class="py-6 text-center text-garuda-muted">Belum ada alokasi port.</div>
          {:else}
            <div class="grid grid-cols-2 gap-1.5">
              {#each allocList as a}
                <div class="p-2 bg-garuda-bg border border-garuda-border flex items-center justify-between">
                  <span>{a.ip}:{a.port}</span>
                  {#if a.server_id}
                    <span class="text-[9px] px-1 border border-garuda-border text-garuda-muted">
                      Server #{a.server_id}
                    </span>
                  {:else}
                    <span class="text-[9px] px-1 border border-garuda-border text-emerald-400">
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
