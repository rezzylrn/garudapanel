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
        errorMsg = 'Gagal memuat network allocations';
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
        successMsg = `Allocation ${alloc.ip}:${alloc.port} kini menjadi Primary port`;
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

<div class="space-y-4">
  <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-5 shadow-lg">
    <h3 class="font-bold text-slate-100 text-sm mb-1">Network & Port Allocations</h3>
    <p class="text-xs text-garuda-muted">
      Daftar IP dan Port yang dialokasikan untuk server game ini. Port default/primary digunakan sebagai endpoint koneksi utama pemain ke server game.
    </p>
  </div>

  {#if successMsg}
    <div class="p-3 bg-emerald-950/40 border border-emerald-800/60 rounded-xl text-xs text-emerald-300 font-mono">
      {successMsg}
    </div>
  {/if}

  {#if errorMsg}
    <div class="p-3 bg-rose-950/40 border border-rose-800/60 rounded-xl text-xs text-rose-300 font-mono">
      {errorMsg}
    </div>
  {/if}

  {#if loading}
    <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat network allocations...</div>
  {:else if allocations.length === 0}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono bg-garuda-surface border border-garuda-border/60 rounded-xl">
      Tidak ada port allocation yang terhubung ke server ini.
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {#each allocations as alloc}
        <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-5 shadow-lg flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between gap-2 mb-2">
              <span class="font-mono text-sm font-bold text-slate-100">{alloc.ip}</span>
              {#if alloc.is_primary}
                <span class="text-[9px] font-mono px-2 py-0.5 rounded bg-sky-950/70 text-garuda-primary border border-sky-800/70 font-semibold">
                  PRIMARY
                </span>
              {/if}
            </div>

            <div class="text-xs font-mono text-garuda-muted mb-4">
              Port: <span class="text-sky-400 font-bold">{alloc.port}</span>
            </div>
          </div>

          <div class="pt-3 border-t border-garuda-border/40 flex items-center justify-between">
            <span class="text-[10px] font-mono text-garuda-muted">Node ID: {alloc.node_id}</span>
            {#if !alloc.is_primary}
              <button
                on:click={() => makePrimary(alloc)}
                disabled={settingPrimary}
                class="px-3 py-1.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-300 hover:text-white rounded-lg text-xs font-medium transition disabled:opacity-50"
              >
                Jadikan Default
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>
