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
    if (!confirm(`Hapus egg "${egg.name}"? Server yang menggunakan egg ini tidak dapat dihapus egg-nya.`)) return;
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

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-garuda-border/50">
    <div>
      <h1 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <span>Eggs Configuration</span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-garuda-card border border-garuda-border font-mono text-garuda-muted">
          {eggs.length}
        </span>
      </h1>
      <p class="text-xs text-garuda-muted mt-0.5">Template container Docker, perintah startup, dan variabel game.</p>
    </div>

    <button
      on:click={() => (showCreateModal = true)}
      class="px-4 py-2 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg text-xs font-bold uppercase tracking-wider transition shadow-lg shadow-sky-600/20"
    >
      + Tambah Egg Baru
    </button>
  </div>

  {#if errorMsg}
    <div class="p-4 rounded-lg bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs font-mono">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat eggs...</div>
  {:else if eggs.length === 0}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono bg-garuda-surface border border-garuda-border/60 rounded-xl">
      Belum ada template egg.
    </div>
  {:else}
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-garuda-border/60 bg-garuda-card/40 text-garuda-muted font-mono uppercase text-[10px]">
              <th class="py-3 px-4 w-14">ID</th>
              <th class="py-3 px-4">Nama Egg</th>
              <th class="py-3 px-4">Nest Category</th>
              <th class="py-3 px-4">Docker Image</th>
              <th class="py-3 px-4">Stop Signal</th>
              <th class="py-3 px-4 text-right">Aksi</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-garuda-border/40 font-mono">
            {#each eggs as eg}
              <tr class="hover:bg-garuda-card/40 transition">
                <td class="py-3 px-4 text-garuda-muted">#{eg.id}</td>
                <td class="py-3 px-4 font-bold text-slate-200">{eg.name}</td>
                <td class="py-3 px-4 text-slate-300">
                  {nests.find((n) => n.id === eg.nest_id)?.name || `Nest #${eg.nest_id}`}
                </td>
                <td class="py-3 px-4 text-sky-400 truncate max-w-[200px]">{eg.docker_image}</td>
                <td class="py-3 px-4 text-garuda-muted">{eg.stop_signal || 'SIGTERM'}</td>
                <td class="py-3 px-4 text-right">
                  <div class="flex items-center justify-end gap-2">
                    <button
                      on:click={() => openVariables(eg)}
                      class="px-2.5 py-1 bg-sky-950/40 hover:bg-sky-900/50 text-sky-300 border border-sky-800/50 rounded text-[11px]"
                    >
                      Variables
                    </button>
                    <button
                      on:click={() => deleteEgg(eg)}
                      class="px-2.5 py-1 bg-rose-950/40 hover:bg-rose-900/50 text-rose-300 border border-rose-800/50 rounded text-[11px]"
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
    </div>
  {/if}

  <!-- Modal Create Egg -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border rounded-xl max-w-lg w-full p-6 shadow-2xl relative max-h-[90vh] overflow-y-auto">
        <h3 class="text-base font-bold text-slate-100 mb-1">Tambah Template Egg Baru</h3>
        <p class="text-xs text-garuda-muted mb-5">Konfigurasi container runtime untuk jenis server game.</p>

        <form on:submit={createEgg} class="space-y-4 font-mono text-xs">
          <div>
            <label for="egg-name" class="block text-slate-300 mb-1">Nama Egg</label>
            <input
              id="egg-name"
              type="text"
              bind:value={name}
              required
              placeholder="PaperMC 1.20"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
          </div>

          <div>
            <label for="egg-nest" class="block text-slate-300 mb-1">Nest</label>
            <select
              id="egg-nest"
              bind:value={nestId}
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            >
              {#each nests as n}
                <option value={n.id}>{n.name}</option>
              {/each}
            </select>
          </div>

          <div>
            <label for="egg-img" class="block text-slate-300 mb-1">Docker Image</label>
            <input
              id="egg-img"
              type="text"
              bind:value={dockerImage}
              required
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
          </div>

          <div>
            <label for="egg-cmd" class="block text-slate-300 mb-1">Startup Command</label>
            <input
              id="egg-cmd"
              type="text"
              bind:value={startupCmd}
              required
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="egg-sig" class="block text-slate-300 mb-1">Stop Signal</label>
              <input
                id="egg-sig"
                type="text"
                bind:value={stopSignal}
                class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
              />
            </div>
          </div>

          <div>
            <label for="egg-script" class="block text-slate-300 mb-1">Install Script (Bash)</label>
            <textarea
              id="egg-script"
              bind:value={installScript}
              rows="4"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            ></textarea>
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
              {submitting ? 'Menyimpan...' : 'Buat Egg'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- Modal Egg Variables -->
  {#if showVarModal && targetEgg}
    <div class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border rounded-xl max-w-xl w-full p-6 shadow-2xl relative max-h-[90vh] flex flex-col font-mono text-xs">
        <div class="flex items-center justify-between pb-3 border-b border-garuda-border/60">
          <div>
            <h3 class="text-base font-bold text-slate-100">Variables for Egg: {targetEgg.name}</h3>
            <p class="text-[11px] text-garuda-muted">Variabel environment template yang akan di-clone ke server baru.</p>
          </div>
          <button on:click={() => (showVarModal = false)} class="text-slate-400 hover:text-white text-lg font-bold">
            &times;
          </button>
        </div>

        <!-- Add Variable Form -->
        <form on:submit={addVariable} class="py-4 border-b border-garuda-border/40 space-y-3">
          <div class="grid grid-cols-2 gap-2">
            <input
              type="text"
              bind:value={newVarName}
              placeholder="NAMA_VAR (contoh: MOTD)"
              required
              class="px-3 py-1.5 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 uppercase"
            />
            <input
              type="text"
              bind:value={newVarDefault}
              placeholder="Nilai Default"
              class="px-3 py-1.5 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100"
            />
          </div>

          <input
            type="text"
            bind:value={newVarDesc}
            placeholder="Deskripsi singkat variabel..."
            class="w-full px-3 py-1.5 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100"
          />

          <div class="flex items-center justify-between">
            <div class="flex items-center gap-4">
              <label class="flex items-center gap-1.5 text-slate-300 cursor-pointer">
                <input type="checkbox" bind:checked={newVarViewable} />
                <span>Viewable</span>
              </label>
              <label class="flex items-center gap-1.5 text-slate-300 cursor-pointer">
                <input type="checkbox" bind:checked={newVarEditable} />
                <span>Editable</span>
              </label>
            </div>
            <button
              type="submit"
              disabled={addingVar}
              class="px-4 py-1.5 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg font-bold transition disabled:opacity-50"
            >
              + Tambah Variabel
            </button>
          </div>
        </form>

        <!-- Variable List -->
        <div class="flex-1 overflow-y-auto py-2 space-y-2">
          {#if loadingVars}
            <div class="py-8 text-center text-garuda-muted">Memuat...</div>
          {:else if eggVars.length === 0}
            <div class="py-8 text-center text-garuda-muted">Belum ada variabel untuk egg ini.</div>
          {:else}
            {#each eggVars as ev}
              <div class="p-3 bg-garuda-bg border border-garuda-border/60 rounded-lg flex items-center justify-between gap-3">
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="font-bold text-sky-400">{ev.name}</span>
                    <span class="text-garuda-muted text-[10px]">({ev.default_value || 'empty'})</span>
                    {#if ev.user_editable}
                      <span class="text-[9px] px-1 bg-emerald-950 text-emerald-400 border border-emerald-800 rounded">EDIT</span>
                    {/if}
                  </div>
                  <div class="text-[11px] text-garuda-muted truncate mt-0.5">{ev.description || 'No desc'}</div>
                </div>
                <button
                  on:click={() => deleteVariable(ev.id)}
                  class="px-2 py-1 bg-rose-950/40 text-rose-300 hover:bg-rose-900 rounded text-[10px]"
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
