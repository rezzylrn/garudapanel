<script>
  import { onMount } from 'svelte';
  import { apiFetch, navigate } from '../stores.js';
  import ConsoleTab from './server/ConsoleTab.svelte';
  import FilesTab from './server/FilesTab.svelte';
  import StartupTab from './server/StartupTab.svelte';
  import NetworkTab from './server/NetworkTab.svelte';
  import ActivityTab from './server/ActivityTab.svelte';

  export let serverId;
  export let serverTab = 'console';

  let server = null;
  let loading = true;
  let errorMsg = '';

  async function loadServer() {
    loading = true;
    errorMsg = '';
    try {
      const res = await apiFetch(`/api/servers/${serverId}`);
      if (res.ok) {
        server = await res.json();
      } else {
        errorMsg = 'Gagal memuat informasi server';
      }
    } catch (e) {
      errorMsg = e.message || 'Koneksi gagal';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadServer();
  });

  $: activeTab = serverTab || 'console';

  function setTab(tab) {
    navigate(`/servers/${serverId}/${tab}`);
  }

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
  {#if loading}
    <div class="py-16 text-center text-xs font-mono text-garuda-muted">
      Memuat informasi instance...
    </div>
  {:else if errorMsg}
    <div class="p-3 bg-garuda-surface border-l-2 border-garuda-danger text-xs font-mono text-rose-400 flex items-center justify-between">
      <span>{errorMsg}</span>
      <button on:click={() => navigate('/servers')} class="px-2 py-0.5 border border-garuda-border text-slate-300 hover:bg-garuda-hover">
        Kembali ke Servers
      </button>
    </div>
  {:else if server}
    <!-- Server Metadata Strip -->
    <div class="border border-garuda-border bg-garuda-surface">
      <!-- Title & Actions Row -->
      <div class="p-3 border-b border-garuda-border flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div class="flex items-center gap-3">
          <span class={`px-2 py-0.5 border text-xs font-mono uppercase font-bold tracking-wider ${getStatusBadge(server.status)}`}>
            {server.status}
          </span>
          <h1 class="text-sm font-mono font-bold text-slate-100">
            {server.name}
          </h1>
          <span class="text-xs font-mono text-garuda-muted">
            #{server.id}
          </span>
        </div>

        <button
          on:click={() => navigate('/servers')}
          class="self-start md:self-auto px-2.5 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-xs font-mono text-garuda-muted hover:text-slate-200"
        >
          &larr; Daftar Server
        </button>
      </div>

      <!-- Specs Bar -->
      <div class="px-3 py-2 bg-garuda-card text-xs font-mono text-slate-300 flex flex-wrap items-center gap-x-6 gap-y-1">
        <div>
          <span class="text-garuda-muted">Endpoint:</span>
          <span class="text-slate-100 ml-1">{server.ip ? `${server.ip}:${server.port}` : 'Belum teralokasi'}</span>
        </div>
        <div>
          <span class="text-garuda-muted">Node:</span>
          <span class="text-slate-100 ml-1">{server.node_name || '-'}</span>
        </div>
        <div>
          <span class="text-garuda-muted">Egg:</span>
          <span class="text-slate-100 ml-1">{server.egg_name || 'Standard'}</span>
        </div>
        <div>
          <span class="text-garuda-muted">RAM/Disk:</span>
          <span class="text-slate-100 ml-1">{server.memory_mb} MB / {server.disk_mb} MB</span>
        </div>
        <div>
          <span class="text-garuda-muted">CPU:</span>
          <span class="text-slate-100 ml-1">{server.cpu_percent}%</span>
        </div>
        <div class="text-garuda-muted truncate max-w-xs" title={server.uuid}>
          UUID: {server.uuid}
        </div>
      </div>

      <!-- Navigation Tabs Strip -->
      <div class="flex items-center border-t border-garuda-border overflow-x-auto text-xs font-mono bg-garuda-surface">
        <button
          on:click={() => setTab('console')}
          class={`px-3 py-2 border-r border-garuda-border transition whitespace-nowrap ${
            activeTab === 'console'
              ? 'bg-garuda-card text-slate-100 font-bold border-b-2 border-b-garuda-accent'
              : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
          }`}
        >
          Console
        </button>

        <button
          on:click={() => setTab('files')}
          class={`px-3 py-2 border-r border-garuda-border transition whitespace-nowrap ${
            activeTab === 'files'
              ? 'bg-garuda-card text-slate-100 font-bold border-b-2 border-b-garuda-accent'
              : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
          }`}
        >
          Files
        </button>

        <button
          on:click={() => setTab('startup')}
          class={`px-3 py-2 border-r border-garuda-border transition whitespace-nowrap ${
            activeTab === 'startup'
              ? 'bg-garuda-card text-slate-100 font-bold border-b-2 border-b-garuda-accent'
              : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
          }`}
        >
          Startup
        </button>

        <button
          on:click={() => setTab('network')}
          class={`px-3 py-2 border-r border-garuda-border transition whitespace-nowrap ${
            activeTab === 'network'
              ? 'bg-garuda-card text-slate-100 font-bold border-b-2 border-b-garuda-accent'
              : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
          }`}
        >
          Network
        </button>

        <button
          on:click={() => setTab('activity')}
          class={`px-3 py-2 border-r border-garuda-border transition whitespace-nowrap ${
            activeTab === 'activity'
              ? 'bg-garuda-card text-slate-100 font-bold border-b-2 border-b-garuda-accent'
              : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
          }`}
        >
          Activity
        </button>
      </div>
    </div>

    <!-- Tab View Container -->
    <div>
      {#if activeTab === 'console'}
        <ConsoleTab {server} />
      {:else if activeTab === 'files'}
        <FilesTab {server} />
      {:else if activeTab === 'startup'}
        <StartupTab {server} />
      {:else if activeTab === 'network'}
        <NetworkTab {server} />
      {:else if activeTab === 'activity'}
        <ActivityTab {server} />
      {/if}
    </div>
  {/if}
</div>
