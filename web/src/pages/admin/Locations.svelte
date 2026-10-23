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

<div class="space-y-4 font-mono text-xs">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-garuda-border">
    <div class="flex items-baseline gap-3">
      <h1 class="text-sm font-bold uppercase tracking-wider text-slate-100">
        Locations
      </h1>
      <span class="text-garuda-muted text-xs">Total: {locations.length}</span>
    </div>

    <button
      on:click={() => (showCreateModal = true)}
      class="self-start sm:self-auto px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 uppercase tracking-wider"
    >
      + Tambah Lokasi
    </button>
  </div>

  {#if errorMsg}
    <div class="p-2 border-l-2 border-garuda-danger bg-garuda-surface text-rose-400">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-16 text-center text-garuda-muted">Memuat daftar lokasi...</div>
  {:else if locations.length === 0}
    <div class="py-12 text-center text-garuda-muted border border-garuda-border bg-garuda-surface">
      Belum ada lokasi yang ditambahkan.
    </div>
  {:else}
    <div class="border border-garuda-border bg-garuda-surface overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
            <th class="py-2 px-3 w-16">ID</th>
            <th class="py-2 px-3 w-40">Short Code</th>
            <th class="py-2 px-3">Deskripsi / Datacenter</th>
            <th class="py-2 px-3 text-right w-24">Aksi</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-garuda-border">
          {#each locations as l}
            <tr class="hover:bg-garuda-hover">
              <td class="py-2 px-3 text-garuda-muted">#{l.id}</td>
              <td class="py-2 px-3 font-semibold text-slate-100 uppercase">{l.short_code}</td>
              <td class="py-2 px-3 text-slate-300">{l.description || '-'}</td>
              <td class="py-2 px-3 text-right">
                <button
                  on:click={() => deleteLocation(l)}
                  class="px-2 py-0.5 border border-garuda-border bg-garuda-card hover:bg-garuda-hover text-rose-400 text-[11px]"
                >
                  Hapus
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <!-- Modal Create Location -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border max-w-md w-full p-5">
        <div class="flex items-center justify-between pb-2 mb-3 border-b border-garuda-border">
          <h3 class="font-bold text-slate-100 uppercase tracking-wider">Tambah Lokasi Datacenter</h3>
          <button on:click={() => (showCreateModal = false)} class="text-garuda-muted hover:text-white">&times;</button>
        </div>

        <form on:submit={createLocation} class="space-y-3">
          <div>
            <label for="loc-code" class="block text-[11px] uppercase text-garuda-muted mb-1">Short Code</label>
            <input
              id="loc-code"
              type="text"
              bind:value={shortCode}
              required
              placeholder="id-jkt"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 uppercase placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div>
            <label for="loc-desc" class="block text-[11px] uppercase text-garuda-muted mb-1">Deskripsi / Datacenter</label>
            <input
              id="loc-desc"
              type="text"
              bind:value={description}
              placeholder="Jakarta DC-01"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
            />
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
              disabled={submitting}
              class="px-4 py-1.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-100 uppercase tracking-wider font-semibold disabled:opacity-50"
            >
              {submitting ? 'Menyimpan...' : 'Simpan'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>
