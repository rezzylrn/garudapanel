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

<div class="min-h-screen bg-garuda-bg text-garuda-text flex flex-col">
  {#if loadingUser}
    <div class="flex-1 flex items-center justify-center">
      <div class="flex flex-col items-center gap-3">
        <div class="w-10 h-10 border-2 border-garuda-primary border-t-transparent rounded-full animate-spin"></div>
        <div class="text-xs uppercase tracking-widest text-garuda-muted font-mono">Loading GarudaPanel...</div>
      </div>
    </div>
  {:else if isLogin}
    <Login on:loginSuccess={loadMe} />
  {:else}
    <Navbar />
    <main class="flex-1 max-w-7xl w-full mx-auto p-4 md:p-6">
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
        <div class="text-center py-20">
          <div class="text-4xl font-mono text-garuda-primary mb-2">404</div>
          <div class="text-garuda-muted mb-6">Halaman tidak ditemukan: {route}</div>
          <button on:click={() => navigate('/')} class="px-4 py-2 bg-garuda-card border border-garuda-border hover:border-garuda-primary rounded text-sm font-medium transition">
            Kembali ke Dashboard
          </button>
        </div>
      {/if}
    </main>
  {/if}
</div>
