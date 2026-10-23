<script>
  import { currentUser, currentRoute, navigate, apiFetch, token } from '../stores.js';

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

  $: route = $currentRoute || '/';
  $: isAdmin = !!$currentUser?.is_admin;
</script>

<header class="border-b border-garuda-border bg-garuda-surface sticky top-0 z-40">
  <div class="max-w-7xl mx-auto px-4 md:px-5 h-12 flex items-center justify-between">
    <!-- Brand & Navigation -->
    <div class="flex items-center gap-6">
      <a href="#/servers" class="flex items-center gap-2">
        <span class="font-mono font-bold text-xs tracking-wider text-slate-100 uppercase">
          Garuda<span class="text-garuda-muted">Panel</span>
        </span>
      </a>

      <!-- Primary Nav -->
      <nav class="hidden md:flex items-center gap-1 text-xs font-mono">
        <a
          href="#/servers"
          class={`px-2.5 py-1 transition ${
            route === '/' || route.startsWith('/servers')
              ? 'text-slate-100 bg-garuda-card border border-garuda-border'
              : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
          }`}
        >
          Servers
        </a>

        {#if isAdmin}
          <span class="text-garuda-border mx-1">|</span>
          <a
            href="#/admin/nodes"
            class={`px-2.5 py-1 transition ${
              route.startsWith('/admin/nodes')
                ? 'text-slate-100 bg-garuda-card border border-garuda-border'
                : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
            }`}
          >
            Nodes
          </a>
          <a
            href="#/admin/servers"
            class={`px-2.5 py-1 transition ${
              route.startsWith('/admin/servers')
                ? 'text-slate-100 bg-garuda-card border border-garuda-border'
                : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
            }`}
          >
            Admin Servers
          </a>
          <a
            href="#/admin/eggs"
            class={`px-2.5 py-1 transition ${
              route.startsWith('/admin/eggs')
                ? 'text-slate-100 bg-garuda-card border border-garuda-border'
                : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
            }`}
          >
            Eggs
          </a>
          <a
            href="#/admin/users"
            class={`px-2.5 py-1 transition ${
              route.startsWith('/admin/users')
                ? 'text-slate-100 bg-garuda-card border border-garuda-border'
                : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
            }`}
          >
            Users
          </a>
          <a
            href="#/admin/locations"
            class={`px-2.5 py-1 transition ${
              route.startsWith('/admin/locations')
                ? 'text-slate-100 bg-garuda-card border border-garuda-border'
                : 'text-garuda-muted hover:text-slate-200 hover:bg-garuda-hover'
            }`}
          >
            Locations
          </a>
        {/if}
      </nav>
    </div>

    <!-- Right Profile & Logout -->
    <div class="flex items-center gap-3">
      {#if $currentUser}
        <div class="flex items-center gap-2 text-xs font-mono">
          <span class="text-slate-300">{$currentUser.username}</span>
          <span class="text-[10px] uppercase text-garuda-muted px-1.5 py-0.5 border border-garuda-border bg-garuda-card">
            {$currentUser.is_admin ? 'ADMIN' : 'USER'}
          </span>
        </div>
        <button
          on:click={handleLogout}
          disabled={loggingOut}
          class="px-2 py-1 text-xs font-mono text-garuda-muted hover:text-garuda-danger border border-garuda-border hover:border-garuda-border bg-garuda-card hover:bg-garuda-hover transition disabled:opacity-50"
          title="Keluar dari sesi"
        >
          Logout
        </button>
      {/if}
    </div>
  </div>

  <!-- Mobile Subnav for Admin -->
  {#if isAdmin}
    <div class="md:hidden flex items-center gap-1 overflow-x-auto px-4 py-1.5 border-t border-garuda-border bg-garuda-card text-xs font-mono">
      <a href="#/servers" class="px-2 py-0.5 text-slate-300 hover:text-white whitespace-nowrap">Servers</a>
      <a href="#/admin/nodes" class="px-2 py-0.5 text-slate-300 hover:text-white whitespace-nowrap">Nodes</a>
      <a href="#/admin/servers" class="px-2 py-0.5 text-slate-300 hover:text-white whitespace-nowrap">Admin Servers</a>
      <a href="#/admin/eggs" class="px-2 py-0.5 text-slate-300 hover:text-white whitespace-nowrap">Eggs</a>
      <a href="#/admin/users" class="px-2 py-0.5 text-slate-300 hover:text-white whitespace-nowrap">Users</a>
      <a href="#/admin/locations" class="px-2 py-0.5 text-slate-300 hover:text-white whitespace-nowrap">Locations</a>
    </div>
  {/if}
</header>
