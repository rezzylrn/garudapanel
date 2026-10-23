<script>
  import { onMount } from 'svelte';
  import { apiFetch } from '../../stores.js';

  let locations = [];
  let loading = true;
  let errorMsg = '';
  let showCreateModal = false;

  // Form State
  let shortCode = '';
  let description = '';
  let submitting = false;

  async function loadLocations() {
    loading = true;
    errorMsg = '';
    try {
      const res = await apiFetch('/api/admin/locations');
      if (res.ok) {
        locations = await res.json();
      } else {
        errorMsg = 'Gagal memuat locations';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      loading = false;
    }
  }

  async function createLocation(e) {
    e.preventDefault();
    submitting = true;
    try {
      const res = await apiFetch('/api/admin/locations', {
        method: 'POST',
        body: { short_code: shortCode.trim().toLowerCase(), description: description.trim() },
      });
      if (res.ok) {
        showCreateModal = false;
        shortCode = '';
        description = '';
        loadLocations();
      } else {
        const err = await res.json();
        alert('Gagal: ' + (err.error || 'Error'));
      }
    } catch (e) {
      alert('Error: ' + e.message);
    } finally {
      submitting = false;
    }
  }

  async function deleteLocation(l) {
    if (!confirm(`Hapus location "${l.short_code}"?`)) return;
    try {
      const res = await apiFetch(`/api/admin/locations/${l.id}`, { method: 'DELETE' });
      if (res.ok) {
        loadLocations();
      } else {
        const err = await res.json();
        alert('Gagal menghapus location: ' + (err.error || 'Masih digunakan oleh Node'));
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  onMount(() => {
    loadLocations();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-garuda-border/50">
    <div>
      <h1 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <span>Locations Management</span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-garuda-card border border-garuda-border font-mono text-garuda-muted">
          {locations.length}
        </span>
      </h1>
      <p class="text-xs text-garuda-muted mt-0.5">Wilayah geografis / datacenter untuk pengelompokan node.</p>
    </div>

    <button
      on:click={() => (showCreateModal = true)}
      class="px-4 py-2 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg text-xs font-bold uppercase tracking-wider transition shadow-lg shadow-sky-600/20"
    >
      + Tambah Lokasi Baru
    </button>
  </div>

  {#if errorMsg}
    <div class="p-4 rounded-lg bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs font-mono">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat lokasi...</div>
  {:else if locations.length === 0}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono bg-garuda-surface border border-garuda-border/60 rounded-xl">
      Belum ada lokasi yang ditambahkan.
    </div>
  {:else}
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-garuda-border/60 bg-garuda-card/40 text-garuda-muted font-mono uppercase text-[10px]">
              <th class="py-3 px-4 w-16">ID</th>
              <th class="py-3 px-4 w-40">Short Code</th>
              <th class="py-3 px-4">Deskripsi / Datacenter</th>
              <th class="py-3 px-4 text-right w-28">Aksi</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-garuda-border/40 font-mono">
            {#each locations as l}
              <tr class="hover:bg-garuda-card/40 transition">
                <td class="py-3 px-4 text-garuda-muted">#{l.id}</td>
                <td class="py-3 px-4 font-bold text-sky-400 uppercase">{l.short_code}</td>
                <td class="py-3 px-4 text-slate-300">{l.description || '-'}</td>
                <td class="py-3 px-4 text-right">
                  <button
                    on:click={() => deleteLocation(l)}
                    class="px-2.5 py-1 bg-rose-950/40 hover:bg-rose-900/50 text-rose-300 border border-rose-800/50 rounded text-[11px]"
                  >
                    Hapus
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Modal Create Location -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border rounded-xl max-w-md w-full p-6 shadow-2xl relative font-mono text-xs">
        <h3 class="text-base font-bold text-slate-100 mb-1">Tambah Lokasi Baru</h3>
        <p class="text-garuda-muted mb-5">Kode singkat wilayah datacenter (misal: sg, id-jkt, us-east).</p>

        <form on:submit={createLocation} class="space-y-4">
          <div>
            <label for="loc-code" class="block text-slate-300 mb-1">Short Code</label>
            <input
              id="loc-code"
              type="text"
              bind:value={shortCode}
              required
              placeholder="sg-sin"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary uppercase"
            />
          </div>

          <div>
            <label for="loc-desc" class="block text-slate-300 mb-1">Deskripsi / Datacenter</label>
            <input
              id="loc-desc"
              type="text"
              bind:value={description}
              placeholder="Singapore Digital Ocean SGP1"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
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
              disabled={submitting}
              class="px-5 py-2 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg font-bold transition disabled:opacity-50"
            >
              {submitting ? 'Menyimpan...' : 'Simpan Lokasi'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>
