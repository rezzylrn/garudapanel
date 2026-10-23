<script>
  import { currentUser, navigate, apiFetch, token } from '../stores.js';

  let loggingOut = false;

  async function handleLogout() {
    loggingOut = true;
    try {
      await apiFetch('/api/auth/logout', { method: 'POST' });
    } catch (e) {
      // ignore
    } finally {
      token.set('');
      currentUser.set(null);
      navigate('/login');
      loggingOut = false;
    }
  }
</script>

<header class="border-b border-garuda-border/60 bg-garuda-surface/90 backdrop-blur sticky top-0 z-40">
  <div class="max-w-7xl mx-auto px-4 md:px-6 h-16 flex items-center justify-between">
    <!-- Brand -->
    <div class="flex items-center gap-6">
      <a href="#/" class="flex items-center gap-2.5 group">
        <div class="w-8 h-8 rounded bg-gradient-to-tr from-sky-600 to-garuda-primary flex items-center justify-center font-black text-white text-lg tracking-wider shadow-lg shadow-sky-500/20 group-hover:scale-105 transition-transform">
          G
        </div>
        <div class="flex flex-col">
          <span class="font-extrabold text-base tracking-wider bg-gradient-to-r from-white via-slate-200 to-sky-400 bg-clip-text text-transparent">
            GARUDA<span class="text-garuda-primary font-light text-xs ml-1 px-1.5 py-0.5 rounded bg-sky-950/60 border border-sky-800/50">PANEL</span>
          </span>
        </div>
      </a>

      <!-- Primary Nav -->
      <nav class="hidden md:flex items-center gap-1 text-sm">
        <a href="#/servers" class="px-3 py-1.5 rounded font-medium text-slate-300 hover:text-white hover:bg-garuda-hover transition">
          Servers
        </a>
        {#if $currentUser?.is_admin}
          <div class="h-4 w-[1px] bg-garuda-border mx-2"></div>
          <a href="#/admin/nodes" class="px-3 py-1.5 rounded font-medium text-slate-400 hover:text-white hover:bg-garuda-hover transition">
            Nodes
          </a>
          <a href="#/admin/servers" class="px-3 py-1.5 rounded font-medium text-slate-400 hover:text-white hover:bg-garuda-hover transition">
            Admin Servers
          </a>
          <a href="#/admin/eggs" class="px-3 py-1.5 rounded font-medium text-slate-400 hover:text-white hover:bg-garuda-hover transition">
            Eggs
          </a>
          <a href="#/admin/users" class="px-3 py-1.5 rounded font-medium text-slate-400 hover:text-white hover:bg-garuda-hover transition">
            Users
          </a>
          <a href="#/admin/locations" class="px-3 py-1.5 rounded font-medium text-slate-400 hover:text-white hover:bg-garuda-hover transition">
            Locations
          </a>
        {/if}
      </nav>
    </div>

    <!-- Right Profile & Logout -->
    <div class="flex items-center gap-3">
      {#if $currentUser}
        <div class="flex items-center gap-2 text-xs">
          <div class="flex flex-col text-right">
            <span class="font-medium text-slate-200">{$currentUser.username}</span>
            <span class="text-garuda-muted text-[10px] font-mono">
              {$currentUser.is_admin ? 'ADMINISTRATOR' : 'USER'}
            </span>
          </div>
          <div class="w-8 h-8 rounded-full bg-garuda-card border border-garuda-border flex items-center justify-center font-bold text-garuda-primary">
            {$currentUser.username.slice(0, 2).toUpperCase()}
          </div>
        </div>
        <button
          on:click={handleLogout}
          disabled={loggingOut}
          class="px-2.5 py-1.5 text-xs text-rose-400 hover:text-rose-300 bg-rose-950/20 hover:bg-rose-950/40 border border-rose-900/40 rounded transition"
          title="Logout"
        >
          Logout
        </button>
      {/if}
    </div>
  </div>

  <!-- Mobile Subnav for Admin -->
  {#if $currentUser?.is_admin}
    <div class="md:hidden flex items-center gap-2 overflow-x-auto px-4 py-2 border-t border-garuda-border/40 text-xs">
      <a href="#/servers" class="px-2.5 py-1 rounded bg-garuda-card text-slate-300 whitespace-nowrap">Servers</a>
      <a href="#/admin/nodes" class="px-2.5 py-1 rounded bg-garuda-card text-slate-300 whitespace-nowrap">Nodes</a>
      <a href="#/admin/servers" class="px-2.5 py-1 rounded bg-garuda-card text-slate-300 whitespace-nowrap">Admin Servers</a>
      <a href="#/admin/eggs" class="px-2.5 py-1 rounded bg-garuda-card text-slate-300 whitespace-nowrap">Eggs</a>
      <a href="#/admin/users" class="px-2.5 py-1 rounded bg-garuda-card text-slate-300 whitespace-nowrap">Users</a>
      <a href="#/admin/locations" class="px-2.5 py-1 rounded bg-garuda-card text-slate-300 whitespace-nowrap">Locations</a>
    </div>
  {/if}
</header>
