# GarudaPanel

A game server management panel in a single static Go binary. Takes cues from
Pterodactyl's information architecture, written from scratch.

## Why

Pterodactyl works, but running it means PHP, MySQL, Redis, and a separate
daemon. GarudaPanel ships as one ~13MB binary with no external services:
SQLite for storage, the web UI embedded, and a daemon mode for nodes.

## Quick start

Build it:

```bash
CGO_ENABLED=0 go build -o garuda ./cmd/garuda
```

Run the panel:

```bash
./garuda panel --db garuda-panel.db
# prints a bootstrap admin password on first run — save it
```

Create a node in the admin UI (or via API) to get a one-time join token,
then on the node machine:

```bash
./garuda daemon --panel http://panel-host:8080 --join <token>
```

For a service install: `sudo ./install.sh`.

## Features

- Auth with revocable sessions (argon2id); admin and owner roles
- Live server console over WebSocket (xterm.js), with log replay on reconnect
- File manager: browse, edit, streaming upload/download
- Power actions: start, stop, restart, kill
- Eggs: Docker image, startup command, install script, and per-server variables
- Resource stats streamed every 2 seconds
- Nodes connect outbound to the panel, so machines behind NAT need no
  firewall changes

## Architecture

One binary, two modes. The panel stores configuration and serves the UI.
The daemon owns container lifecycle and streams console output, stats, and
files back over a single reverse WebSocket. If the link drops, the daemon
keeps running and the panel replays missed logs when it reconnects.

## Status

MVP. Covered by unit and live API tests: auth, RBAC, console, file manager,
power actions. Container lifecycle against a real Docker host is not yet
tested here. Backups, schedules, and granular subusers are not implemented.

## Layout

```
cmd/garuda          entrypoint (panel | daemon)
internal/panel      HTTP API
internal/daemon     node agent: Docker, console, files
internal/daemonhub  panel-side daemon connections
internal/filecore   sandboxed filesystem (traversal guard, quota)
internal/db         SQLite + embedded migrations
web/                Svelte frontend (embedded via go:embed)
```
