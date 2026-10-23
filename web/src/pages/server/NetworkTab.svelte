<script>
  import { onMount } from 'svelte';
  import { apiFetch } from '../../stores.js';

  export let server;

  let allocations = [];
  let loading = true;
  let settingPrimary = false;
  let errorMsg = '';
  let successMsg = '';

  async function loadAllocations() {
    loading = true;
    errorMsg = '';
    try {
      const res = await apiFetch(`/api/servers/${server.id}/allocations`);
      if (res.ok) {
        allocations = await res.json();
      } else {
        errorMsg = 'Gagal memuat alokasi port';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      loading = false;
    }
  }

  async function makePrimary(alloc) {
    if (alloc.is_primary) return;
    settingPrimary = true;
    errorMsg = '';
    successMsg = '';
    try {
      const res = await apiFetch(`/api/servers/${server.id}/allocations/primary`, {
        method: 'POST',
        body: { allocation_id: alloc.id },
      });
      if (res.ok) {
        successMsg = `Port ${alloc.ip}:${alloc.port} dijadikan Primary`;
        server.ip = alloc.ip;
        server.port = alloc.port;
        loadAllocations();
        setTimeout(() => (successMsg = ''), 3000);
      } else {
        const err = await res.json();
        errorMsg = err.error || 'Gagal mengubah primary allocation';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      settingPrimary = false;
    }
  }

  onMount(() => {
    loadAllocations();
  });
</script>

<div class="space-y-3 font-mono text-xs">
  <div class="border border-garuda-border bg-garuda-surface p-3 flex flex-col md:flex-row md:items-center justify-between gap-2">
    <div>
      <h3 class="font-bold text-slate-100 uppercase tracking-wider text-xs">Network & Port Allocations</h3>
      <p class="text-garuda-muted text-[11px] mt-0.5">
        Port dan IP address yang dialokasikan dari node worker untuk server ini.
      </p>
    </div>
    <button
      on:click={loadAllocations}
      class="self-start md:self-auto px-2.5 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-garuda-muted hover:text-slate-200"
    >
      Refresh
    </button>
  </div>

  {#if successMsg}
    <div class="p-2 border-l-2 border-garuda-accent bg-garuda-surface text-slate-200">
      {successMsg}
    </div>
  {/if}

  {#if errorMsg}
    <div class="p-2 border-l-2 border-garuda-danger bg-garuda-surface text-rose-400">
      {errorMsg}
    </div>
  {/if}

  {#if loading}
    <div class="py-16 text-center text-garuda-muted">Memuat alokasi jaringan...</div>
  {:else if allocations.length === 0}
    <div class="py-12 text-center text-garuda-muted border border-garuda-border bg-garuda-surface">
      Tidak ada port allocation yang terhubung ke server ini.
    </div>
  {:else}
    <div class="border border-garuda-border bg-garuda-surface overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
            <th class="py-2 px-3">IP Address</th>
            <th class="py-2 px-3">Port</th>
            <th class="py-2 px-3">Tipe</th>
            <th class="py-2 px-3">Node ID</th>
            <th class="py-2 px-3 text-right">Aksi</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-garuda-border">
          {#each allocations as alloc}
            <tr class="hover:bg-garuda-hover">
              <td class="py-2 px-3 font-semibold text-slate-100">{alloc.ip}</td>
              <td class="py-2 px-3 text-slate-200">{alloc.port}</td>
              <td class="py-2 px-3">
                {#if alloc.is_primary}
                  <span class="px-1.5 py-0.5 border border-garuda-border bg-garuda-card text-emerald-400 text-[10px] uppercase font-bold">
                    PRIMARY
                  </span>
                {:else}
                  <span class="text-garuda-muted text-[10px] uppercase">SECONDARY</span>
                {/if}
              </td>
              <td class="py-2 px-3 text-garuda-muted">#{alloc.node_id}</td>
              <td class="py-2 px-3 text-right">
                {#if !alloc.is_primary}
                  <button
                    on:click={() => makePrimary(alloc)}
                    disabled={settingPrimary}
                    class="px-2 py-0.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 text-[11px] disabled:opacity-50"
                  >
                    Set Primary
                  </button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
