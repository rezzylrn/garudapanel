<script>
  import { onMount, onDestroy } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import { apiFetch } from '../../stores.js';

  export let server;

  let terminalContainer;
  let term;
  let fitAddon;
  let ws;
  let wsConnected = false;
  let connStatus = 'Connecting...';
  let commandInput = '';

  let stats = {
    cpu_percent: 0,
    mem_usage_mb: 0,
    mem_limit_mb: 0,
    net_rx_kb: 0,
    net_tx_kb: 0,
  };

  let powerLoading = '';

  async function sendPower(action) {
    powerLoading = action;
    try {
      const res = await apiFetch(`/api/servers/${server.id}/power`, {
        method: 'POST',
        body: { action },
      });
      if (res.ok) {
        if (action === 'start') server.status = 'starting';
        if (action === 'stop' || action === 'kill') server.status = 'stopping';
        if (action === 'restart') server.status = 'starting';
      }
    } catch (e) {
      console.error('power action error:', e);
    } finally {
      powerLoading = '';
    }
  }

  async function initTerminalAndWS() {
    term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
      theme: {
        background: '#090b0e',
        foreground: '#e2e8f0',
        cursor: '#94a3b8',
        selectionBackground: 'rgba(148, 163, 184, 0.25)',
        black: '#0f1115',
        red: '#dc2626',
        green: '#16a34a',
        yellow: '#d97706',
        blue: '#2563eb',
        magenta: '#9333ea',
        cyan: '#0891b2',
        white: '#f1f5f9',
      },
    });

    fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(terminalContainer);
    fitAddon.fit();

    term.onData((data) => {
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ t: 'in', d: data }));
      }
    });

    connectWS();
  }

  let reconnectTimeout;

  async function connectWS() {
    if (!server || !server.id) return;
    connStatus = 'Token...';

    try {
      const res = await apiFetch(`/api/servers/${server.id}/console-token`, {
        method: 'POST',
      });
      if (!res.ok) {
        connStatus = 'Auth failed';
        scheduleReconnect();
        return;
      }
      const data = await res.json();
      const token = data.token;

      const loc = window.location;
      const wsProtocol = loc.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${wsProtocol}//${loc.host}/api/servers/${server.id}/ws`;

      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        connStatus = 'Authenticating';
        ws.send(JSON.stringify({ t: 'auth', token }));
      };

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          if (msg.t === 'out') {
            term.write(msg.d);
          } else if (msg.t === 'stat') {
            if (msg.d) {
              stats = { ...stats, ...msg.d };
            }
          } else if (msg.t === 'status') {
            connStatus = msg.d?.s || msg.s || 'Connected';
            if (msg.s) server.status = msg.s;
          }
        } catch (e) {
          term.write(event.data);
        }
      };

      ws.onclose = () => {
        wsConnected = false;
        connStatus = 'Terputus';
        scheduleReconnect();
      };

      ws.onerror = (e) => {
        console.error('ws error', e);
      };

      wsConnected = true;
      connStatus = 'Connected';

      if (fitAddon) {
        const dims = fitAddon.proposeDimensions();
        if (dims && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ t: 'resize', cols: dims.cols, rows: dims.rows }));
        }
      }
    } catch (e) {
      connStatus = 'Error';
      scheduleReconnect();
    }
  }

  function scheduleReconnect() {
    clearTimeout(reconnectTimeout);
    reconnectTimeout = setTimeout(() => {
      connectWS();
    }, 3000);
  }

  function submitCommand(e) {
    e.preventDefault();
    if (!commandInput.trim() || !ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify({ t: 'in', d: commandInput + '\n' }));
    commandInput = '';
  }

  function handleResize() {
    if (fitAddon) {
      fitAddon.fit();
      const dims = fitAddon.proposeDimensions();
      if (dims && ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ t: 'resize', cols: dims.cols, rows: dims.rows }));
      }
    }
  }

  onMount(() => {
    initTerminalAndWS();
    window.addEventListener('resize', handleResize);
  });

  onDestroy(() => {
    clearTimeout(reconnectTimeout);
    window.removeEventListener('resize', handleResize);
    if (ws) ws.close();
    if (term) term.dispose();
  });
</script>

<div class="space-y-3 font-mono">
  <!-- Power & Live Metrics Row -->
  <div class="border border-garuda-border bg-garuda-surface p-3 flex flex-col md:flex-row md:items-center justify-between gap-3">
    <!-- Controls -->
    <div class="flex items-center gap-1.5 text-xs">
      <button
        on:click={() => sendPower('start')}
        disabled={!!powerLoading}
        class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-emerald-400 font-semibold uppercase tracking-wider transition disabled:opacity-50"
      >
        Start
      </button>

      <button
        on:click={() => sendPower('restart')}
        disabled={!!powerLoading}
        class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-amber-400 font-semibold uppercase tracking-wider transition disabled:opacity-50"
      >
        Restart
      </button>

      <button
        on:click={() => sendPower('stop')}
        disabled={!!powerLoading}
        class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-rose-400 font-semibold uppercase tracking-wider transition disabled:opacity-50"
      >
        Stop
      </button>

      <button
        on:click={() => sendPower('kill')}
        disabled={!!powerLoading}
        class="px-2.5 py-1 bg-garuda-card hover:bg-red-950/40 border border-garuda-border text-red-500 uppercase tracking-wider transition disabled:opacity-50"
        title="Force kill container (SIGKILL)"
      >
        Kill
      </button>

      {#if powerLoading}
        <span class="text-xs text-garuda-muted animate-pulse ml-2">
          Mengirim {powerLoading}...
        </span>
      {/if}
    </div>

    <!-- Metrics -->
    <div class="flex items-center gap-4 text-xs">
      <div>
        <span class="text-garuda-muted">CPU:</span>
        <span class="text-slate-100 font-semibold ml-1">{stats.cpu_percent ? stats.cpu_percent.toFixed(1) : '0.0'}%</span>
      </div>

      <div>
        <span class="text-garuda-muted">MEM:</span>
        <span class="text-slate-100 font-semibold ml-1">
          {stats.mem_usage_mb ? stats.mem_usage_mb.toFixed(1) : '0.0'} / {server.memory_mb} MB
        </span>
      </div>

      <div>
        <span class="text-garuda-muted">NET:</span>
        <span class="text-slate-100 font-semibold ml-1">
          ↓{(stats.net_rx_kb || 0).toFixed(0)}K ↑{(stats.net_tx_kb || 0).toFixed(0)}K
        </span>
      </div>

      <div class="border-l border-garuda-border pl-3">
        <span class="text-garuda-muted">Status:</span>
        <span class="text-slate-300 ml-1">{connStatus}</span>
      </div>
    </div>
  </div>

  <!-- Terminal Console -->
  <div class="border border-garuda-border bg-[#090b0e] flex flex-col h-[520px]">
    <div class="px-3 py-1.5 border-b border-garuda-border bg-garuda-surface flex items-center justify-between text-xs text-garuda-muted">
      <span>Console Output</span>
      <span>TTY / Stream</span>
    </div>

    <!-- Mount xterm -->
    <div class="flex-1 p-2 overflow-hidden" bind:this={terminalContainer}></div>

    <!-- Command Input -->
    <form on:submit={submitCommand} class="p-2 border-t border-garuda-border bg-garuda-surface flex items-center gap-2">
      <span class="text-garuda-muted text-xs pl-1 font-bold">&gt;</span>
      <input
        type="text"
        bind:value={commandInput}
        placeholder="Perintah server..."
        class="flex-1 bg-garuda-bg border border-garuda-border px-2.5 py-1 text-xs text-slate-100 placeholder-garuda-border focus:outline-none focus:border-garuda-accent"
      />
      <button
        type="submit"
        class="px-3 py-1 bg-garuda-card hover:bg-garuda-hover border border-garuda-border text-xs text-slate-200"
      >
        Kirim
      </button>
    </form>
  </div>
</div>
