<script>
  import { onMount } from 'svelte';
  import { apiFetch } from '../../stores.js';

  let users = [];
  let loading = true;
  let errorMsg = '';
  let showCreateModal = false;

  // Form State
  let username = '';
  let email = '';
  let password = '';
  let isAdmin = false;
  let submitting = false;

  async function loadUsers() {
    loading = true;
    errorMsg = '';
    try {
      const res = await apiFetch('/api/admin/users');
      if (res.ok) {
        users = await res.json();
      } else {
        errorMsg = 'Gagal memuat pengguna';
      }
    } catch (e) {
      errorMsg = e.message || 'Error koneksi';
    } finally {
      loading = false;
    }
  }

  async function createUser(e) {
    e.preventDefault();
    submitting = true;
    try {
      const res = await apiFetch('/api/admin/users', {
        method: 'POST',
        body: { username, email, password, is_admin: isAdmin },
      });
      if (res.ok) {
        showCreateModal = false;
        username = '';
        email = '';
        password = '';
        isAdmin = false;
        loadUsers();
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

  async function deleteUser(u) {
    if (!confirm(`Hapus user "${u.username}"?`)) return;
    try {
      const res = await apiFetch(`/api/admin/users/${u.id}`, { method: 'DELETE' });
      if (res.ok) {
        loadUsers();
      } else {
        const err = await res.json();
        alert('Gagal menghapus user: ' + (err.error || 'Error'));
      }
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  onMount(() => {
    loadUsers();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-garuda-border/50">
    <div>
      <h1 class="text-xl font-bold text-slate-100 flex items-center gap-2">
        <span>Users Management</span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-garuda-card border border-garuda-border font-mono text-garuda-muted">
          {users.length}
        </span>
      </h1>
      <p class="text-xs text-garuda-muted mt-0.5">Kelola akun pengguna dan hak akses administrator.</p>
    </div>

    <button
      on:click={() => (showCreateModal = true)}
      class="px-4 py-2 bg-garuda-primary hover:bg-sky-400 text-white rounded-lg text-xs font-bold uppercase tracking-wider transition shadow-lg shadow-sky-600/20"
    >
      + Tambah User Baru
    </button>
  </div>

  {#if errorMsg}
    <div class="p-4 rounded-lg bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs font-mono">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-20 text-center text-xs text-garuda-muted font-mono">Memuat pengguna...</div>
  {:else if users.length === 0}
    <div class="py-16 text-center text-xs text-garuda-muted font-mono bg-garuda-surface border border-garuda-border/60 rounded-xl">
      Belum ada user terdaftar.
    </div>
  {:else}
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl overflow-hidden shadow-xl">
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-xs">
          <thead>
            <tr class="border-b border-garuda-border/60 bg-garuda-card/40 text-garuda-muted font-mono uppercase text-[10px]">
              <th class="py-3 px-4 w-16">ID</th>
              <th class="py-3 px-4">Username</th>
              <th class="py-3 px-4">Email</th>
              <th class="py-3 px-4">Role</th>
              <th class="py-3 px-4 text-right">Aksi</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-garuda-border/40 font-mono">
            {#each users as u}
              <tr class="hover:bg-garuda-card/40 transition">
                <td class="py-3 px-4 text-garuda-muted">#{u.id}</td>
                <td class="py-3 px-4 font-bold text-slate-200">{u.username}</td>
                <td class="py-3 px-4 text-slate-300">{u.email}</td>
                <td class="py-3 px-4">
                  {#if u.is_admin}
                    <span class="text-[9px] px-2 py-0.5 rounded bg-sky-950 text-sky-400 border border-sky-800 font-bold">
                      ADMIN
                    </span>
                  {:else}
                    <span class="text-[9px] px-2 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">
                      USER
                    </span>
                  {/if}
                </td>
                <td class="py-3 px-4 text-right">
                  <button
                    on:click={() => deleteUser(u)}
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

  <!-- Modal Create User -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border rounded-xl max-w-md w-full p-6 shadow-2xl relative font-mono text-xs">
        <h3 class="text-base font-bold text-slate-100 mb-1">Tambah User Baru</h3>
        <p class="text-garuda-muted mb-5">Kredensial login untuk akses platform.</p>

        <form on:submit={createUser} class="space-y-4">
          <div>
            <label for="u-name" class="block text-slate-300 mb-1">Username</label>
            <input
              id="u-name"
              type="text"
              bind:value={username}
              required
              placeholder="player1"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
          </div>

          <div>
            <label for="u-email" class="block text-slate-300 mb-1">Email Address</label>
            <input
              id="u-email"
              type="email"
              bind:value={email}
              required
              placeholder="player1@garuda.io"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
          </div>

          <div>
            <label for="u-pwd" class="block text-slate-300 mb-1">Password</label>
            <input
              id="u-pwd"
              type="password"
              bind:value={password}
              required
              placeholder="••••••••••••"
              class="w-full px-3 py-2 bg-garuda-bg border border-garuda-border rounded-lg text-slate-100 focus:outline-none focus:border-garuda-primary"
            />
          </div>

          <div class="pt-1">
            <label class="flex items-center gap-2 cursor-pointer text-slate-300">
              <input type="checkbox" bind:checked={isAdmin} />
              <span>Jadikan Administrator</span>
            </label>
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
              {submitting ? 'Menyimpan...' : 'Buat User'}
            </button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>
