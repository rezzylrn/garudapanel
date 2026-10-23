<script>
  import { onMount } from 'svelte';
  import { apiFetch } from '../../stores.js';

  let eggs = [];
  let nests = [];
  let loading = true;
  let errorMsg = '';
  let showCreateModal = false;

  // Form New Egg
  let name = '';
  let nestId = 1;
  let dockerImage = 'ghcr.io/pterodactyl/yolks:java_17';
  let startupCmd = 'java -Xms128M -Xmx{{SERVER_MEMORY}}M -jar server.jar';
  let stopSignal = 'SIGTERM';
  let installScript = '#!/bin/ash\necho "installing..."';
  let submitting = false;

  // Variables Management Modal
  let showVarModal = false;
  let targetEgg = null;
  let eggVars = [];
  let loadingVars = false;
  let newVarName = '';
  let newVarDefault = '';
  let newVarViewable = true;
  let newVarEditable = true;
  let newVarDesc = '';
  let addingVar = false;

  async function loadData() {
    loading = true;
    errorMsg = '';
    try {
      const [resE, resN] = await Promise.all([
        apiFetch('/api/admin/eggs'),
        apiFetch('/api/admin/nests'),
      ]);
      if (resE.ok && resN.ok) {
        eggs = await resE.json();
        nests = await resN.json();
        if (nests.length > 0 && !nestId) {
          nestId = nests[0].id;
        }
      } else {
        errorMsg = 'Gagal memuat eggs atau nests';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      loading = false;
    }
  }

  async function createEgg(e) {
    e.preventDefault();
    submitting = true;
    try {
      const res = await apiFetch('/api/admin/eggs', {
        method: 'POST',
        body: {
          name,
          nest_id: Number(nestId),
          docker_image: dockerImage,
          startup_cmd: startupCmd,
          stop_signal: stopSignal,
          install_script: installScript,
        },
      });
      if (res.ok) {
        showCreateModal = false;
        name = '';
        loadData();
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

  async function deleteEgg(egg) {
    if (!confirm(`Hapus egg "${egg.name}"?`)) return;
    try {
      const res = await apiFetch(`/api/admin/eggs/${egg.id}`, { method: 'DELETE' });
      if (res.ok) {
        loadData();
      } else {
        const err = await res.json();
        alert('Gagal menghapus egg: ' + (err.error || 'Error'));
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  async function openVariables(egg) {
    targetEgg = egg;
    showVarModal = true;
    loadEggVariables(egg.id);
  }

  async function loadEggVariables(eggId) {
    loadingVars = true;
    try {
      const res = await apiFetch(`/api/admin/eggs/${eggId}`);
      if (res.ok) {
        const data = await res.json();
        eggVars = data.variables || [];
      }
    } catch (e) {
    } finally {
      loadingVars = false;
    }
  }

  async function addVariable(e) {
    e.preventDefault();
    if (!newVarName.trim()) return;
    addingVar = true;
    try {
      const res = await apiFetch(`/api/admin/eggs/${targetEgg.id}/variables`, {
        method: 'POST',
        body: {
          name: newVarName.trim().toUpperCase(),
          default_value: newVarDefault,
          user_viewable: newVarViewable,
          user_editable: newVarEditable,
          description: newVarDesc,
        },
      });
      if (res.ok) {
        newVarName = '';
        newVarDefault = '';
        newVarDesc = '';
        loadEggVariables(targetEgg.id);
      } else {
        const err = await res.json();
        alert('Gagal: ' + (err.error || 'Error'));
      }
    } catch (e) {
      alert('Error: ' + e.message);
    } finally {
      addingVar = false;
    }
  }

  async function deleteVariable(vid) {
    if (!confirm('Hapus variabel ini dari egg?')) return;
    try {
      const res = await apiFetch(`/api/admin/eggs/${targetEgg.id}/variables/${vid}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        loadEggVariables(targetEgg.id);
      } else {
        alert('Gagal menghapus variabel');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  onMount(() => {
    loadData();
  });
</script>

<div class="space-y-4 font-mono text-xs">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-garuda-border">
    <div class="flex items-baseline gap-3">
      <h1 class="text-sm font-bold uppercase tracking-wider text-slate-100">
        Eggs
      </h1>
      <span class="text-garuda-muted text-xs">Total: {eggs.length}</span>
    </div>

    <button
      on:click={() => (showCreateModal = true)}
      class="self-start sm:self-auto px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 uppercase tracking-wider"
    >
      + Tambah Egg
    </button>
  </div>

  {#if errorMsg}
    <div class="p-2 border-l-2 border-garuda-danger bg-garuda-surface text-rose-400">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-16 text-center text-garuda-muted">Memuat daftar egg...</div>
  {:else if eggs.length === 0}
    <div class="py-12 text-center text-garuda-muted border border-garuda-border bg-garuda-surface">
      Belum ada template egg.
    </div>
  {:else}
    <div class="border border-garuda-border bg-garuda-surface overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
            <th class="py-2 px-3 w-16">ID</th>
            <th class="py-2 px-3">Nama Egg</th>
            <th class="py-2 px-3">Nest Category</th>
            <th class="py-2 px-3">Docker Image</th>
            <th class="py-2 px-3">Stop Signal</th>
            <th class="py-2 px-3 text-right">Aksi</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-garuda-border">
          {#each eggs as eg}
            <tr class="hover:bg-garuda-hover">
              <td class="py-2 px-3 text-garuda-muted">#{eg.id}</td>
              <td class="py-2 px-3 font-semibold text-slate-100">{eg.name}</td>
              <td class="py-2 px-3 text-slate-300">
                {nests.find((n) => n.id === eg.nest_id)?.name || `Nest #${eg.nest_id}`}
              </td>
              <td class="py-2 px-3 text-slate-300 truncate max-w-[200px]">{eg.docker_image}</td>
              <td class="py-2 px-3 text-garuda-muted">{eg.stop_signal || 'SIGTERM'}</td>
              <td class="py-2 px-3 text-right">
                <div class="flex items-center justify-end gap-1">
                  <button
                    on:click={() => openVariables(eg)}
                    class="px-2 py-0.5 border border-garuda-border bg-garuda-card hover:bg-garuda-hover text-slate-200 text-[11px]"
                  >
                    Variables
                  </button>
                  <button
                    on:click={() => deleteEgg(eg)}
                    class="px-2 py-0.5 border border-garuda-border bg-garuda-card hover:bg-garuda-hover text-rose-400 text-[11px]"
                  >
                    Hapus
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <!-- Modal Create Egg -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border max-w-lg w-full p-5 max-h-[90vh] overflow-y-auto">
        <div class="flex items-center justify-between pb-2 mb-3 border-b border-garuda-border">
          <h3 class="font-bold text-slate-100 uppercase tracking-wider">Tambah Template Egg</h3>
          <button on:click={() => (showCreateModal = false)} class="text-garuda-muted hover:text-white">&times;</button>
        </div>

        <form on:submit={createEgg} class="space-y-3">
          <div>
            <label for="egg-name" class="block text-[11px] uppercase text-garuda-muted mb-1">Nama Egg</label>
            <input
              id="egg-name"
              type="text"
              bind:value={name}
              required
              placeholder="PaperMC 1.20"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div>
            <label for="egg-nest" class="block text-[11px] uppercase text-garuda-muted mb-1">Nest</label>
            <select
              id="egg-nest"
              bind:value={nestId}
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
            >
              {#each nests as n}
                <option value={n.id}>{n.name}</option>
              {/each}
            </select>
          </div>

          <div>
            <label for="egg-img" class="block text-[11px] uppercase text-garuda-muted mb-1">Docker Image</label>
            <input
              id="egg-img"
              type="text"
              bind:value={dockerImage}
              required
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div>
            <label for="egg-cmd" class="block text-[11px] uppercase text-garuda-muted mb-1">Startup Command</label>
            <input
              id="egg-cmd"
              type="text"
              bind:value={startupCmd}
              required
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div>
            <label for="egg-sig" class="block text-[11px] uppercase text-garuda-muted mb-1">Stop Signal</label>
            <input
              id="egg-sig"
              type="text"
              bind:value={stopSignal}
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div>
            <label for="egg-script" class="block text-[11px] uppercase text-garuda-muted mb-1">Install Script (Bash)</label>
            <textarea
              id="egg-script"
              bind:value={installScript}
              rows="4"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 focus:outline-none focus:border-garuda-accent"
            ></textarea>
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

  <!-- Modal Egg Variables -->
  {#if showVarModal && targetEgg}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border max-w-xl w-full p-5 max-h-[90vh] flex flex-col">
        <div class="flex items-center justify-between pb-2 border-b border-garuda-border">
          <div>
            <h3 class="font-bold text-slate-100 uppercase tracking-wider">Variabel Egg: {targetEgg.name}</h3>
            <p class="text-[11px] text-garuda-muted">Template variabel environment untuk server baru.</p>
          </div>
          <button on:click={() => (showVarModal = false)} class="text-garuda-muted hover:text-white">&times;</button>
        </div>

        <!-- Add Variable Form -->
        <form on:submit={addVariable} class="py-3 border-b border-garuda-border space-y-2">
          <div class="grid grid-cols-2 gap-2">
            <input
              type="text"
              bind:value={newVarName}
              placeholder="NAMA_VAR"
              required
              class="px-2.5 py-1 bg-garuda-bg border border-garuda-border text-slate-100 uppercase placeholder-garuda-border"
            />
            <input
              type="text"
              bind:value={newVarDefault}
              placeholder="Nilai Default"
              class="px-2.5 py-1 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border"
            />
          </div>

          <input
            type="text"
            bind:value={newVarDesc}
            placeholder="Deskripsi variabel..."
            class="w-full px-2.5 py-1 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border"
          />

          <div class="flex items-center justify-between pt-1">
            <div class="flex items-center gap-4 text-slate-300">
              <label class="flex items-center gap-1.5 cursor-pointer">
                <input type="checkbox" bind:checked={newVarViewable} />
                <span>Viewable</span>
              </label>
              <label class="flex items-center gap-1.5 cursor-pointer">
                <input type="checkbox" bind:checked={newVarEditable} />
                <span>Editable</span>
              </label>
            </div>
            <button
              type="submit"
              disabled={addingVar}
              class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 uppercase tracking-wider disabled:opacity-50"
            >
              + Tambah Variabel
            </button>
          </div>
        </form>

        <!-- Variable List -->
        <div class="flex-1 overflow-y-auto py-2 space-y-1.5">
          {#if loadingVars}
            <div class="py-6 text-center text-garuda-muted">Memuat...</div>
          {:else if eggVars.length === 0}
            <div class="py-6 text-center text-garuda-muted">Belum ada variabel untuk egg ini.</div>
          {:else}
            {#each eggVars as ev}
              <div class="p-2 bg-garuda-bg border border-garuda-border flex items-center justify-between gap-3">
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="font-bold text-slate-100 uppercase">{ev.name}</span>
                    <span class="text-garuda-muted text-[10px]">({ev.default_value || '-'})</span>
                    {#if ev.user_editable}
                      <span class="text-[9px] px-1 border border-garuda-border bg-garuda-card text-emerald-400">EDIT</span>
                    {/if}
                  </div>
                  <div class="text-[11px] text-garuda-muted truncate mt-0.5">{ev.description || 'Tanpa deskripsi'}</div>
                </div>
                <button
                  on:click={() => deleteVariable(ev.id)}
                  class="px-2 py-0.5 border border-garuda-border bg-garuda-card hover:bg-garuda-hover text-rose-400 text-[10px]"
                >
                  Hapus
                </button>
              </div>
            {/each}
          {/if}
        </div>
      </div>
    </div>
  {/if}
</div>
