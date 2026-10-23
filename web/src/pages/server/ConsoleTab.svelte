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
      fontFamily: 'JetBrains Mono, Menlo, monospace',
      theme: {
        background: '#070a0f',
        foreground: '#e2e8f0',
        cursor: '#38bdf8',
        selectionBackground: 'rgba(56, 189, 248, 0.3)',
        black: '#0f172a',
        red: '#f43f5e',
        green: '#10b981',
        yellow: '#f59e0b',
        blue: '#0ea5e9',
        magenta: '#d946ef',
        cyan: '#06b6d4',
        white: '#f8fafc',
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
    connStatus = 'Requesting console token...';

    try {
      const res = await apiFetch(`/api/servers/${server.id}/console-token`, {
        method: 'POST',
      });
      if (!res.ok) {
        connStatus = 'Failed to get console token';
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
        connStatus = 'Authenticating...';
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
        connStatus = 'Disconnected. Reconnecting in 3s...';
        scheduleReconnect();
      };

      ws.onerror = (e) => {
        console.error('ws error', e);
      };

      wsConnected = true;
      connStatus = 'Connected';

      // send initial resize
      if (fitAddon) {
        const dims = fitAddon.proposeDimensions();
        if (dims && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ t: 'resize', cols: dims.cols, rows: dims.rows }));
        }
      }
    } catch (e) {
      connStatus = 'Connection error';
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

<div class="space-y-4">
  <!-- Power Actions & Live Stat Bar -->
  <div class="bg-garuda-surface border border-garuda-border/80 rounded-xl p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-lg">
    <!-- Power Controls -->
    <div class="flex items-center gap-2">
      <button
        on:click={() => sendPower('start')}
        disabled={!!powerLoading}
        class="px-4 py-2 bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-400 border border-emerald-500/40 rounded-lg text-xs font-bold uppercase tracking-wider transition disabled:opacity-50 flex items-center gap-1.5"
      >
        <div class="w-2 h-2 rounded-full bg-emerald-400"></div>
        <span>Start</span>
      </button>

      <button
        on:click={() => sendPower('restart')}
        disabled={!!powerLoading}
        class="px-4 py-2 bg-amber-600/20 hover:bg-amber-600/30 text-amber-400 border border-amber-500/40 rounded-lg text-xs font-bold uppercase tracking-wider transition disabled:opacity-50 flex items-center gap-1.5"
      >
        <div class="w-2 h-2 rounded-full bg-amber-400"></div>
        <span>Restart</span>
      </button>

      <button
        on:click={() => sendPower('stop')}
        disabled={!!powerLoading}
        class="px-4 py-2 bg-rose-600/20 hover:bg-rose-600/30 text-rose-400 border border-rose-500/40 rounded-lg text-xs font-bold uppercase tracking-wider transition disabled:opacity-50 flex items-center gap-1.5"
      >
        <div class="w-2 h-2 rounded-full bg-rose-400"></div>
        <span>Stop</span>
      </button>

      <button
        on:click={() => sendPower('kill')}
        disabled={!!powerLoading}
        class="px-3 py-2 bg-slate-800 hover:bg-rose-950/60 text-slate-400 hover:text-rose-400 border border-slate-700 hover:border-rose-800 rounded-lg text-xs font-mono uppercase transition disabled:opacity-50"
        title="Force kill container"
      >
        Kill
      </button>

      {#if powerLoading}
        <span class="text-xs text-garuda-muted animate-pulse ml-2 font-mono">
          Mengirim perintah {powerLoading}...
        </span>
      {/if}
    </div>

    <!-- Live Stat Badges -->
    <div class="flex items-center gap-4 text-xs font-mono">
      <div class="bg-garuda-bg/80 px-3 py-1.5 rounded-lg border border-garuda-border flex flex-col">
        <span class="text-[10px] text-garuda-muted uppercase">CPU Usage</span>
        <span class="font-bold text-slate-200">{stats.cpu_percent ? stats.cpu_percent.toFixed(1) : '0.0'}%</span>
      </div>

      <div class="bg-garuda-bg/80 px-3 py-1.5 rounded-lg border border-garuda-border flex flex-col">
        <span class="text-[10px] text-garuda-muted uppercase">Memory</span>
        <span class="font-bold text-slate-200">
          {stats.mem_usage_mb ? stats.mem_usage_mb.toFixed(1) : '0.0'} / {server.memory_mb} MB
        </span>
      </div>

      <div class="bg-garuda-bg/80 px-3 py-1.5 rounded-lg border border-garuda-border flex flex-col">
        <span class="text-[10px] text-garuda-muted uppercase">Network I/O</span>
        <span class="font-bold text-slate-200">
          ↓{(stats.net_rx_kb || 0).toFixed(0)} KB / ↑{(stats.net_tx_kb || 0).toFixed(0)} KB
        </span>
      </div>
    </div>
  </div>

  <!-- Terminal Box -->
  <div class="bg-[#070a0f] border border-garuda-border rounded-xl overflow-hidden shadow-2xl flex flex-col h-[520px]">
    <!-- Terminal Header Bar -->
    <div class="px-4 py-2.5 bg-garuda-surface/60 border-b border-garuda-border/60 flex items-center justify-between text-xs">
      <div class="flex items-center gap-2 font-mono">
        <span class="w-2.5 h-2.5 rounded-full bg-rose-500/80"></span>
        <span class="w-2.5 h-2.5 rounded-full bg-amber-500/80"></span>
        <span class="w-2.5 h-2.5 rounded-full bg-emerald-500/80"></span>
        <span class="ml-2 text-slate-400 font-semibold text-[11px] uppercase tracking-wider">Container Console Session</span>
      </div>
      <div class="flex items-center gap-2 font-mono text-[11px]">
        <span class="text-garuda-muted">Status:</span>
        <span class="text-sky-400 font-medium">{connStatus}</span>
      </div>
    </div>

    <!-- xterm.js Canvas mount -->
    <div class="flex-1 p-3 overflow-hidden" bind:this={terminalContainer}></div>

    <!-- Command Input Bar -->
    <form on:submit={submitCommand} class="p-2.5 bg-garuda-surface/80 border-t border-garuda-border/60 flex items-center gap-2">
      <div class="text-garuda-primary font-mono text-xs pl-2 font-bold">&gt;</div>
      <input
        type="text"
        bind:value={commandInput}
        placeholder="Ketik perintah ke server (contoh: say hello, help)..."
        class="flex-1 bg-garuda-bg/80 border border-garuda-border/70 rounded-lg px-3 py-2 text-xs font-mono text-slate-100 placeholder-slate-600 focus:outline-none focus:border-garuda-primary transition"
      />
      <button
        type="submit"
        class="px-4 py-2 bg-garuda-card hover:bg-garuda-hover border border-garuda-border rounded-lg text-xs font-mono text-slate-200 transition"
      >
        Kirim
      </button>
    </form>
  </div>
</div>
