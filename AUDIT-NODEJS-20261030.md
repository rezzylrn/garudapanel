# GarudaPanel — Audit Node.js Egg "Bot Whatsap" (2026-10-30 dini hari)

Perintah rezz: audit semua sampai bisa jalanin Node.js pakai egg ini (egg_26_jrjw.json).
Status akhir: SEMUA JALUR TERVERIFIKASI kecuali eksekusi container — terblokir sandbox VM.

## 1. Bug servers.php — FIXED (commit ef8c774)
- `pages/servers.php` require `partials/header.php` & `footer.php` tanpa `../` → fatal error.
- Satu-satunya file yang salah; sweep 11 halaman: HTTP 200, 0 fatal error.

## 2. Egg import — OK
- Egg "Bot Whatsap" (PTDL_v2) import via API JSON `{nest_id, egg}` → ID 2, 5 variables.
- Catatan: endpoint import HANYA terima JSON, bukan multipart (curl awal gagal "invalid body" — salah format, bukan bug).
- Variables tampil di GET /api/servers/{id}/variables: GIT_ADDRESS, BRANCH, USERNAME, ACCESS_TOKEN, CMD_RUN (default "npm start").

## 3. Infra test — OK
- Nest "JavaScript" (ID 3), location "local" (ID 2), node "local-node2" (ID 3, ONLINE via daemon join).
- Server "Bot WhatsApp Test" (ID 2, UUID 118c682b-...) di node 3.
- Daemon terima `servers.sync: 1 registered`.
- Upload package.json + index.js via file API → 200, tampil di listing.
- Power `start` sampai ke daemon (202 Accepted, "command sent").

## 4. Docker — GAGAL, batasan keras sandbox (verifikasi 2 jalur)
- `docker.io` 29.1.3 terinstall; dockerd jalan dengan `--iptables=false --bridge=none --storage-driver=vfs`.
- `docker pull` gagal (TLS timeout — dockerd abaikan env proxy); workaround curl+`docker load` BERHASIL load hello-world.
- `docker run` GAGAL final: `runc create failed: ... setns: operation not permitted` (seccomp blokir syscall namespace).
- Kesimpulan: container TIDAK BISA jalan di VM ini. Siklus hidup container (start→running→stop) HANYA bisa diverifikasi di host ber-Docker (PC rezz / VPS). Daemon di VM ini harus pakai FakeRuntime.

## 5. Fallback: startup flow egg dengan Node.js asli — OK
- Dijalankan di volume dir server ASLI (`daemon/data/volumes/118c682b-.../`), env vars sama persis (CMD_RUN="npm start"), Node v24.22.1.
- Output: `Bot WhatsApp starting...` → `running` (teks startup-done egg) → heartbeat tiap 5 dtk.
- Terbukti: parsing egg, substitusi variabel, startup command, dan deteksi "running" semuanya benar. Yang belum terbukti HANYA isolasi container-nya.

## 6. Temuan minor
- Daemon log: `chown ... operation not permitted` saat upload file (daemon non-root di sandbox). Upload tetap sukses (200). Di production (daemon root) chown jalan normal.
- Serveo tunnel tidak stabil (putus tiap ~2-5 mnt, URL ganti). Cloudflared diblokir proxy (TLS intercept). Untuk demo ke rezz: pakai URL terbaru dari current_domain.txt.
- PHP 8.5: `curl_close()` deprecated → warning merusak redirect (sudah dihapus dari lib/api.php).

## Yang harus rezz lakukan untuk E2E penuh
Jalankan installer di PC Pop!_OS (punya Docker) → daemon join → buat server pakai egg ini → Start → console harus keluar "running".
