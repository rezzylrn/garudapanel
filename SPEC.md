# GarudaPanel — SPEC v1.0

Panel manajemen game server ala Pterodactyl, dibangun dari NOL: gaya orisinal,
information architecture boleh contek Pterodactyl. Codename: **GarudaPanel**
(boleh diganti rezz kapan saja).

Status: 2026-10-23 — arsitektur diputuskan lewat debat Kubu Node vs Kubu Go +
tie-break Gemini 4 Argon. Pemenang: **Go, single static binary**.

---

## 1. Keputusan arsitektur (final, jangan diubah tanpa persetujuan rezz)

- **Satu binary statis** (`CGO_ENABLED=0`): `./garuda panel` dan `./garuda daemon`.
  Diferensiator inti vs Pterodactyl = instalasi satu perintah, tanpa PHP/MySQL/Redis.
- **Panel = toko konfigurasi + UI.** Daemon = **single source of truth** untuk
  siklus hidup container (start/stop/crash/restart). Panel tidak menyimpan state
  eksekusi; kalau koneksi putus, daemon tetap jalan, panel replay saat tersambung.
- **Reverse WebSocket**: daemon yang membuka koneksi WS persisten ke panel
  (bukan sebaliknya) → node di belakang NAT/CGNAT langsung berfungsi tanpa
  setting firewall. Auth: token per-node + TLS.
- **HTTP stack: `net/http` standar (Go 1.22+ routing) atau Chi.**
  DILARANG pakai Fiber/fasthttp — fasthttp tidak kompatibel dengan net/http dan
  memaksa payload besar (upload/backup) masuk buffer RAM → OOM di VPS 1GB.
- **WebSocket: `nhooyr.io/websocket`** (terawat, native net/http). Fallback
  `gorilla/websocket` hanya dengan justifikasi tertulis.
- **Database: SQLite + WAL mode** via `modernc.org/sqlite` (pure Go, tanpa CGO).
  Migrasi: `golang-migrate`, file SQL di-embed. Postgres hanya jika multi-region
  (fase 3+, bukan sekarang).
- **Password hashing: argon2id** via `golang.org/x/crypto` (pure Go, off-thread).
- **Frontend: Svelte + Vite + Tailwind + xterm.js**, hasil build di-embed via
  `go:embed`. Tanpa SSR, tanpa nginx.
- **Kontrak API ditulis SEKALI** (Go struct dishare panel↔daemon). Opsi hybrid
  (panel Node + daemon Go) DITOLAK — jebakan produktivitas untuk tim sekecil ini.

---

## 2. Stack final

| Lapisan      | Pilihan |
|---|---|
| Bahasa | Go 1.22+ |
| HTTP router | net/http std / Chi |
| WebSocket | nhooyr.io/websocket |
| DB | SQLite (WAL) + modernc.org/sqlite, migrasi golang-migrate |
| Auth | session token stateful (revocable) + argon2id, RBAC |
| Frontend | Svelte + Vite + Tailwind + xterm.js, go:embed |
| Docker | Docker SDK resmi for Go |
| Build | `CGO_ENABLED=0 go build` → 1 binary ±15-20MB |

---

## 3. Skema data (esensial)

- `users(id, username, email, password_hash, is_admin, totp_secret NULL, created_at)`
- `sessions(token_hash, user_id, expires_at, ip, user_agent)` — revocable
- `locations(id, short_code, description)`
- `nodes(id, location_id, name, fqdn, daemon_token_hash, memory_limit_mb, disk_limit_mb, created_at)`
- `allocations(id, node_id, ip, port, server_id NULL)` — port bisa di-seed dari range
- `nests(id, name)` — kategori game
- `eggs(id, nest_id, name, docker_image, startup_cmd, stop_signal, install_script, created_at)`
- `egg_variables(id, egg_id, name, default_value, user_viewable, user_editable, description)`
- `servers(id, uuid, name, owner_id, node_id, egg_id, allocation_id, memory_mb, disk_mb, cpu_percent, status, created_at)`
- `server_variables(server_id, variable_id, value)`
- `activity_logs(id, actor_id, server_id NULL, action, detail, created_at)`
- Fase 2 (skema boleh disiapkan, UI belakangan): `subusers`, `schedules`, `backups`, `database_hosts`.

Status server: `offline | starting | running | stopping | installing | suspended`.
Status adalah milik daemon; panel hanya menampilkan.

---

## 4. API surface

### Panel REST (user)
- `POST /api/auth/login|logout`, `GET /api/auth/me`
- `GET /api/servers`, `GET /api/servers/:id`
- `GET /api/servers/:id/ws` — WebSocket console (browser→panel, diproxy ke daemon)
- Power: `POST /api/servers/:id/power {action: start|stop|restart|kill}`
- Files: `GET/PUT/POST/DELETE /api/servers/:id/files...`, upload/download stream
- Stats: via WS yang sama (`{t:"stats"}`)
- `GET /api/servers/:id/activity`

### Panel REST (admin)
- CRUD `/api/admin/users`, `/api/admin/nodes`, `/api/admin/servers`,
  `/api/admin/eggs`, `/api/admin/locations`, `/api/admin/allocations`
- `POST /api/admin/nodes` → balikan **join token sekali pakai** untuk daemon
- `POST /api/admin/servers/:id/suspend|unsuspend|reinstall`

### Protokol daemon↔panel (satu WS reverse, multiplex channel)
Frame JSON: `{ch:"console:server-uuid"|"rpc"|"stats", type, payload}`.
- Daemon→panel: `hello` (auth token node), `state` (perubahan status),
  `console:out`, `stats`, `rpc:res`
- Panel→daemon: `power`, `console:in`, `rpc:req` (file ops, install), `ping`
- Daemon menyimpan **ring buffer 100KB log terakhir per container** di memori;
  panel minta `replay` saat (re)connect.

### Protokol console browser↔panel WS
- Client→server: `{t:"auth", token}`, `{t:"in", d:"..."}`, `{t:"resize", cols, rows}`
- Server→client: `{t:"out", d:"..."}`, `{t:"stat", d:{...}}`, `{t:"status", s:"..."}`
- Batas: maks 30 sesi console per server; throttle output 2000 baris/100ms;
  token WS sekali pakai, expiry 60 detik.

---

## 5. MVP scope

### IN (fase 1)
1. Auth + session + RBAC (`admin`, `owner`). 2FA: fase 2.
2. Admin: CRUD location/node/server/egg/user/allocation; join token node.
3. Daemon: register via join token, heartbeat, Docker create/start/stop/restart/
   kill dengan limit memory/CPU, volume per server, inject env var.
4. Console live (attach stdout/stderr + stdin) + ring buffer + replay.
5. Power actions + indikator status real-time.
6. File manager: browse/read/write/delete/rename, upload/download (streaming,
   JANGAN buffer seluruh file di RAM), ekstrak zip.
7. Statistik resource live (CPU, memory, disk, net) tiap 2 detik.
8. Egg sederhana: docker image + startup cmd + stop signal + install script +
   variables.
9. Activity log untuk semua aksi.
10. Build single binary + `install.sh` (binary + systemd unit).

### OUT (fase 2+)
Backups (lokal/S3), schedules/cron, subuser granular, database provisioning,
SFTP server built-in, transfer antar node, egg import/export JSON, billing,
multi-panel, 2FA.

---

## 6. Frontend (halaman)

- Login; daftar server (kartu: nama, status, CPU/RAM mini-graph);
- Detail server (tab): **Console** (xterm.js full, auto-reconnect + replay),
  **Files** (breadcrumb, editor, upload drag-drop), **Startup** (variabel egg),
  **Network** (allocations), **Activity**;
- Admin: Nodes, Servers, Eggs, Users, Locations — tabel + form.
- Gaya: orisinal, dark, gaming — BUKAN tiruan visual Pterodactyl.

---

## 7. Detail daemon (bagian tersulit — baca baik-baik)

1. **Lifecycle**: daemon satu-satunya yang boleh create/stop container.
   Crash → tandai `offline`, laporkan ke panel, JANGAN restart otomatis di MVP
   (kebijakan restart = fase 2).
2. **Console**: `ContainerExecCreate` + attach TTY; 2 goroutine per sesi
   (baca-tulis); tutup bersih saat WS putus (no goroutine leak — wajib test).
3. **File ops**: JANGAN pakai `filepath.Walk` tiap detik. Quota = akuntansi
   berbasis event (tambah/kurang saat upload/write/delete/extract) + background
   scan throttled (maks 1x/5 menit, nice rendah).
4. **Permission**: operasi file daemon HARUS memakai UID/GID yang sama dengan
   user di dalam container (baca dari image config). File yang ditulis via panel
   tidak boleh jadi root-owned.
5. **Path traversal**: semua path user divalidasi dengan `os.Root` (Go 1.24+)
   atau validasi symlink ketat — tidak boleh keluar dari volume server.
6. **Stats**: `container.Stats` streaming, ringkas di daemon, kirim tiap 2 dtk.

---

## 8. Keamanan (non-negotiable)

- Session token 32-byte random, disimpan sebagai hash; logout = revoke.
- Node token: join token sekali pakai (expiry 15 menit) → tukar jadi node token
  long-lived yang bisa di-revoke dari panel.
- Rate limit login (5x/menit/IP) + rate limit WS.
- Semua input path tervalidasi (lihat §7.5). Semua query pakai parameter binding.
- Daemon hanya listen di localhost untuk hal non-WS; tidak ada port publik
  tambahan selain panel.

---

## 9. Risiko & mitigasi (dari Gemini 4 Argon + riset Pterodactyl)

| Risiko | Mitigasi |
|---|---|
| WS putus saat install/start | Daemon = source of truth + ring buffer; panel replay |
| File jadi root-owned, server gagal jalan | I/O daemon pakai UID/GID container |
| Path traversal keluar volume | `os.Root` / validasi symlink ketat |
| Quota check bunuh I/O VPS kentang | Akuntansi event + scan throttled |
| Upload 500MB → OOM | Streaming murni, tanpa fasthttp/buffer penuh |
| Token node bocor | Token per-node, revocable, scope terbatas |
| Goroutine leak di console | Test: buka/tutup 100 sesi, goroutine count kembali |

---

## 10. Fase & estimasi (1 eksekutor)

- **Minggu 1**: fondasi — migrasi DB, auth/RBAC, CRUD admin, struktur `panel`/`daemon`.
- **Minggu 2**: daemon Docker + power + console WS + ring buffer + replay.
- **Minggu 3**: file manager (daemon RPC + panel proxy) + stats stream + egg variables.
- **Minggu 4-5**: frontend Svelte (semua halaman §6) + go:embed + install.sh + hardening.
- Total MVP: **±5 minggu-kerja**.

---

## 11. Aturan eksekusi (untuk Hermes)

1. Ikuti **coding pipeline**: check code → check bug → check security → fix →
   implement. Temuan kritis = STOP + konfirmasi ke rlnmy, jangan lanjut diam-diam.
2. Setiap modul selesai = tulis ringkasan + cara verifikasi. Klaim "selesai"
   tanpa bukti = belum selesai.
3. Jangan ubah keputusan §1 tanpa persetujuan rezz.
4. Commit git per milestone dengan pesan jelas. Repo: `~/workspace/garuda-panel/`.
5. rlnmy me-review sebelum milestone dinyatakan done.

---

## 12. Definisi "selesai" MVP (acceptance)

- [ ] `CGO_ENABLED=0 go build -o garuda ./cmd/garuda` → 1 binary, jalan di
      Ubuntu 22.04 fresh tanpa dependensi tambahan.
- [ ] `./garuda panel` serve UI + API; `./garuda daemon --join <token>`
      register ke panel tanpa error.
- [ ] Buat node + egg + server dari UI admin; install script jalan; status
      berubah `installing → offline`.
- [ ] Start server dari console; log live muncul <1 dtk; kirim perintah via
      input; stop graceful; kill paksa.
- [ ] File manager: upload file 500MB tanpa OOM; edit file; hasil edit
      terbaca game server (permission benar).
- [ ] Matikan koneksi WS daemon 30 detik lalu sambungkan lagi: install/start
      tetap jalan, log ter-replay.
- [ ] Buka+tutup console 100x: jumlah goroutine daemon kembali ke baseline.
