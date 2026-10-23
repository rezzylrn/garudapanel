// Package daemon: ring buffer log terakhir per container.
// Kapasitas 100KB per server (SPEC §4): memori daemon tetap kecil
// berapapun jumlah server, output console diputar (overwrite) saat penuh.
package daemon

import (
	"sync"
)

// RingBuffer buffer melingkar byte dengan batas ukuran.
// Thread-safe (dipakai dari goroutine baca console & goroutine replay).
type RingBuffer struct {
	mu     sync.Mutex
	buf    []byte
	size   int // kapasitas maksimum
	start  int // index byte pertama yang valid
	length int // jumlah byte valid
}

// NewRingBuffer alokasi buffer ukuran size byte.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 1
	}
	return &RingBuffer{buf: make([]byte, size), size: size}
}

// Write menambah data ke buffer; kalau lebih besar dari kapasitas,
// hanya N byte terakhir yang tersisa (bufor diputar).
func (r *RingBuffer) Write(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// terlalu besar: ambil ekor saja
	if len(p) >= r.size {
		copy(r.buf, p[len(p)-r.size:])
		r.start = 0
		r.length = r.size
		return
	}
	// perlu rotasi: geser konten lama ke depan biar tetap urut
	if r.length+len(p) > r.size {
		overflow := r.length + len(p) - r.size
		r.start = (r.start + overflow) % r.size
		r.length -= overflow
	}
	end := (r.start + r.length) % r.size
	// tulis dengan maksimal dua segmen (wrap)
	n1 := copy(r.buf[end:], p)
	if n1 < len(p) {
		copy(r.buf, p[n1:])
	}
	r.length += len(p)
}

// Bytes mengembalikan snapshot isi buffer dalam urutan tulis.
// Copy aman dari mutasi berikutnya.
func (r *RingBuffer) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, r.length)
	n1 := copy(out, r.buf[r.start:r.start+r.length])
	if n1 < r.length { // wrap
		copy(out[n1:], r.buf[:r.length-n1])
	}
	return out
}

// Len jumlah byte yang tersimpan.
func (r *RingBuffer) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.length
}
