-- GarudaPanel fase 1: core tables (SPEC §3).
PRAGMA journal_mode = WAL; -- no-op on some setups; connection also sets WAL
CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    totp_secret   TEXT,
    created_at    INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE sessions (
    token_hash  TEXT PRIMARY KEY, -- sha256 hex of raw token
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  INTEGER NOT NULL,
    ip          TEXT NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE locations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    short_code  TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE nodes (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    location_id       INTEGER NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
    name              TEXT NOT NULL UNIQUE,
    fqdn              TEXT NOT NULL,
    daemon_token_hash TEXT,           -- sha256 hex of long-lived node token; NULL until joined
    memory_limit_mb   INTEGER NOT NULL DEFAULT 0,
    disk_limit_mb     INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL DEFAULT (unixepoch())
);

-- One-time join tokens for daemon registration (SPEC §8: expiry 15 min).
CREATE TABLE node_join_tokens (
    token_hash TEXT PRIMARY KEY,        -- sha256 hex of raw join token
    node_id    INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    used       INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_join_tokens_node ON node_join_tokens(node_id);

CREATE TABLE allocations (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id   INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    ip        TEXT NOT NULL,
    port      INTEGER NOT NULL,
    server_id INTEGER REFERENCES servers(id) ON DELETE SET NULL,
    UNIQUE(node_id, ip, port)
);

CREATE TABLE nests (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE eggs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    nest_id         INTEGER NOT NULL REFERENCES nests(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    docker_image    TEXT NOT NULL,
    startup_cmd     TEXT NOT NULL,
    stop_signal     TEXT NOT NULL DEFAULT 'SIGTERM',
    install_script  TEXT NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_eggs_nest ON eggs(nest_id);

CREATE TABLE egg_variables (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    egg_id        INTEGER NOT NULL REFERENCES eggs(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    default_value TEXT NOT NULL DEFAULT '',
    user_viewable INTEGER NOT NULL DEFAULT 0,
    user_editable INTEGER NOT NULL DEFAULT 0,
    description   TEXT NOT NULL DEFAULT '',
    UNIQUE(egg_id, name)
);

CREATE TABLE servers (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid          TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    owner_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    node_id       INTEGER NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,
    egg_id        INTEGER NOT NULL REFERENCES eggs(id) ON DELETE RESTRICT,
    allocation_id INTEGER NOT NULL REFERENCES allocations(id) ON DELETE RESTRICT,
    memory_mb     INTEGER NOT NULL,
    disk_mb       INTEGER NOT NULL,
    cpu_percent   INTEGER NOT NULL DEFAULT 100,
    status        TEXT NOT NULL DEFAULT 'offline'
                  CHECK (status IN ('offline','starting','running','stopping','installing','suspended')),
    created_at    INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_servers_owner ON servers(owner_id);
CREATE INDEX idx_servers_node ON servers(node_id);

CREATE TABLE server_variables (
    server_id   INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    variable_id INTEGER NOT NULL REFERENCES egg_variables(id) ON DELETE CASCADE,
    value       TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (server_id, variable_id)
);

CREATE TABLE activity_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    actor_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
    server_id  INTEGER REFERENCES servers(id) ON DELETE SET NULL,
    action     TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_activity_actor ON activity_logs(actor_id);
CREATE INDEX idx_activity_server ON activity_logs(server_id);
