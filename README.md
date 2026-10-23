# GarudaPanel

Panel manajemen game server ala Pterodactyl dalam satu binary statis Go.
Spesifikasi lengkap: [SPEC.md](SPEC.md).

## Status: Milestone 5 (Final Polish + Hardening + QA)

Semua modul inti telah lengkap:
- **Panel & Daemon Dual-Role**: Satu binary statis Go, CGO-free, tanpa dependensi eksternal.
- **ContainerRuntime interface** (`internal/dockerrt`): implementasi asli Docker SDK v27 + FakeRuntime untuk unit test tanpa Docker.
- **Supervisor daemon** (`internal/daemon`): single source of truth untuk siklus hidup container (start/stop/restart/kill).
- **Live console**: exec TTY, PTY resize, ring buffer 100KB per container + replay saat reconnect.
- **File Manager Streaming Proxy**: upload & download streaming murni 32KB chunk (tanpa buffer RAM), quota event-driven + throttled scan, batas upload configurable (default 100MB).
- **Error consistency**: respon 503 "node offline" konsisten di semua endpoint yang membutuhkan koneksi worker daemon (files, power, console).
- **Frontend Svelte**: Gaming dark UI di-embed langsung via `go:embed` dengan SPA fallback.
- **Installer & Manager** (`install.sh`): service systemd dengan user terisolasi, LimitNOFILE 65535, dan uninstaller.

---

## 1. Arsitektur Singkat

GarudaPanel dirancang dengan konsep *single-binary, dual-role*:

```
+--------------------------------------------------------------+
|                     Browser / Web UI                         |
|   (Svelte + Vite + Tailwind + Xterm.js di-embed via go:embed)|
+------------------------------+-------------------------------+
                               | REST & WebSocket
                               v
+--------------------------------------------------------------+
|                    GarudaPanel (Panel)                       |
|  - SQLite (pure Go modernc.org/sqlite, WAL mode)             |
|  - Auth Argon2id + Stateful Session Tokens (SHA-256)         |
|  - Reverse WebSocket Hub (menerima koneksi keluar node)      |
|  - File Streaming Proxy (32KB chunks, no-RAM-buffer)         |
+------------------------------^-------------------------------+
                               | Reverse WS (X-Node-Token)
+------------------------------+-------------------------------+
|                    GarudaPanel (Daemon)                      |
|  - Berjalan di tiap node worker                              |
|  - Docker Container Engine (ContainerRuntime abstraction)    |
|  - File Manager Engine (Quota event-driven, UID/GID sync)    |
|  - Ring Buffer 100KB log console + Replay otomatis           |
+--------------------------------------------------------------+
```

1. **Panel Mode (`garuda panel`)**:
   - Menyajikan antarmuka Web SPA modern dan REST API.
   - Mengelola database SQLite untuk users, nodes, servers, eggs, dan activity log.
   - Menjalankan **DaemonHub**: daemon dari node-node luar yang melakukan koneksi *reverse WebSocket* ke panel. Panel tidak memerlukan akses port masuk ke node.
2. **Daemon Mode (`garuda daemon`)**:
   - Berjalan langsung di host node worker yang terpasang Docker daemon.
   - Menukar token join satu kali pakai menjadi token node permanen tersimpan (`node-token`).
   - Bertindak sebagai *single source of truth* untuk siklus hidup container (Start, Stop, Restart, Kill).
   - Mengontrol streaming console PTY, ring buffer log, dan operasi file sistem container secara aman tanpa root leakage.

---

## 2. Cara Install via `install.sh`

Script installer serbaguna untuk Ubuntu / Debian Linux:

```bash
# Berikan hak eksekusi lalu jalankan sebagai root / sudo
chmod +x install.sh
sudo ./install.sh
```

Menu interaktif akan muncul:
- **1) Panel**: Membuat user sistem `garudapanel`, direktori `/var/lib/garudapanel`, mengompilasi/menyalin binary ke `/usr/local/bin/garuda`, dan mendaftarkan service `garuda-panel.service`.
- **2) Daemon**: Meminta URL Panel dan Join Token, mendaftarkan `garuda-daemon.service` yang bergantung pada `docker.service`.
- **3) Keduanya (All-in-one)**: Memasang Panel dan Daemon di satu mesin yang sama (cocok untuk VPS tunggal).
- **4) Uninstall**: Menghentikan & menghapus service systemd, binary, dan opsi pembersihan direktori data (`sudo ./install.sh uninstall`).

---

## 3. Cara Pakai Dasar

### Langkah 1: Jalankan Panel & Dapatkan Akses Admin
Jika dijalankan manual:
```bash
CGO_ENABLED=0 go build -o garuda ./cmd/garuda
./garuda panel --addr :8080 --db garuda-panel.db
```
*Catatan:* Pada peluncuran pertama dengan database kosong, panel akan mencetak kredensial bootstrap otomatis:
```
BOOTSTRAP ADMIN dibuat -> username: admin  password: gp-xxxxxx
```
Buka browser di `http://<IP-VPS>:8080` lalu masuk dengan kredensial tersebut.

### Langkah 2: Buat Lokasi dan Node di Panel
1. Buka menu **Admin -> Locations**, klik **+ Tambah Lokasi** (misal kode: `id-jkt`, nama: `Jakarta DC`).
2. Buka menu **Admin -> Nodes**, klik **+ Tambah Node**:
   - Masukkan Nama Node, FQDN/IP Host, RAM, dan Disk.
   - Klik **Simpan Node**.
3. Modal akan menampilkan **Join Token** sekali pakai (berlaku 15 menit). Salin token tersebut.
4. Tambahkan port alokasi untuk node tersebut (misal IP: `0.0.0.0`, Port: `25565`).

### Langkah 3: Hubungkan Daemon ke Panel
Di server node (atau terminal lain):
```bash
./garuda daemon --panel http://127.0.0.1:8080 --join <JOIN_TOKEN> --data /var/lib/garudapanel
```
Daemon akan menukar token, terhubung ke reverse WebSocket panel, dan status Node di web panel akan berubah menjadi **ONLINE** (hijau). Restart daemon berikutnya tidak memerlukan flag `--join` lagi karena token tersimpan otomatis di `/var/lib/garudapanel/node-token`.

### Langkah 4: Buat Server Game & Operasikan
1. Buka menu **Admin -> Servers**, klik **+ Buat Server**.
2. Pilih Egg template (misal: Minecraft Java / Paper), pilih Node yang online, pilih port alokasi, alokasi memori & disk, lalu tentukan pemilik server.
3. Klik **Buat Server**.
4. Di daftar server, klik server untuk masuk ke dashboard server:
   - **Console**: Lihat live log, kirim command, nyalakan (Start), matikan (Stop), atau paksa (Kill).
   - **Files**: Unggah file (drag & drop / streaming hingga 100MB default), edit file konfigurasi, buat folder, atau ekstrak zip.
   - **Startup**: Ubah variabel startup bawaan Egg secara dinamis.

---

## 4. Keterbatasan (Jujur & Transparan)

Untuk menjaga performa dan kesederhanaan pada rilis MVP ini, beberapa batasan berlaku:
1. **Membutuhkan Docker di Node Worker**: Daemon mengelola container melalui Docker socket host (`/var/run/docker.sock`). Node worker wajib memiliki Docker yang terpasang dan berjalan.
2. **Belum Ada Backup & Schedule Otomatis**: Fitur snapshot backup berkala ke S3/R2 dan cron scheduler tugas otomatis belum termasuk di MVP ini (dijadwalkan untuk rilis berikutnya).
3. **Belum Ada Sub-users / Granular Permissions**: Akses server saat ini berbasis role Admin atau Pemilik Server (Owner). Pembagian hak akses perizinan granular untuk sub-user belum didukung.
4. **Single Node per Database SQLite**: Database panel memakai SQLite WAL murni. Panel cocok untuk satu instance controller; bukan untuk cluster controller multi-master yang terdistribusi.

---

## 5. Build & Pengujian

### Build Statis Murni (Tanpa CGO)
```bash
CGO_ENABLED=0 go build -o garuda ./cmd/garuda
```

### Menjalankan Seluruh Test Suite
```bash
GOTOOLCHAIN=local go test -v -count=1 ./...
```

### Verifikasi Linting & Formatting
```bash
gofmt -l .
GOTOOLCHAIN=local go vet ./...
```
Semua perintah di atas harus bersih dan hijau tanpa error.

---

## 6. Struktur Direktori

```
cmd/garuda/          entrypoint (subcommand panel | daemon)
internal/db/         koneksi SQLite + migrasi embedded (migrations/*.sql)
internal/auth/       argon2id, session revocable, middleware RBAC
internal/panel/      handler REST admin + auth + files + router
internal/daemonhub/  hub koneksi reverse WS daemon + RPC router
internal/daemon/     client daemon: join, supervisor, container lifecycle, fileio
internal/dockerrt/   abstraksi Docker SDK (ContainerRuntime & FakeRuntime)
internal/filecore/   engine I/O file server (quota, permission sync, traversal guard)
internal/activity/   activity log & audit trail
web/                 frontend SPA Svelte (src/ + build dist/ di-embed via go:embed)
```

---

## 7. Catatan Keamanan (SPEC §8)

- Token session/node/join disimpan sebagai SHA-256 hash; raw hanya tampil sekali.
- Join token: sekali pakai, expiry 15 menit, konsumsi atomik di DB.
- Rate limit login: 5 percobaan/menit/IP.
- Body limiter & Upload cap: JSON API dibatasi 1MB, upload file dibatasi 100MB (configurable).
- Semua query SQL memakai parameter binding (anti SQL injection).
- Password di-hash menggunakan argon2id (m=64MB, t=3, p=2).
