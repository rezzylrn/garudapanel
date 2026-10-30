# GarudaPanel — Audit Node.js Egg "Bot Whatsap" (2026-10-30 dini hari)

Perintah rezz: audit semua sampai bisa jalanin Node.js pakai egg ini (egg_26_jrjw.json).
Status akhir: **JS BERHASIL LAUNCH via tombol Start, console live.** Commit 08e91ac.

## 1. Bug servers.php — FIXED (commit ef8c774)
- `pages/servers.php` require `partials/header.php` & `footer.php` tanpa `../` → fatal error.
- Satu-satunya file yang salah; sweep 11 halaman: HTTP 200, 0 fatal error.

## 2. Bug PHP warnings (dari SS rezz 01:33) — FIXED
- Startup page: `$vars['data']` padahal API return `{"variables":[...]}` → 1 baris warning + tabel kosong. Fix: `['data']['variables']`.
- Files page: `$en['is_dir']` padahal API return `mode: "f"/"d"` → warning tiap baris. Fix: `mode === 'd'` + kolom mod_time (Go: Entry.ModTime).
- Startup command tampil "-" karena server detail tidak punya startup_cmd. Fix Go: JOIN eggs + `startup_cmd` di response.
- List servers: tidak ada egg_name/node_name/owner_name → tampil "-". Fix Go: JOIN di kedua list query.

## 3. Bug WS panel→daemon (frame power hilang) — FIXED
- Akar: sync hello ditulis LANGSUNG via writeFrame sementara goroutine writeCh juga menulis WS yang sama (concurrent write dilarang nhooyr/websocket). Goroutine tulis mati diam-diam → frame power masuk lubang hitam (202 tetap kembali).
- Fix: SEMUA tulis panel→daemon lewat writeCh (c.Send), termasuk sync hello. Sisi daemon: hello juga lewat writeCh.
- Bukti: instrumentasi frame log — power sampai setelah fix.

## 4. Bug console WS browser 401 — FIXED
- `/ws` tertutup middleware RequireAuth; browser tidak bisa kirim header Bearer saat handshake WS → console dari browser SELALU 401 (E2E sebelumnya lolos karena test pakai header).
- Fix: `/ws` keluar dari grup RequireAuth; auth via frame pertama console-token 60 detik (sesuai desain).
- Console page: WS base URL dari `ws_url.txt` (tunnel serveo kedua khusus port 8080, keepalive-ws.sh) — browser di luar VM tidak bisa capai 127.0.0.1:8080.

## 5. Startup command egg tidak pernah dikirim — FIXED
- Sync payload hardcode placeholder `echo 'garuda: startup command belum dikonfigurasi'`.
- Fix: SELECT startup_cmd, substitusi {{VAR}} dari server variables, Cmd = ["/bin/bash", "-c", script] (bash wajib — egg pakai [[ ]]).

## 6. Docker — GAGAL, batasan keras sandbox (verifikasi 2 jalur)
- `docker.io` 29.1.3 terinstall; dockerd jalan dengan `--iptables=false --bridge=none --storage-driver=vfs`.
- `docker run` GAGAL final: `runc create failed: ... setns: operation not permitted` (seccomp).
- Solusi: **ProcessRuntime** (internal/dockerrt/process.go) — daemon fallback otomatis bila Docker ping gagal. Proses jalan langsung di host (TANPA isolasi, dicatat di log). Fitur: broadcast output + replay 100KB, stdin, stop grup sinyal, exit event, stats /proc. Strip prefix /usr/local/bin yolks (path image tidak ada di host).

## 7. E2E JS launch — LOLOS (2026-10-30 01:47 WIB)
- POST /power start → daemon frame power → ProcessRuntime start → `node index.js` hidup (PID terverifikasi).
- Status: offline → running. Console WS (tanpa Bearer): status Connected + replay output asli (`npm start`, `Bot WhatsApp starting...`, `running`, heartbeat) + stats mem 70MB + frame live.
- POST /power stop → SIGINT grup → offline, tidak ada proses tersisa.
- 11/11 halaman PHP: HTTP 200, 0 warning. Isi terverifikasi (startup cmd asli, 5 variables, file + mod_time, egg/node/owner name).

## 8. UI remake — palet biru profesional
- Aksen hijau #4ade80 → biru #3b82f6 (bg navy-slate). Hijau #34d399 hanya untuk status semantik (running/ok). Tombol primary teks putih.

## 9. Responsif mobile + power guard — riset Pterodactyl (2026-10-30 ~02:10 WIB)
- Riset source `pterodactyl/panel` (PowerButtons.tsx, ServerConsoleContainer.tsx, Console.tsx, SubNavigation): Start hanya dari offline, Stop jadi Kill saat stopping, dialog konfirmasi hanya untuk Kill, terminal pakai FitAddon, layout grid stacking di mobile.
- **Responsif**: nol horizontal scroll di semua halaman pada 390px — tabel berubah jadi kartu bertumpuk (label kolom dari `<th>` via JS footer), `overflow-x: clip` di html/body, startup command wrap, terminal pakai FitAddon + frame resize ke daemon, toolbar files menumpuk.
- **Power 2 langkah**: semua aksi (Start/Restart/Stop/Kill) lewat dialog konfirmasi. Tombol diblokir tetap bisa diklik (aria-disabled) supaya alasannya tampil sebagai toast error.
- **Startup-done (temuan riset)**: `config_startup_done` egg selama ini TIDAK PERNAH dibaca daemon — status langsung "running" begitu proses hidup, sehingga jendela "starting" tidak pernah ada. Sekarang: ProcessRuntime memantau output, status tetap "starting" sampai teks done muncul (pola wings); sync payload panel→daemon membawa `startup_done`.
- **State guard backend** (`powerAllowed`, unit test): start hanya dari offline; stop/restart hanya dari running; stop/restart saat starting → 409 "still starting"; kill = escape hatch (boleh saat starting/stopping/running). Jendela balapan 3 detik (bukan 20 — 20 detik salah memblokir stop yang sah).
- **E2E guard 11/11 LOLOS** (guard-test.py): stop/restart saat starting → 409, kill → 202 → offline, start saat running → 409, stop saat offline → 409, stop normal → offline.

## Yang harus rezz lakukan untuk E2E Docker penuh
Jalankan installer di PC Pop!_OS (punya Docker) → daemon join (pakai Docker runtime otomatis) → buat server pakai egg ini → Start → console harus keluar "running".

## 10. Console stats, lifecycle lines, file editor (2026-10-30, commit 5a06c37)

Perintah rezz (SS startup page): variabel masih belum oke; console harus bersih
(hanya output script + start/restart/stop); tambah statistik server mirip Pterodactyl;
lalu interupsi: "kasih edit file juga di file managernya".

- **Startup variables**: tabel diganti `.var-list` (grid nama 250px + kontrol) —
  nama tidak terpotong lagi di mobile; input + Save lega. 5 var-item terverifikasi.
- **Console bersih**: heartbeat berasal dari script tes (`setInterval` tiap 5 detik),
  BUKAN dari panel — script diganti diam (hanya `starting...` + `running`).
  Daemon kini mencetak baris lifecycle `[GarudaPanel] Server started./stopped./
  restarted.` ke ring buffer + live (ikut replay). Verifikasi WS: replay setelah
  stop mengandung kedua baris; TIDAK ada heartbeat.
- **Stats strip** (pola ServerDetailsBlock Pterodactyl): Address (IP:port), Uptime,
  CPU Load, Memory (+bar), Disk (+bar, via endpoint `server/usage`), Network
  (in/out). Frame stat WS tiap 2 detik kini membawa `uptime_seconds`, `state`,
  dan CPU% riil (delta jiffies /proc vs baseline; panggilan pertama 0).
  Verifikasi: uptime 11 dtk, mem 77,8/512 MB, state running.
- **File editor**: tombol Edit per file + New file; editor textarea mono 55vh,
  guard 512 KB/binary/1 MB; save via PUT mentah. **Delete diperbaiki**: handler
  delete tadinya di dalam blok POST padahal link-nya GET — tidak pernah jalan.
- **BUG KRITIS upload 0 byte**: `file:write-begin` di-dispatch `go` sementara
  chunk sinkron → chunk kalah balapan, file tertulis 0 byte (curl/PHP selalu
  gagal, urllib kadang lolos). Fix: write-begin sinkron; tes regresi
  `TestFileWriteBeginChunkRace` 20x via HandleFrame; `go test -race` lolos
  (termasuk perbaikan lock `MemFS.resolveTarget`).
- E2E penuh `console-verify.py`: ALL PASS. Server dibiarkan RUNNING.
- Satu false alarm saat debug: 2 sesi WS pasca-stop gagal karena BUG SCRIPT TES
  (token diambil dari key `data` yang tidak ada → token null → 401). Bukan bug panel.

## 11. Egg library, node Activate, grafik traffic, console transparan (2026-10-30)
- **Egg library** (`eggs/`, generator `generate.py`): 15 egg PTDL_v2 — Minecraft (Paper, Bedrock, Forge), Node.js (Node.js, Discord.js Bot), Python (Python, Django, Flask), Web Hosting (PHP Website, Static Website, Laravel), Game Servers (Terraria, Valheim, Rust, FiveM). Semua ter-import ke live DB (15/15, 0 skip). Aturan done-string: kosong kalau tidak pasti (lihat AGENTS.md).
- **Node Activate**: endpoint baru `POST /api/admin/nodes/{id}/join-token` (Go, token segar 15 menit) + tombol Activate di halaman Nodes untuk node offline → modal berisi join token + script Docker (alpine + binary release, jalan hari ini) + script binary langsung, tombol copy. Panel URL diambil dari `ws_url.txt` (tunnel API Go).
- **Grafik traffic**: canvas hand-rolled di halaman console (tanpa dependensi) — RX biru / TX hijau, 90 sampel terakhir (~3 menit), rate dihitung dari delta net kumulatif stat frame. Terverifikasi: stat frame mengalir (console-verify ALL PASS).
- **Console transparan + rapih**: xterm `allowTransparency` + theme background transparan, kartu console `.glass` (backdrop blur), tombol Clear, toolbar console-bar dengan spacer. `Dockerfile.daemon` ditambahkan ke repo (image proper; perintah Docker di modal tidak butuh image custom).
- **Verifikasi**: `go vet` + `go test ./...` lolos; join-token endpoint 200 + token 15 menit; modal Activate ter-render; egg baru muncul di dropdown New Server; console-verify ALL PASS; server dibiarkan RUNNING.
