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
        // data.variables: [{id, name, value, default_value, user_editable, description}]
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
        successMsg = `Variabel ${v.name} berhasil disimpan`;
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

<div class="space-y-4">
  <!-- Info Header -->
  <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-5 shadow-lg">
    <h3 class="font-bold text-slate-100 text-sm mb-1">Konfigurasi Startup & Lingkungan Egg</h3>
    <p class="text-xs text-garuda-muted">
      Variabel environment yang diinjeksikan langsung ke dalam container server game. Hanya variabel bertanda <span class="text-emerald-400 font-semibold">User Editable</span> yang dapat dimodifikasi secara langsung.
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
    <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat variabel startup...</div>
  {:else if variables.length === 0}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono bg-garuda-surface border border-garuda-border/60 rounded-xl">
      Tidak ada variabel egg yang didefinisikan untuk server ini.
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      {#each variables as v}
        <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-5 shadow-lg flex flex-col justify-between">
          <div>
            <div class="flex items-center justify-between gap-2 mb-1.5">
              <span class="font-mono text-xs font-bold text-sky-400 tracking-wider uppercase">{v.name}</span>
              {#if v.user_editable}
                <span class="text-[9px] font-mono px-2 py-0.5 rounded bg-emerald-950/50 text-emerald-400 border border-emerald-800/50">
                  EDITABLE
                </span>
              {:else}
                <span class="text-[9px] font-mono px-2 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">
                  READONLY
                </span>
              {/if}
            </div>

            <p class="text-[11px] text-garuda-muted mb-3 min-h-[1.5rem]">
              {v.description || 'Tidak ada deskripsi untuk variabel ini.'}
            </p>

            <div class="space-y-1 mb-4">
              <label for={"var-" + v.name} class="text-[10px] uppercase font-mono text-slate-400 block">Nilai Variabel</label>
              <input
                id={"var-" + v.name}
                type="text"
                bind:value={v.value}
                disabled={!v.user_editable || saving[v.name]}
                placeholder={v.default_value || 'Kosong'}
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-xs font-mono text-slate-100 focus:outline-none focus:border-garuda-primary transition disabled:opacity-50 disabled:bg-slate-900/60"
              />
            </div>
          </div>

          <div class="pt-3 border-t border-garuda-border/40 flex items-center justify-between">
            <span class="text-[10px] font-mono text-garuda-muted truncate max-w-[150px]">
              Default: {v.default_value || '-'}
            </span>

            {#if v.user_editable}
              <button
                on:click={() => updateVariable(v)}
                disabled={saving[v.name]}
                class="px-3 py-1.5 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg text-xs font-medium transition disabled:opacity-50 shadow"
              >
                {saving[v.name] ? 'Menyimpan...' : 'Simpan'}
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>
