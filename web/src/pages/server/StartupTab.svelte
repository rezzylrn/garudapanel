<script>
  import { onMount } from 'svelte';
  import { apiFetch } from '../../stores.js';

  export let server;

  let variables = [];
  let loading = true;
  let saving = {};
  let errorMsg = '';
  let successMsg = '';

  async function loadVariables() {
    loading = true;
    errorMsg = '';
    try {
      const res = await apiFetch(`/api/servers/${server.id}/variables`);
      if (res.ok) {
        const data = await res.json();
        variables = data.variables || [];
      } else {
        errorMsg = 'Gagal memuat variabel startup';
      }
    } catch (e) {
      errorMsg = e.message || 'Koneksi error';
    } finally {
      loading = false;
    }
  }

  async function updateVariable(v) {
    saving[v.name] = true;
    errorMsg = '';
    successMsg = '';
    try {
      const res = await apiFetch(`/api/servers/${server.id}/variables/${encodeURIComponent(v.name)}`, {
        method: 'PATCH',
        body: { value: v.value },
      });
      if (res.ok) {
        successMsg = `Variabel ${v.name} disimpan`;
        setTimeout(() => (successMsg = ''), 3000);
      } else {
        const err = await res.json();
        errorMsg = err.error || 'Gagal mengubah variabel';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      saving[v.name] = false;
    }
  }

  onMount(() => {
    loadVariables();
  });
</script>

<div class="space-y-3 font-mono text-xs">
  <div class="border border-garuda-border bg-garuda-surface p-3 flex flex-col md:flex-row md:items-center justify-between gap-2">
    <div>
      <h3 class="font-bold text-slate-100 uppercase tracking-wider text-xs">Environment & Startup</h3>
      <p class="text-garuda-muted text-[11px] mt-0.5">
        Variabel runtime yang diinjeksikan langsung ke dalam container server.
      </p>
    </div>
    <button
      on:click={loadVariables}
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
    <div class="py-16 text-center text-garuda-muted">Memuat variabel startup...</div>
  {:else if variables.length === 0}
    <div class="py-12 text-center text-garuda-muted border border-garuda-border bg-garuda-surface">
      Tidak ada variabel egg yang didefinisikan untuk instance ini.
    </div>
  {:else}
    <div class="border border-garuda-border bg-garuda-surface divide-y divide-garuda-border">
      {#each variables as v}
        <div class="p-3 flex flex-col md:flex-row md:items-center justify-between gap-3 hover:bg-garuda-hover">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span class="font-bold text-slate-100 uppercase">{v.name}</span>
              {#if v.user_editable}
                <span class="text-[9px] px-1 py-0.2 border border-garuda-border bg-garuda-card text-emerald-400">
                  EDITABLE
                </span>
              {:else}
                <span class="text-[9px] px-1 py-0.2 border border-garuda-border bg-garuda-card text-garuda-muted">
                  READONLY
                </span>
              {/if}
            </div>

            <div class="text-[11px] text-garuda-muted truncate">
              {v.description || 'Tanpa deskripsi'}
            </div>
            <div class="text-[10px] text-garuda-border mt-0.5">
              Default: {v.default_value || '-'}
            </div>
          </div>

          <div class="flex items-center gap-2 w-full md:w-auto">
            <input
              type="text"
              bind:value={v.value}
              disabled={!v.user_editable || saving[v.name]}
              placeholder={v.default_value || 'Kosong'}
              class="w-full md:w-64 px-2.5 py-1 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent disabled:opacity-50"
            />

            {#if v.user_editable}
              <button
                on:click={() => updateVariable(v)}
                disabled={saving[v.name]}
                class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 uppercase tracking-wider whitespace-nowrap disabled:opacity-50"
              >
                {saving[v.name] ? '...' : 'Simpan'}
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>
