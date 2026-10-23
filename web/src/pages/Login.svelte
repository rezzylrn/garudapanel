<script>
  import { createEventDispatcher } from 'svelte';
  import { token, currentUser, navigate, apiFetch } from '../stores.js';

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
  <div class="w-full max-w-md">
    <!-- Brand Box -->
    <div class="text-center mb-8">
      <div class="inline-flex items-center justify-center w-14 h-14 rounded-xl bg-gradient-to-tr from-sky-600 to-garuda-primary shadow-xl shadow-sky-500/20 mb-3 border border-sky-400/30">
        <span class="font-black text-white text-2xl tracking-tighter">G</span>
      </div>
      <h1 class="text-2xl font-black tracking-wider bg-gradient-to-r from-white via-slate-200 to-sky-400 bg-clip-text text-transparent">
        GARUDA<span class="text-garuda-primary font-normal text-sm ml-1.5 px-2 py-0.5 rounded bg-sky-950/80 border border-sky-800/60">PANEL</span>
      </h1>
      <p class="text-xs text-garuda-muted mt-1 uppercase tracking-widest font-mono">Next-Gen Game Server Control</p>
    </div>

    <!-- Login Card -->
    <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-6 sm:p-8 shadow-2xl relative overflow-hidden">
      <div class="absolute top-0 left-0 right-0 h-[2px] bg-gradient-to-r from-transparent via-garuda-primary to-transparent opacity-80"></div>

      <h2 class="text-lg font-bold text-slate-100 mb-1">Masuk ke Panel</h2>
      <p class="text-xs text-garuda-muted mb-6">Gunakan akun administrator atau kredensial user Anda.</p>

      {#if errorMsg}
        <div class="mb-5 p-3 rounded-lg bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs flex items-center gap-2">
          <div class="w-2 h-2 rounded-full bg-rose-500"></div>
          <span>{errorMsg}</span>
        </div>
      {/if}

      <form on:submit={handleLogin} class="space-y-4">
        <div>
          <label for="ident" class="block text-xs font-semibold text-slate-300 mb-1 uppercase tracking-wider">
            Username / Email
          </label>
          <input
            id="ident"
            type="text"
            bind:value={identifier}
            disabled={loading}
            placeholder="admin atau user@garuda.io"
            class="w-full px-3.5 py-2.5 bg-garuda-bg border border-garuda-border rounded-lg text-sm text-slate-100 placeholder-slate-600 focus:outline-none focus:border-garuda-primary focus:ring-1 focus:ring-garuda-primary transition"
          />
        </div>

        <div>
          <label for="pwd" class="block text-xs font-semibold text-slate-300 mb-1 uppercase tracking-wider">
            Password
          </label>
          <input
            id="pwd"
            type="password"
            bind:value={password}
            disabled={loading}
            placeholder="••••••••••••"
            class="w-full px-3.5 py-2.5 bg-garuda-bg border border-garuda-border rounded-lg text-sm text-slate-100 placeholder-slate-600 focus:outline-none focus:border-garuda-primary focus:ring-1 focus:ring-garuda-primary transition font-mono"
          />
        </div>

        <button
          type="submit"
          disabled={loading}
          class="w-full mt-2 py-3 px-4 rounded-lg bg-gradient-to-r from-sky-600 to-garuda-primary hover:from-sky-500 hover:to-sky-400 text-white font-bold text-sm tracking-wide shadow-lg shadow-sky-600/20 active:scale-[0.99] transition disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
        >
          {#if loading}
            <div class="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
            <span>Memverifikasi...</span>
          {:else}
            <span>Masuk Sekarang</span>
          {/if}
        </button>
      </form>

      <div class="mt-6 pt-4 border-t border-garuda-border/40 text-center">
        <span class="text-[11px] text-garuda-muted font-mono">
          GarudaPanel Single-Binary Architecture &copy; 2026
        </span>
      </div>
    </div>
  </div>
</div>
