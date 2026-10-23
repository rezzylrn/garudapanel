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

<div class="space-y-4 font-mono text-xs">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-garuda-border">
    <div class="flex items-baseline gap-3">
      <h1 class="text-sm font-bold uppercase tracking-wider text-slate-100">
        Users
      </h1>
      <span class="text-garuda-muted text-xs">Total: {users.length}</span>
    </div>

    <button
      on:click={() => (showCreateModal = true)}
      class="self-start sm:self-auto px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-slate-200 uppercase tracking-wider"
    >
      + Tambah User
    </button>
  </div>

  {#if errorMsg}
    <div class="p-2 border-l-2 border-garuda-danger bg-garuda-surface text-rose-400">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="py-16 text-center text-garuda-muted">Memuat pengguna...</div>
  {:else if users.length === 0}
    <div class="py-12 text-center text-garuda-muted border border-garuda-border bg-garuda-surface">
      Belum ada user terdaftar.
    </div>
  {:else}
    <div class="border border-garuda-border bg-garuda-surface overflow-x-auto">
      <table class="w-full text-left border-collapse text-xs">
        <thead>
          <tr class="border-b border-garuda-border bg-garuda-card text-garuda-muted text-[11px] uppercase">
            <th class="py-2 px-3 w-16">ID</th>
            <th class="py-2 px-3">Username</th>
            <th class="py-2 px-3">Email</th>
            <th class="py-2 px-3">Role</th>
            <th class="py-2 px-3 text-right">Aksi</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-garuda-border">
          {#each users as u}
            <tr class="hover:bg-garuda-hover">
              <td class="py-2 px-3 text-garuda-muted">#{u.id}</td>
              <td class="py-2 px-3 font-semibold text-slate-100">{u.username}</td>
              <td class="py-2 px-3 text-slate-300">{u.email}</td>
              <td class="py-2 px-3">
                {#if u.is_admin}
                  <span class="px-1.5 py-0.5 border border-garuda-border bg-garuda-card text-emerald-400 text-[10px] uppercase font-bold">
                    ADMIN
                  </span>
                {:else}
                  <span class="text-garuda-muted text-[10px] uppercase">
                    USER
                  </span>
                {/if}
              </td>
              <td class="py-2 px-3 text-right">
                <button
                  on:click={() => deleteUser(u)}
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

  <!-- Modal Create User -->
  {#if showCreateModal}
    <div class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
      <div class="bg-garuda-surface border border-garuda-border max-w-md w-full p-5">
        <div class="flex items-center justify-between pb-2 mb-3 border-b border-garuda-border">
          <h3 class="font-bold text-slate-100 uppercase tracking-wider">Tambah User Baru</h3>
          <button on:click={() => (showCreateModal = false)} class="text-garuda-muted hover:text-white">&times;</button>
        </div>

        <form on:submit={createUser} class="space-y-3">
          <div>
            <label for="u-name" class="block text-[11px] uppercase text-garuda-muted mb-1">Username</label>
            <input
              id="u-name"
              type="text"
              bind:value={username}
              required
              placeholder="player1"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div>
            <label for="u-email" class="block text-[11px] uppercase text-garuda-muted mb-1">Email</label>
            <input
              id="u-email"
              type="email"
              bind:value={email}
              required
              placeholder="player1@garuda.io"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div>
            <label for="u-pwd" class="block text-[11px] uppercase text-garuda-muted mb-1">Password</label>
            <input
              id="u-pwd"
              type="password"
              bind:value={password}
              required
              placeholder="••••••••••••"
              class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
            />
          </div>

          <div class="pt-1">
            <label class="flex items-center gap-2 cursor-pointer text-slate-300">
              <input type="checkbox" bind:checked={isAdmin} />
              <span>Hak Akses Administrator</span>
            </label>
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
