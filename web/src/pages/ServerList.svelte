<script>
  import { onMount } from 'svelte';
  import { apiFetch, navigate } from '../stores.js';

  let servers = [];
  let loading = true;
  let errorMsg = '';
  let searchQuery = '';

  async function loadServers() {
    loading = true;
    errorMsg = '';
    try {
      const res = await apiFetch('/api/servers');
      if (res.ok) {
        servers = await res.json();
      } else {
        errorMsg = 'Gagal memuat daftar server';
      }
    } catch (err) {
      errorMsg = err.message || 'Error koneksi';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadServers();
  });

  $: filteredServers = servers.filter((s) => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase();
    return (
      s.name.toLowerCase().includes(q) ||
      (s.node_name && s.node_name.toLowerCase().includes(q)) ||
      (s.ip && s.ip.toLowerCase().includes(q))
    );
  });

  function getStatusBadge(status) {
    switch (status) {
      case 'running':
        return 'text-emerald-400 border-emerald-900/60 bg-emerald-950/20';
      case 'starting':
      case 'installing':
        return 'text-amber-400 border-amber-900/60 bg-amber-950/20';
      case 'stopping':
        return 'text-rose-400 border-rose-900/60 bg-rose-950/20';
      case 'suspended':
        return 'text-purple-400 border-purple-900/60 bg-purple-950/20';
      default:
        return 'text-slate-400 border-garuda-border bg-garuda-card';
    }
  }
</script>

<div class="space-y-4">
  <!-- Top bar: Heading & Search Filter -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-garuda-border">
    <div class="flex items-baseline gap-3">
      <h1 class="text-sm font-mono font-bold uppercase tracking-wider text-slate-100">
        Servers
      </h1>
      <span class="text-xs font-mono text-garuda-muted">
        Total: {filteredServers.length}
      </span>
    </div>

    <div class="flex items-center gap-2">
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Filter nama, node, IP..."
        class="w-full sm:w-64 px-2.5 py-1 bg-garuda-surface border border-garuda-border text-xs font-mono text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
      />
      <button
        on:click={loadServers}
        class="px-2.5 py-1 bg-garuda-surface hover:bg-garuda-hover border border-garuda-border text-xs font-mono text-garuda-muted hover:text-slate-200"
        title="Refresh data"
      >
        Refresh
      </button>
    </div>
  </div>

  {#if loading}
    <div class="py-16 text-center text-xs font-mono text-garuda-muted">
      Memuat daftar server...
    </div>
  {:else if errorMsg}
    <div class="p-3 bg-garuda-surface border-l-2 border-garuda-danger text-xs font-mono text-rose-400 flex items-center justify-between">
      <span>{errorMsg}</span>
      <button on:click={loadServers} class="px-2 py-0.5 border border-garuda-border text-slate-300 hover:bg-garuda-hover">
        Coba Lagi
      </button>
    </div>
  {:else if filteredServers.length === 0}
    <div class="border border-garuda-border bg-garuda-surface p-8 text-center">
      <div class="text-xs font-mono text-garuda-muted">
        {searchQuery ? 'Tidak ada server yang cocok dengan filter pencarian.' : 'Belum ada server terdaftar pada akun ini.'}
      </div>
    </div>
  {:else}
    <!-- Dense Server Table -->
    <div class="border border-garuda-border bg-garuda-surface overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs font-mono">
        <thead>
          <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
            <th class="py-2 px-3 w-16">ID</th>
            <th class="py-2 px-3">Nama Server</th>
            <th class="py-2 px-3">Node</th>
            <th class="py-2 px-3">Endpoint (IP:Port)</th>
            <th class="py-2 px-3">Alokasi RAM</th>
            <th class="py-2 px-3">CPU Limit</th>
            <th class="py-2 px-3 w-28">Status</th>
            <th class="py-2 px-3 text-right w-20">Aksi</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-garuda-border">
          {#each filteredServers as s}
            <tr class="hover:bg-garuda-hover transition-colors">
              <td class="py-2 px-3 text-garuda-muted">#{s.id}</td>
              <td class="py-2 px-3 font-semibold text-slate-100">
                <button
                  on:click={() => navigate(`/servers/${s.id}/console`)}
                  class="text-left hover:underline hover:text-white"
                >
                  {s.name}
                </button>
              </td>
              <td class="py-2 px-3 text-slate-300">{s.node_name || '-'}</td>
              <td class="py-2 px-3 text-slate-300">
                {s.ip ? `${s.ip}:${s.port}` : '-'}
              </td>
              <td class="py-2 px-3 text-garuda-muted">{s.memory_mb} MB</td>
              <td class="py-2 px-3 text-garuda-muted">{s.cpu_percent}%</td>
              <td class="py-2 px-3">
                <span class={`inline-block px-1.5 py-0.5 border text-[10px] uppercase font-bold tracking-wider ${getStatusBadge(s.status)}`}>
                  {s.status}
                </span>
              </td>
              <td class="py-2 px-3 text-right">
                <button
                  on:click={() => navigate(`/servers/${s.id}/console`)}
                  class="px-2 py-0.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 text-[11px]"
                >
                  Buka
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
