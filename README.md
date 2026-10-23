# GarudaPanel

A game server management panel in a single static Go binary. Takes cues from
Pterodactyl's information architecture, written from scratch.

## Why

Pterodactyl works, but running it means PHP, MySQL, Redis, and a separate
daemon. GarudaPanel ships as one ~13MB binary with no external services:
SQLite for storage, the web UI embedded, and a daemon mode for nodes.

## Quick start

### One-line installer

On Ubuntu or Debian, run:

```bash
curl -fsSL https://github.com/rezzylrn/garudapanel/releases/latest/download/install.sh -o /tmp/gp-install.sh && sudo bash /tmp/gp-install.sh
```

The interactive script detects dependencies, downloads the static binary,
configures systemd services, and walks you through creating the administrator
account (username, email, password).

Example session (Panel only):
```text
==================================================
         GarudaPanel Interactive Installer        
==================================================

Select component to install:
  1) Panel only (Web UI + API Server)
  2) Daemon only (Node Worker Service)
  3) All-in-one (Panel + Daemon on this server)
  4) Uninstall GarudaPanel

Enter choice [1-4]: 1
[INFO] Creating system user 'garuda'...
[INFO] Using local binary './garuda' -> /usr/local/bin/garuda

[INFO] Configuring GarudaPanel (Web UI + API Server)...
Listen address [default: :8080]: 
Data directory [default: /var/lib/garudapanel]: 

[INFO] Administrator account setup
  Create the administrator account for the GarudaPanel web interface.

  Username [default: admin]: 
  Email address: admin@example.com
  Password (min 8 characters): 
  Confirm password: 
[OK] Administrator account 'admin' created successfully.
[INFO] Writing /etc/systemd/system/garuda-panel.service...
[OK] garuda-panel.service enabled and started.

==================================================
              Installation Complete               
==================================================

Panel Service:
  Status:   active (garuda-panel.service)
  Listen:   :8080
  Admin:    admin (use the password you set during installation)
  Logs:     journalctl -u garuda-panel -f

Uninstall command:
  curl -fsSL https://github.com/rezzylrn/garudapanel/releases/latest/download/install.sh -o /tmp/gp-install.sh && sudo bash /tmp/gp-install.sh --uninstall
==================================================
```

### Manual build

Build from source:

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
