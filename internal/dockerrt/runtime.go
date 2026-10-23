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
	Ref          string            // uuid server — dipakai sebagai nama container (garuda-<uuid>)
	Image        string            // docker image
	Cmd          []string          // argv
	Env          []string          // KEY=value
	MemoryMB     int64             // limit memori (0 = tanpa limit)
	CPUPercent   int               // limit CPU dalam persen satu core (0 = tanpa limit)
	StopSignal   string            // mis. "SIGTERM" (default SIGTERM)
	BindPort     int               // port host yang dipublish (0 = tidak)
	HostPath     string            // volume data server di host (kosong = tanpa volume)
	ContainerUID int               // UID di dalam container (untuk chown file, fase 3)
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

// StateEvent: perubahan status satu container (dari event Docker atau polling).
type StateEvent struct {
	Ref    string // uuid server
	Status string // offline/starting/running/stopping
}
