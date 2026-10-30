// Package runtime: abstraksi container runtime untuk daemon.
//
// Semua logika daemon (lifecycle, power, console, stats) ditulis terhadap
// interface ContainerRuntime — BUKAN langsung terhadap Docker SDK. Alasannya:
// - daemon harus bisa di-test tanpa Docker (unit test pakai FakeRuntime),
// - SDK asli hanya menyentuh satuan method ini, gampang diaudit.
package dockerrt

import (
	"context"
	"io"
)

// State lifecycle container (mirror status server SPEC §3).
const (
	StateOffline  = "offline"
	StateStarting = "starting"
	StateRunning  = "running"
	StateStopping = "stopping"
)

// ContainerSpec konfigurasi create container (dipakai daemon saat create).
type ContainerSpec struct {
	Ref          string   // uuid server — dipakai sebagai nama container (garuda-<uuid>)
	Image        string   // docker image
	Cmd          []string // argv
	Env          []string // KEY=value
	MemoryMB     int64    // limit memori (0 = tanpa limit)
	CPUPercent   int      // limit CPU dalam persen satu core (0 = tanpa limit)
	StopSignal   string   // mis. "SIGTERM" (default SIGTERM)
	BindPort     int      // port host yang dipublish (0 = tidak)
	HostPath     string   // volume data server di host (kosong = tanpa volume)
	ContainerUID int      // UID di dalam container (untuk chown file, fase 3)
	DiskMB       int64    // limit disk volume server (quota, SPEC §7.3)
	ImageUser    string   // field User image config (UID/GID operasi file, SPEC §7.4)
	// StartupDone: teks penanda server selesai start (egg config.startup.done,
	// pola Pterodactyl wings). Kosong = running begitu proses hidup. Bila
	// diisi, status tetap "starting" sampai teks ini muncul di output —
	// selama itu Stop/Restart diblokir state guard panel.
	StartupDone string
}

// FileOwner: UID/GID untuk operasi I/O file volume (SPEC §7.4). Sumber:
// image config (ImageUser). Fallback 1000:1000 + log via filecore.OwnerFor.
func (c ContainerSpec) FileOwner() (uid, gid int) {
	return filecoreOwnerFor(c.Ref, c.ImageUser)
}

// ExecResult hasil ExecAttach: exit code proses di dalam container.
type ExecResult struct {
	ExitCode int
}

// Runtime adalah satu-satunya jalan daemon menyentuh Docker.
// Semua method harus aman dipanggil dari goroutine mana pun.
type ContainerRuntime interface {
	// Create membuat (atau mengganti) container untuk spec.
	Create(ctx context.Context, spec ContainerSpec) error
	Start(ctx context.Context, ref string) error
	Stop(ctx context.Context, ref string, timeoutSec int) error
	Restart(ctx context.Context, ref string, timeoutSec int) error
	Kill(ctx context.Context, ref string) error
	// Remove menghapus container (dipakai saat uninstall/reinstall server).
	Remove(ctx context.Context, ref string, force bool) error
	// State membaca status sekarang tanpa mengubah apa pun.
	State(ctx context.Context, ref string) (string, error)
	// ExecAttach membuka exec TTY (console): stdin dari in, stdout+stderr
	// ke out (streaming; JANGAN pernah buffer semua output di memori).
	// Return exec ID yang bisa di-Resize / di-Inspect.
	ExecAttach(ctx context.Context, ref string, in io.Reader, out io.Writer) (ExecHandle, error)
	// Events mengirim event perubahan state (state lifecycle) ke ch.
	// Berkas dipanggil sekali per runtime; channel ditutup saat ctx.Done.
	Events(ctx context.Context, ch chan<- StateEvent)
}

// ExecHandle: sesi exec aktif.
type ExecHandle interface {
	// Resize mengubah ukuran TTY (dari frame resize browser).
	Resize(ctx context.Context, cols, rows uint) error
	// Close menutup sesi exec (stdin ditutup, stream berhenti).
	Close() error
}

// StatsProvider: runtime yang bisa memberi snapshot stats per container
// (opsional — diimplement Docker asli + Fake untuk test).
type StatsProvider interface {
	// StatsOnce ambil satu snapshot stats container ref.
	StatsOnce(ctx context.Context, ref string) (StatsSnapshot, error)
}

// StatsSnapshot rangkuman resource satu container.
type StatsSnapshot struct {
	ServerUUID   string  `json:"server_uuid,omitempty"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemUsageMB   float64 `json:"mem_used_mb"`
	MemLimitMB   float64 `json:"mem_limit_mb"`
	NetRxKB      float64 `json:"net_rx_kb"`
	NetTxKB      float64 `json:"net_tx_kb"`
	BlockReadMB  float64 `json:"block_read_mb"`
	BlockWriteMB float64 `json:"block_write_mb"`
	UptimeSec    int64   `json:"uptime_seconds"`
}

// StateEvent: perubahan status satu container (dari event Docker atau polling).
type StateEvent struct {
	Ref    string // uuid server
	Status string // offline/starting/running/stopping
}
