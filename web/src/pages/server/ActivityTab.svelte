<script>
  import { onMount } from 'svelte';
  import { apiFetch } from '../../stores.js';

  export let server;

  let logs = [];
  let loading = true;
  let errorMsg = '';

  async function loadActivity() {
    loading = true;
    errorMsg = '';
    try {
      const res = await apiFetch(`/api/servers/${server.id}/activity`);
      if (res.ok) {
        logs = await res.json();
      } else {
        errorMsg = 'Gagal memuat log aktivitas';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      loading = false;
    }
  }

  function formatTime(unixSec) {
    if (!unixSec) return '-';
    return new Date(unixSec * 1000).toLocaleString('id-ID');
  }

  onMount(() => {
    loadActivity();
  });
</script>

<div class="space-y-4">
  <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-5 shadow-lg flex items-center justify-between">
    <div>
      <h3 class="font-bold text-slate-100 text-sm mb-1">Riwayat Aktivitas & Audit Log</h3>
      <p class="text-xs text-garuda-muted">
        Catatan tindakan yang dilakukan pada server ini (power action, modifikasi file, perubahan konfigurasi).
      </p>
    </div>
    <button
      on:click={loadActivity}
      class="px-3 py-1.5 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-300 rounded-lg text-xs font-mono transition"
    >
      Refresh
    </button>
  </div>

  {#if errorMsg}
    <div class="p-3 bg-rose-950/40 border border-rose-800/60 rounded-xl text-xs text-rose-300 font-mono">
      {errorMsg}
    </div>
  {/if}

  {#if loading}
    <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat log aktivitas...</div>
  {:else if logs.length === 0}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono bg-garuda-surface border border-garuda-border/60 rounded-xl">
      Belum ada aktivitas tercatat untuk server ini.
    </div>
  {:else}
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl overflow-hidden shadow-lg">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-garuda-border/60 bg-garuda-card/40 text-garuda-muted font-mono uppercase text-[10px]">
              <th class="py-3 px-4 w-44">Waktu</th>
              <th class="py-3 px-4 w-44">Aksi</th>
              <th class="py-3 px-4">Detail</th>
              <th class="py-3 px-4 w-28 text-right">Actor ID</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-garuda-border/40 font-mono">
            {#each logs as log}
              <tr class="hover:bg-garuda-card/40 transition">
                <td class="py-3 px-4 text-garuda-muted text-[11px] whitespace-nowrap">
                  {formatTime(log.created_at)}
                </td>
                <td class="py-3 px-4 font-semibold text-sky-400 whitespace-nowrap">
                  {log.action}
                </td>
                <td class="py-3 px-4 text-slate-300 break-all">
                  {log.detail || '-'}
                </td>
                <td class="py-3 px-4 text-right text-garuda-muted">
                  #{log.actor_id}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}
</div>
