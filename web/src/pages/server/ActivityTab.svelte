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

<div class="space-y-3 font-mono text-xs">
  <div class="border border-garuda-border bg-garuda-surface p-3 flex flex-col md:flex-row md:items-center justify-between gap-2">
    <div>
      <h3 class="font-bold text-slate-100 uppercase tracking-wider text-xs">Audit & Activity Log</h3>
      <p class="text-garuda-muted text-[11px] mt-0.5">
        Catatan tindakan operasional pada server (status power, konfigurasi, file).
      </p>
    </div>
    <button
      on:click={loadActivity}
      class="self-start md:self-auto px-2.5 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-garuda-muted hover:text-slate-200"
    >
      Refresh
    </button>
  </div>

  {#if errorMsg}
    <div class="p-2 border-l-2 border-garuda-danger bg-garuda-surface text-rose-400">
      {errorMsg}
    </div>
  {/if}

  {#if loading}
    <div class="py-16 text-center text-garuda-muted">Memuat catatan aktivitas...</div>
  {:else if logs.length === 0}
    <div class="py-12 text-center text-garuda-muted border border-garuda-border bg-garuda-surface">
      Belum ada aktivitas tercatat untuk server ini.
    </div>
  {:else}
    <div class="border border-garuda-border bg-garuda-surface overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
            <th class="py-2 px-3 w-44">Waktu</th>
            <th class="py-2 px-3 w-40">Aksi</th>
            <th class="py-2 px-3">Detail</th>
            <th class="py-2 px-3 w-28 text-right">Actor</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-garuda-border">
          {#each logs as log}
            <tr class="hover:bg-garuda-hover">
              <td class="py-2 px-3 text-garuda-muted text-[11px] whitespace-nowrap">
                {formatTime(log.created_at)}
              </td>
              <td class="py-2 px-3 font-semibold text-slate-200 whitespace-nowrap uppercase">
                {log.action}
              </td>
              <td class="py-2 px-3 text-slate-300 break-all">
                {log.detail || '-'}
              </td>
              <td class="py-2 px-3 text-right text-garuda-muted">
                #{log.actor_id}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
