// Package filecore: operasi file volume server (SPEC §7.3-7.5).
//
// Semua logika file manager daemon ditulis terhadap interface FS — bukan
// os langsung — supaya bisa di-unit-test tanpa Docker dan tanpa mount asli
// (pola sama dengan ContainerRuntime di milestone 2). Implementasi FS asli
// (osFS) wajib memakai os.Root (Go 1.24+) ATAU validasi symlink ketat.
//
// Keamanan non-negotiable (SPEC §8/§9):
//   - path user tidak boleh keluar dari volume server (path traversal,
//     symlink escape) — HARUS ditolak dengan ErrOutsideVolume;
//   - hasil operasi owned UID/GID container (fallback 1000:1000 + log).
package filecore

import (
	"errors"
	"io"
)

// ErrOutsideVolume: path menunjuk keluar volume (traversal/symlink).
var ErrOutsideVolume = errors.New("path resolves outside server volume")

// ErrQuotaExceeded ditolak bila write/upload/extract melampaui disk quota.
var ErrQuotaExceeded = errors.New("disk quota exceeded")

// Entry satu entri daftar file (bentuk wire JSON, dishare panel↔daemon).
type Entry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Mode  string `json:"mode"`     // "f" file, "d" dir, "s" symlink, lainnya "o"
	ModeT string `json:"mode_raw"` // mode asli (permission bits)
}

// FS abstraksi filesystem volume server. Root = direktori volume server
// (dipasang ke /data container). Semua path user RELATIF terhadap root.
type FS interface {
	// List isi direktori path.
	List(path string) ([]Entry, error)
	// Read buka file untuk streaming (closer ditutup caller).
	Read(path string) (io.ReadCloser, error)
	// Write buat/timpa file dari streaming reader (atomic tmp+rename).
	Write(path string, size int64, r io.Reader) (int64, error)
	// Mkdir buat direktori (rekursif).
	Mkdir(path string) error
	// Delete hapus file ATAU direktori rekursif.
	Delete(path string) error
	// Rename pindah/nama baru (dalam satu volume).
	Rename(from, to string) error
	// ExtractZip ekstrak arsip zip src ke dir dst. Return total byte terulis.
	ExtractZip(src, dst string) (int64, error)
	// DiskUsage pemakaian disk nyata (untuk background scan, throttled).
	DiskUsage() (int64, error)
	// Chown ubah kepemilikan path (UID/GID container, SPEC §7.4).
	Chown(path string, uid, gid int) error
}
