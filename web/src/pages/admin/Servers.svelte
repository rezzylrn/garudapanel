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
      alert('Node ini tidak memiliki alokasi port bebas');
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

<div class="space-y-4 font-mono text-xs">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-garuda-border">
    <div class="flex items-baseline gap-3">
      <h1 class="text-sm font-bold uppercase tracking-wider text-slate-100">
        Admin Servers
      </h1>
      <span class="text-garuda-muted text-xs">Total: {servers.length}</span>
    </div>

    <button
      on:click={openCreate}
      class="self-start sm:self-auto px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 uppercase tracking-wider"
    >
      + Tambah Server
    </button>
  </div>

  {#if errorMsg}
    <div class="p-2 border-l-2 border-garuda-danger bg-garuda-surface text-rose-400">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-16 text-center text-garuda-muted">Memuat server admin...</div>
  {:else if servers.length === 0}
    <div class="py-12 text-center text-garuda-muted border border-garuda-border bg-garuda-surface">
      Belum ada server yang dibuat.
    </div>
  {:else}
    <div class="border border-garuda-border bg-garuda-surface overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
            <th class="py-2 px-3 w-16">ID</th>
            <th class="py-2 px-3">Nama Server</th>
            <th class="py-2 px-3">Owner</th>
            <th class="py-2 px-3">Node</th>
            <th class="py-2 px-3">RAM / Disk</th>
            <th class="py-2 px-3">Status</th>
            <th class="py-2 px-3 text-right">Aksi</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-garuda-border">
          {#each servers as s}
            <tr class="hover:bg-garuda-hover">
              <td class="py-2 px-3 text-garuda-muted">#{s.id}</td>
              <td class="py-2 px-3 font-semibold text-slate-100">
                <button on:click={() => navigate(`/servers/${s.id}/console`)} class="hover:underline text-left">
                  {s.name}
                </button>
              </td>
              <td class="py-2 px-3 text-slate-300">
                {users.find((u) => u.id === s.owner_id)?.username || `User #${s.owner_id}`}
              </td>
              <td class="py-2 px-3 text-slate-300">
                {nodes.find((n) => n.id === s.node_id)?.name || `Node #${s.node_id}`}
              </td>
              <td class="py-2 px-3 text-garuda-muted">
                {s.memory_mb} MB / {s.disk_mb} MB
              </td>
              <td class="py-2 px-3">
                <span class="px-1.5 py-0.5 border border-garuda-border bg-garuda-card text-[10px] uppercase text-slate-300">
                  {s.status}
                </span>
              </td>
              <td class="py-2 px-3 text-right">
                <div class="flex items-center justify-end gap-1">
                  <button
                    on:click={() => navigate(`/servers/${s.id}/console`)}
                    class="px-2 py-0.5 border border-garuda-border bg-garuda-card hover:bg-garuda-hover text-slate-200 text-[11px]"
                  >
                    Buka
                  </button>
                  <button
                    on:click={() => deleteServer(s)}
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

  <!-- Modal Create Server -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border max-w-lg w-full p-5 max-h-[90vh] overflow-y-auto">
        <div class="flex items-center justify-between pb-2 mb-3 border-b border-garuda-border">
          <h3 class="font-bold text-slate-100 uppercase tracking-wider">Tambah Server Game</h3>
          <button on:click={() => (showCreateModal = false)} class="text-garuda-muted hover:text-white">&times;</button>
        </div>

        <form on:submit={createServer} class="space-y-3">
          <div>
            <label for="srv-name" class="block text-[11px] uppercase text-garuda-muted mb-1">Nama Server</label>
            <input
              id="srv-name"
              type="text"
              bind:value={name}
              required
              placeholder="Minecraft Survival"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="srv-owner" class="block text-[11px] uppercase text-garuda-muted mb-1">Owner User</label>
              <select
                id="srv-owner"
                bind:value={ownerId}
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              >
                {#each users as u}
                  <option value={u.id}>{u.username} ({u.email})</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="srv-node" class="block text-[11px] uppercase text-garuda-muted mb-1">Node Target</label>
              <select
                id="srv-node"
                bind:value={nodeId}
                on:change={onNodeChange}
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              >
                {#each nodes as n}
                  <option value={n.id}>{n.name} ({n.fqdn})</option>
                {/each}
              </select>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="srv-egg" class="block text-[11px] uppercase text-garuda-muted mb-1">Game Egg</label>
              <select
                id="srv-egg"
                bind:value={eggId}
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              >
                {#each eggs as eg}
                  <option value={eg.id}>{eg.name}</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="srv-alloc" class="block text-[11px] uppercase text-garuda-muted mb-1">Allocation</label>
              <select
                id="srv-alloc"
                bind:value={allocationId}
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              >
                {#if allocations.length === 0}
                  <option value={null}>Tidak ada port bebas</option>
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
              <label for="srv-ram" class="block text-[11px] uppercase text-garuda-muted mb-1">RAM (MB)</label>
              <input
                id="srv-ram"
                type="number"
                bind:value={memoryMb}
                required
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              />
            </div>

            <div>
              <label for="srv-disk" class="block text-[11px] uppercase text-garuda-muted mb-1">Disk (MB)</label>
              <input
                id="srv-disk"
                type="number"
                bind:value={diskMb}
                required
                class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
              />
            </div>

            <div>
              <label for="srv-cpu" class="block text-[11px] uppercase text-garuda-muted mb-1">CPU (%)</label>
              <input
                id="srv-cpu"
                type="number"
                bind:value={cpuPercent}
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
              disabled={submitting || !allocationId}
              class="px-4 py-1.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-100 uppercase tracking-wider font-semibold disabled:opacity-50"
            >
              {submitting ? 'Membuat...' : 'Deploy'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>
