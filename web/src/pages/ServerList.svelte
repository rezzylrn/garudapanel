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

  function getStatusColor(status) {
    switch (status) {
      case 'running':
        return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30';
      case 'starting':
      case 'installing':
        return 'bg-amber-500/10 text-amber-400 border-amber-500/30';
      case 'stopping':
        return 'bg-rose-500/10 text-rose-400 border-rose-500/30';
      case 'suspended':
        return 'bg-purple-500/10 text-purple-400 border-purple-500/30';
      default:
        return 'bg-slate-700/30 text-slate-400 border-slate-700';
    }
  }
</script>

<div class="space-y-6">
  <!-- Header Bar -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-garuda-border/50">
    <div>
      <h1 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <span>Game Servers</span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-garuda-card border border-garuda-border font-mono text-garuda-muted">
          {filteredServers.length}
        </span>
      </h1>
      <p class="text-xs text-garuda-muted mt-0.5">Daftar instance server game aktif di infrastruktur Anda.</p>
    </div>

    <!-- Search box -->
    <div class="w-full sm:w-72">
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Cari server, node, IP..."
        class="w-full px-3 py-2 bg-garuda-surface border border-garuda-border rounded-lg text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-garuda-primary transition"
      />
    </div>
  </div>

  {#if loading}
    <div class="flex items-center justify-center py-20">
      <div class="flex flex-col items-center gap-2">
        <div class="w-8 h-8 border-2 border-garuda-primary border-t-transparent rounded-full animate-spin"></div>
        <span class="text-xs text-garuda-muted font-mono">Memuat servers...</span>
      </div>
    </div>
  {:else if errorMsg}
    <div class="p-4 rounded-lg bg-rose-950/30 border border-rose-800/50 text-rose-300 text-sm flex items-center justify-between">
      <span>{errorMsg}</span>
      <button on:click={loadServers} class="px-3 py-1 bg-rose-900/50 hover:bg-rose-900 rounded text-xs">Coba Lagi</button>
    </div>
  {:else if filteredServers.length === 0}
    <div class="text-center py-20 bg-garuda-surface/50 border border-garuda-border/40 rounded-xl p-8">
      <div class="w-12 h-12 rounded-xl bg-garuda-card border border-garuda-border text-garuda-muted flex items-center justify-center mx-auto mb-3 font-mono text-lg">
        0
      </div>
      <h3 class="text-sm font-semibold text-slate-200">Belum ada server</h3>
      <p class="text-xs text-garuda-muted mt-1 max-w-sm mx-auto">
        Tidak ada server yang terdaftar pada akun ini atau cocok dengan pencarian Anda.
      </p>
    </div>
  {:else}
    <!-- Server Cards Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each filteredServers as s}
        <button
          type="button"
          on:click={() => navigate(`/servers/${s.id}/console`)}
          class="text-left bg-garuda-surface hover:bg-garuda-card/80 border border-garuda-border/70 hover:border-garuda-primary/60 rounded-xl p-5 shadow-lg transition-all duration-200 group relative overflow-hidden flex flex-col justify-between"
        >
          <!-- Top Accent line on hover -->
          <div class="absolute top-0 left-0 right-0 h-[2px] bg-garuda-primary opacity-0 group-hover:opacity-100 transition-opacity"></div>

          <div>
            <!-- Header Card: Name & Status badge -->
            <div class="flex items-start justify-between gap-3 mb-2">
              <div class="flex-1 min-w-0">
                <h3 class="font-bold text-slate-100 group-hover:text-garuda-primary transition-colors text-base truncate">
                  {s.name}
                </h3>
                <div class="text-[11px] font-mono text-garuda-muted truncate mt-0.5">
                  {s.ip ? `${s.ip}:${s.port}` : 'Belum ada port'}
                </div>
              </div>
              <span class={`text-[10px] font-mono px-2 py-0.5 rounded border uppercase tracking-wider ${getStatusColor(s.status)}`}>
                {s.status}
              </span>
            </div>

            <!-- Node / Location tag -->
            <div class="text-[11px] text-slate-400 flex items-center gap-1.5 mb-4">
              <span class="w-1.5 h-1.5 rounded-full bg-garuda-primary/60"></span>
              <span class="truncate">Node: {s.node_name || 'n/a'}</span>
            </div>
          </div>

          <!-- Bottom: Resource specs / mini metrics -->
          <div class="pt-3 border-t border-garuda-border/40 grid grid-cols-2 gap-2 text-xs font-mono">
            <div class="bg-garuda-bg/60 p-2 rounded border border-garuda-border/40">
              <div class="text-[10px] text-garuda-muted uppercase">Memory</div>
              <div class="font-semibold text-slate-200 mt-0.5">{s.memory_mb} MB</div>
            </div>
            <div class="bg-garuda-bg/60 p-2 rounded border border-garuda-border/40">
              <div class="text-[10px] text-garuda-muted uppercase">CPU Limit</div>
              <div class="font-semibold text-slate-200 mt-0.5">{s.cpu_percent}%</div>
            </div>
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>
