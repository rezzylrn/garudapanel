<script>
  import { onMount } from 'svelte';
  import { apiFetch, navigate } from '../../stores.js';

  let servers = [];
  let users = [];
  let nodes = [];
  let eggs = [];
  let allocations = [];
  let loading = true;
  let errorMsg = '';
  let showCreateModal = false;

  // Create Form State
  let name = '';
  let ownerId = null;
  let nodeId = null;
  let eggId = null;
  let allocationId = null;
  let memoryMb = 1024;
  let diskMb = 5120;
  let cpuPercent = 100;
  let submitting = false;

  async function loadData() {
    loading = true;
    errorMsg = '';
    try {
      const [resS, resU, resN, resE] = await Promise.all([
        apiFetch('/api/admin/servers'),
        apiFetch('/api/admin/users'),
        apiFetch('/api/admin/nodes'),
        apiFetch('/api/admin/eggs'),
      ]);
      if (resS.ok && resU.ok && resN.ok && resE.ok) {
        servers = await resS.json();
        users = await resU.json();
        nodes = await resN.json();
        eggs = await resE.json();
      } else {
        errorMsg = 'Gagal memuat data master server';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      loading = false;
    }
  }

  async function onNodeChange() {
    allocationId = null;
    allocations = [];
    if (!nodeId) return;
    try {
      const res = await apiFetch(`/api/admin/allocations?node_id=${nodeId}`);
      if (res.ok) {
        const data = await res.json();
        // filter allocation yang belum dipakai (server_id == 0 / null)
        allocations = data.filter((a) => !a.server_id);
        if (allocations.length > 0) {
          allocationId = allocations[0].id;
        }
      }
    } catch (e) {}
  }

  function openCreate() {
    if (users.length > 0) ownerId = users[0].id;
    if (nodes.length > 0) {
      nodeId = nodes[0].id;
      onNodeChange();
    }
    if (eggs.length > 0) eggId = eggs[0].id;
    showCreateModal = true;
  }

  async function createServer(e) {
    e.preventDefault();
    if (!allocationId) {
      alert('Node ini tidak memiliki allocation port bebas. Tambah allocation di halaman Nodes dulu.');
      return;
    }
    submitting = true;
    errorMsg = '';
    try {
      const res = await apiFetch('/api/admin/servers', {
        method: 'POST',
        body: {
          name,
          owner_id: Number(ownerId),
          node_id: Number(nodeId),
          egg_id: Number(eggId),
          allocation_id: Number(allocationId),
          memory_mb: Number(memoryMb),
          disk_mb: Number(diskMb),
          cpu_percent: Number(cpuPercent),
        },
      });
      const data = await res.json();
      if (res.ok) {
        showCreateModal = false;
        name = '';
        loadData();
      } else {
        alert(data.error || 'Gagal membuat server');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    } finally {
      submitting = false;
    }
  }

  async function deleteServer(s) {
    if (!confirm(`Hapus server "${s.name}" (ID: #${s.id})? Data akan dihapus permanen.`)) return;
    try {
      const res = await apiFetch(`/api/admin/servers/${s.id}`, { method: 'DELETE' });
      if (res.ok) {
        loadData();
      } else {
        const err = await res.json();
        alert('Gagal: ' + (err.error || 'Error'));
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
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
        <span>Admin Servers Management</span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-garuda-card border border-garuda-border font-mono text-garuda-muted">
          {servers.length}
        </span>
      </h1>
      <p class="text-xs text-garuda-muted mt-0.5">Kelola dan deploy server game untuk seluruh user platform.</p>
    </div>

    <button
      on:click={openCreate}
      class="px-4 py-2 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg text-xs font-bold uppercase tracking-wider transition shadow-lg shadow-sky-600/20"
    >
      + Buat Server Baru
    </button>
  </div>

  {#if errorMsg}
    <div class="p-4 rounded-lg bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs font-mono">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat servers...</div>
  {:else if servers.length === 0}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono bg-garuda-surface border border-garuda-border/60 rounded-xl">
      Belum ada server yang dibuat.
    </div>
  {:else}
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-garuda-border/60 bg-garuda-card/40 text-garuda-muted font-mono uppercase text-[10px]">
              <th class="py-3 px-4 w-14">ID</th>
              <th class="py-3 px-4">Nama Server</th>
              <th class="py-3 px-4">Owner</th>
              <th class="py-3 px-4">Node</th>
              <th class="py-3 px-4">Status</th>
              <th class="py-3 px-4">RAM / Disk</th>
              <th class="py-3 px-4 text-right">Aksi</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-garuda-border/40 font-mono">
            {#each servers as s}
              <tr class="hover:bg-garuda-card/40 transition">
                <td class="py-3 px-4 text-garuda-muted">#{s.id}</td>
                <td class="py-3 px-4 font-bold text-slate-200">
                  <button on:click={() => navigate(`/servers/${s.id}/console`)} class="hover:text-garuda-primary text-left">
                    {s.name}
                  </button>
                </td>
                <td class="py-3 px-4 text-slate-300">
                  {users.find((u) => u.id === s.owner_id)?.username || `User #${s.owner_id}`}
                </td>
                <td class="py-3 px-4 text-slate-300">
                  {nodes.find((n) => n.id === s.node_id)?.name || `Node #${s.node_id}`}
                </td>
                <td class="py-3 px-4">
                  <span class="text-[10px] px-2 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700 uppercase">
                    {s.status}
                  </span>
                </td>
                <td class="py-3 px-4 text-garuda-muted">
                  {s.memory_mb} MB / {s.disk_mb} MB
                </td>
                <td class="py-3 px-4 text-right">
                  <div class="flex items-center justify-end gap-2">
                    <button
                      on:click={() => navigate(`/servers/${s.id}/console`)}
                      class="px-2.5 py-1 bg-garuda-card hover:bg-garuda-hover text-slate-300 rounded text-[11px]"
                    >
                      Buka
                    </button>
                    <button
                      on:click={() => deleteServer(s)}
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

  <!-- Modal Create Server -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border rounded-xl max-w-lg w-full p-6 shadow-2xl relative max-h-[90vh] overflow-y-auto">
        <h3 class="text-base font-bold text-slate-100 mb-1">Deploy Server Game Baru</h3>
        <p class="text-xs text-garuda-muted mb-5">
          Tentukan konfigurasi node, template egg, resource limit, dan port alokasi.
        </p>

        <form on:submit={createServer} class="space-y-4 font-mono text-xs">
          <div>
            <label for="srv-name" class="block text-slate-300 mb-1">Nama Server</label>
            <input
              id="srv-name"
              type="text"
              bind:value={name}
              required
              placeholder="Minecraft Survival #1"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="srv-owner" class="block text-slate-300 mb-1">Owner User</label>
              <select
                id="srv-owner"
                bind:value={ownerId}
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
              >
                {#each users as u}
                  <option value={u.id}>{u.username} ({u.email})</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="srv-node" class="block text-slate-300 mb-1">Node Target</label>
              <select
                id="srv-node"
                bind:value={nodeId}
                on:change={onNodeChange}
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
              >
                {#each nodes as n}
                  <option value={n.id}>{n.name} ({n.fqdn})</option>
                {/each}
              </select>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="srv-egg" class="block text-slate-300 mb-1">Game Egg</label>
              <select
                id="srv-egg"
                bind:value={eggId}
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
              >
                {#each eggs as eg}
                  <option value={eg.id}>{eg.name}</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="srv-alloc" class="block text-slate-300 mb-1">Primary Allocation</label>
              <select
                id="srv-alloc"
                bind:value={allocationId}
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
              >
                {#if allocations.length === 0}
                  <option value={null}>Tidak ada port bebas di node ini</option>
                {:else}
                  {#each allocations as a}
                    <option value={a.id}>{a.ip}:{a.port}</option>
                  {/each}
                {/if}
              </select>
            </div>
          </div>

          <div class="grid grid-cols-3 gap-3">
            <div>
              <label for="srv-ram" class="block text-slate-300 mb-1">Memory (MB)</label>
              <input
                id="srv-ram"
                type="number"
                bind:value={memoryMb}
                required
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
              />
            </div>

            <div>
              <label for="srv-disk" class="block text-slate-300 mb-1">Disk (MB)</label>
              <input
                id="srv-disk"
                type="number"
                bind:value={diskMb}
                required
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
              />
            </div>

            <div>
              <label for="srv-cpu" class="block text-slate-300 mb-1">CPU Limit (%)</label>
              <input
                id="srv-cpu"
                type="number"
                bind:value={cpuPercent}
                required
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
              />
            </div>
          </div>

          <div class="pt-4 border-t border-garuda-border/60 flex items-center justify-end gap-2">
            <button
              type="button"
              on:click={() => (showCreateModal = false)}
              class="px-4 py-2 bg-garuda-card hover:bg-garuda-hover rounded-lg text-slate-400"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={submitting || !allocationId}
              class="px-5 py-2 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg font-bold transition disabled:opacity-50"
            >
              {submitting ? 'Membuat Server...' : 'Deploy Server'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>
