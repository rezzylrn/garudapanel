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
  <!-- Server Hero Header -->
  {#if loading}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono">Memuat server...</div>
  {:else if errorMsg}
    <div class="p-5 bg-rose-950/40 border border-rose-800/60 rounded-xl text-rose-300 text-sm flex items-center justify-between">
      <span>{errorMsg}</span>
      <button on:click={() => navigate('/servers')} class="px-3 py-1 bg-rose-900/60 hover:bg-rose-900 rounded text-xs">
        Daftar Server
      </button>
    </div>
  {:else if server}
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-5 shadow-xl">
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div class="flex items-center gap-3">
            <h1 class="text-xl font-black text-slate-100 tracking-wide">{server.name}</h1>
            <span class={`text-[10px] font-mono px-2.5 py-0.5 rounded border uppercase tracking-wider font-semibold ${getStatusColor(server.status)}`}>
              {server.status}
            </span>
          </div>

          <div class="flex flex-wrap items-center gap-4 text-xs font-mono text-garuda-muted mt-2">
            <span class="text-slate-300 flex items-center gap-1.5">
              <span class="w-1.5 h-1.5 rounded-full bg-garuda-primary"></span>
              <span>{server.ip ? `${server.ip}:${server.port}` : 'No Port'}</span>
            </span>
            <span>•</span>
            <span>Node: <span class="text-slate-300 font-semibold">{server.node_name || 'n/a'}</span></span>
            <span>•</span>
            <span>Egg: <span class="text-slate-300">{server.egg_name || 'Standard'}</span></span>
            <span>•</span>
            <span class="text-[10px] text-slate-500 truncate max-w-[200px]" title={server.uuid}>UUID: {server.uuid}</span>
          </div>
        </div>

        <!-- Back to list button -->
        <button
          on:click={() => navigate('/servers')}
          class="px-3 py-1.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border rounded-lg text-xs font-mono text-slate-300 transition self-start md:self-auto"
        >
          &larr; Semua Server
        </button>
      </div>

      <!-- Navigation Tabs -->
      <div class="flex items-center gap-1 mt-6 pt-4 border-t border-garuda-border/60 overflow-x-auto text-xs font-mono">
        <button
          on:click={() => setTab('console')}
          class={`px-4 py-2 rounded-lg font-medium transition whitespace-nowrap ${
            activeTab === 'console'
              ? 'bg-garuda-primary text-white shadow-md shadow-sky-500/20'
              : 'text-slate-400 hover:text-slate-200 hover:bg-garuda-card'
          }`}
        >
          Console
        </button>

        <button
          on:click={() => setTab('files')}
          class={`px-4 py-2 rounded-lg font-medium transition whitespace-nowrap ${
            activeTab === 'files'
              ? 'bg-garuda-primary text-white shadow-md shadow-sky-500/20'
              : 'text-slate-400 hover:text-slate-200 hover:bg-garuda-card'
          }`}
        >
          File Manager
        </button>

        <button
          on:click={() => setTab('startup')}
          class={`px-4 py-2 rounded-lg font-medium transition whitespace-nowrap ${
            activeTab === 'startup'
              ? 'bg-garuda-primary text-white shadow-md shadow-sky-500/20'
              : 'text-slate-400 hover:text-slate-200 hover:bg-garuda-card'
          }`}
        >
          Startup & Variables
        </button>

        <button
          on:click={() => setTab('network')}
          class={`px-4 py-2 rounded-lg font-medium transition whitespace-nowrap ${
            activeTab === 'network'
              ? 'bg-garuda-primary text-white shadow-md shadow-sky-500/20'
              : 'text-slate-400 hover:text-slate-200 hover:bg-garuda-card'
          }`}
        >
          Network
        </button>

        <button
          on:click={() => setTab('activity')}
          class={`px-4 py-2 rounded-lg font-medium transition whitespace-nowrap ${
            activeTab === 'activity'
              ? 'bg-garuda-primary text-white shadow-md shadow-sky-500/20'
              : 'text-slate-400 hover:text-slate-200 hover:bg-garuda-card'
          }`}
        >
          Activity
        </button>
      </div>
    </div>

    <!-- Tab Contents -->
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
