<script>
  import { createEventDispatcher } from 'svelte';
  import { token, navigate, apiFetch } from '../stores.js';

  const dispatch = createEventDispatcher();

  let identifier = '';
  let password = '';
  let errorMsg = '';
  let loading = false;

  async function handleLogin(e) {
    e.preventDefault();
    errorMsg = '';
    if (!identifier.trim() || !password) {
      errorMsg = 'Username/email dan password wajib diisi';
      return;
    }
    loading = true;
    try {
      const res = await apiFetch('/api/auth/login', {
        method: 'POST',
        body: { identifier: identifier.trim(), password },
      });
      const data = await res.json();
      if (!res.ok) {
        errorMsg = data.error || 'Login gagal';
        return;
      }
      token.set(data.token);
      dispatch('loginSuccess');
      navigate('/');
    } catch (err) {
      errorMsg = err.message || 'Koneksi ke server gagal';
    } finally {
      loading = false;
    }
  }
</script>

<div class="flex-1 flex items-center justify-center p-4">
  <div class="w-full max-w-sm">
    <!-- Header -->
    <div class="mb-4 text-center">
      <div class="font-mono font-bold text-base tracking-wider uppercase text-slate-100">
        Garuda<span class="text-garuda-muted">Panel</span>
      </div>
      <div class="text-[11px] font-mono text-garuda-muted mt-1">Infrastruktur Server Game</div>
    </div>

    <!-- Login Panel -->
    <div class="bg-garuda-surface border border-garuda-border p-5">
      <div class="text-xs font-mono font-semibold uppercase tracking-wider text-slate-300 pb-3 mb-4 border-b border-garuda-border">
        Autentikasi
      </div>

      {#if errorMsg}
        <div class="mb-4 p-2.5 bg-garuda-card border-l-2 border-garuda-danger text-xs font-mono text-red-400">
          {errorMsg}
        </div>
      {/if}

      <form on:submit={handleLogin} class="space-y-3 font-mono text-xs">
        <div>
          <label for="ident" class="block text-[11px] uppercase text-garuda-muted mb-1">
            Username / Email
          </label>
          <input
            id="ident"
            type="text"
            bind:value={identifier}
            disabled={loading}
            placeholder="admin"
            class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
          />
        </div>

        <div>
          <label for="pwd" class="block text-[11px] uppercase text-garuda-muted mb-1">
            Password
          </label>
          <input
            id="pwd"
            type="password"
            bind:value={password}
            disabled={loading}
            placeholder="••••••••••••"
            class="w-full px-2.5 py-1.5 bg-garuda-bg border border-garuda-border text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
          />
        </div>

        <button
          type="submit"
          disabled={loading}
          class="w-full mt-2 py-2 px-3 bg-garuda-card hover:bg-garuda-hover border border-garuda-border hover:border-slate-400 text-slate-200 text-xs font-mono uppercase tracking-wider transition disabled:opacity-50"
        >
          {loading ? 'Memverifikasi...' : 'Masuk'}
        </button>
      </form>

      <div class="mt-4 pt-3 border-t border-garuda-border text-[10px] font-mono text-garuda-muted text-center">
        GarudaPanel Daemon / Panel Core
      </div>
    </div>
  </div>
</div>
