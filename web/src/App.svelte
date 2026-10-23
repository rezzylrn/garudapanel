<script>
  import { onMount } from 'svelte';
  import { token, currentUser, currentRoute, navigate, apiFetch } from './stores.js';
  import Login from './pages/Login.svelte';
  import ServerList from './pages/ServerList.svelte';
  import ServerDetail from './pages/ServerDetail.svelte';
  import AdminNodes from './pages/admin/Nodes.svelte';
  import AdminServers from './pages/admin/Servers.svelte';
  import AdminEggs from './pages/admin/Eggs.svelte';
  import AdminUsers from './pages/admin/Users.svelte';
  import AdminLocations from './pages/admin/Locations.svelte';
  import Navbar from './components/Navbar.svelte';

  let loadingUser = true;

  async function loadMe() {
    if (!$token) {
      loadingUser = false;
      if ($currentRoute !== '/login') {
        navigate('/login');
      }
      return;
    }
    try {
      const res = await apiFetch('/api/auth/me');
      if (res.ok) {
        const data = await res.json();
        currentUser.set(data);
        if ($currentRoute === '/login') {
          navigate('/');
        }
      } else {
        token.set('');
        currentUser.set(null);
        navigate('/login');
      }
    } catch (e) {
      token.set('');
      currentUser.set(null);
      navigate('/login');
    } finally {
      loadingUser = false;
    }
  }

  onMount(() => {
    loadMe();
  });

  $: route = $currentRoute || '/';
  $: isLogin = route === '/login';

  // Parser rute server detail: /servers/:id or /servers/:id/:tab
  $: serverMatch = route.match(/^\/servers\/(\d+)(?:\/([a-zA-Z0-9_-]+))?/);
  $: serverId = serverMatch ? serverMatch[1] : null;
  $: serverTab = serverMatch ? serverMatch[2] || 'console' : null;
</script>

<div class="min-h-screen bg-garuda-bg text-garuda-text flex flex-col font-sans">
  {#if loadingUser}
    <div class="flex-1 flex items-center justify-center">
      <div class="flex items-center gap-2 text-xs font-mono text-garuda-muted">
        <span class="inline-block w-2 h-2 bg-garuda-muted animate-pulse"></span>
        <span>Memuat panel...</span>
      </div>
    </div>
  {:else if isLogin}
    <Login on:loginSuccess={loadMe} />
  {:else}
    <Navbar />
    <main class="flex-1 max-w-7xl w-full mx-auto p-4 md:p-5">
      {#if route === '/' || route === '/servers'}
        <ServerList />
      {:else if serverMatch}
        <ServerDetail {serverId} {serverTab} />
      {:else if route === '/admin/nodes'}
        <AdminNodes />
      {:else if route === '/admin/servers'}
        <AdminServers />
      {:else if route === '/admin/eggs'}
        <AdminEggs />
      {:else if route === '/admin/users'}
        <AdminUsers />
      {:else if route === '/admin/locations'}
        <AdminLocations />
      {:else}
        <div class="border border-garuda-border bg-garuda-surface p-6 text-center max-w-md mx-auto my-12">
          <div class="text-xs font-mono uppercase text-garuda-muted mb-1">Error 404</div>
          <div class="text-sm font-semibold text-slate-100 mb-2">Halaman tidak ditemukan</div>
          <div class="text-xs font-mono text-garuda-muted mb-4">{route}</div>
          <button
            on:click={() => navigate('/')}
            class="px-3 py-1.5 bg-garuda-card border border-garuda-border text-xs font-mono text-slate-200 hover:bg-garuda-hover"
          >
            Kembali ke Server
          </button>
        </div>
      {/if}
    </main>
  {/if}
</div>
