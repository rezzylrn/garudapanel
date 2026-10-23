# GarudaPanel

Panel manajemen game server ala Pterodactyl dalam satu binary statis Go.
Spesifikasi lengkap: [SPEC.md](SPEC.md).

## Status: Milestone 2 (daemon + power + console)

Tambahan milestone-2:
- **ContainerRuntime interface** (`internal/dockerrt`): implementasi asli Docker
  SDK v27 + FakeRuntime untuk unit test tanpa Docker. Semua logika daemon
  ditulis terhadap interface — tidak menyentuh SDK langsung.
- **Supervisor daemon** (`internal/daemon`): daemon satu-satunya yang boleh
  mengubah state container (start/stop/restart/kill), tiap transisi state
  dilaporkan ke panel via WS. Tidak ada auto-restart (MVP).
- **Power end-to-end**: REST panel → WS RPC multiplex → ContainerRuntime.
- **Live console**: exec TTY, 2 goroutine per sesi, tutup bersih saat WS putus
  (test: 100x buka-tutup, goroutine kembali ke baseline — `go test -race` hijau).
- **Ring buffer 100KB** log terakhir per container + replay saat reconnect.
- **Console browser↔panel**: token sekali pakai expiry 60 detik, frame
  `{t:auth|in|out|stat|status|resize}`, throttle 2000 baris/100ms, maks 30
  sesi per server.
- **Stats**: snapshot docker stats per 2 detik via WS (`StatsProvider`).
- **Streaming murni**: console I/O via io.Copy + io.Pipe; tidak ada buffer
  file penuh di RAM; WS read limit 1MB.

## Milestone 1 (fondasi)

Yang sudah jalan:
- Auth: login/logout, session token stateful revocable (disimpan sebagai SHA-256 hash), argon2id, rate limit login 5x/menit/IP, RBAC (admin/owner).
- REST admin: CRUD users, locations, nodes (+ join token sekali pakai, expiry 15 menit), allocations, nests, eggs, servers.
- Daemon skeleton: tukar join token → node token long-lived, reverse WebSocket ke panel, heartbeat tiap 10 detik. Panel mencatat node online/offline.
- Frontend placeholder (`web/dist`).

## Build

```bash
CGO_ENABLED=0 go build -o garuda ./cmd/garuda
```

Hasil: satu binary statis ±15MB, jalan di Ubuntu 22.04 fresh tanpa dependensi.

## Menjalankan

### 1. Panel

```bash
./garuda panel --addr :8080 --db garuda-panel.db
# Output BOOTSTRAP ADMIN (username: admin + password acak) hanya muncul saat
# database masih kosong — simpan segera. Bisa juga: --bootstrap-admin PASSWORD.
```

### 2. Buat node + join token (dari API)

```bash
TOKEN=<login-token-admin>   # dari POST /api/auth/login
curl -X POST localhost:8080/api/admin/locations -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"short_code":"sg","description":"Singapore"}'
curl -X POST http://localhost:8080/api/admin/nodes \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"location_id":1,"name":"node1","fqdn":"node1.example.com"}'
# Respons berisi "join_token" — sekali pakai, kedaluwarsa 15 menit.
```

### 3. Daemon (di mesin node)

```bash
./garuda daemon --panel http://panel-host:8080 --data /var/lib/garudapanel \
  --join <JOIN_TOKEN>
```

Daemon menukar join token jadi node token long-lived (disimpan di
`--data/node-token`, mode 0600). Restart berikutnya cukup:

```bash
./garuda daemon --panel http://panel-host:8080 --data /var/lib/garudapanel
```

Daemon membuka reverse WebSocket ke panel (`/api/daemon/ws`, header
`X-Node-Token`) dan mengirim heartbeat tiap 10 detik. Node yang disconnect
otomatis ditandai offline di panel.

## Verifikasi cepat

```bash
# login
curl -s localhost:8080/api/auth/login -X POST -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"..."}'

# semua endpoint admin butuh header: Authorization: Bearer <token>
curl http://localhost:8080/api/auth/me -H "Authorization: Bearer $TOKEN"
curl http://127.0.0.1:8080/api/admin/nodes -H "Authorization: Bearer $TOKEN"
```

## Struktur

```
cmd/garuda/          entrypoint (subcommand panel | daemon)
internal/db/         koneksi SQLite + migrasi embedded (migrations/*.sql)
internal/auth/       argon2id, session revocable, middleware RBAC
internal/panel/      handler REST admin + auth + router
internal/daemonhub/  hub koneksi WS daemon + endpoint /api/daemon/ws + /api/daemon/join
internal/daemon/     client daemon: join, reverse WS, heartbeat 10 dtk
internal/activity/   activity log
web/dist/            placeholder frontend (Svelte+Vite di fase 4, go:embed)
```

## Catatan keamanan (SPEC §8)

- Token session/node/join disimpan sebagai SHA-256 hash; raw hanya tampil sekali.
- Join token: sekali pakai, expiry 15 menit, konsumsi atomik di DB.
- Rate limit login: 5 percobaan/menit/IP.
- Semua query pakai parameter binding. Password: argon2id (m=64MB,t=3,p=2).
