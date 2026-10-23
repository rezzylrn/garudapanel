// Quota disk server: akuntansi berbasis event + background scan throttled
// (SPEC §7.3). JANGAN filepath.Walk tiap detik — pemakaian di-track saat
// write/delete/upload/extract; scan penuh hanya maksimal 1x per 5 menit.
package filecore

import (
	"sync"
	"time"
)

// MemQuota akuntansi disk in-memory satu server.
type MemQuota struct {
	mu              sync.Mutex
	used            int64
	limit           int64
	lastScan        time.Time
	minScanInterval time.Duration
}

// NewQuota: limit byte (0 = tanpa batas); scan di-throttle minInterval
// (default 5 menit per SPEC).
func NewQuota(limit int64) *MemQuota {
	return &MemQuota{limit: limit, minScanInterval: 5 * time.Minute}
}

// Charge tambah/kurangi pemakaian tercatat.
func (q *MemQuota) Charge(delta int64) {
	if q == nil {
		return
	}
	q.mu.Lock()
	q.used += delta
	if q.used < 0 {
		q.used = 0
	}
	q.mu.Unlock()
}

// CanFit cek apakah menambah n byte masih dalam limit (limit 0 = unlimited).
func (q *MemQuota) CanFit(n int64) bool {
	if q == nil {
		return true
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.limit == 0 {
		return true
	}
	return q.used+n <= q.limit
}

func (q *MemQuota) Used() int64 {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.used
}

func (q *MemQuota) Limit() int64 {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.limit
}

// SetLimit ubah batas (mis. admin ubah disk_mb server).
func (q *MemQuota) SetLimit(l int64) {
	q.mu.Lock()
	q.limit = l
	q.mu.Unlock()
}

// SetUsed koreksi pemakaian (hasil background scan).
func (q *MemQuota) SetUsed(v int64) {
	q.mu.Lock()
	q.used = v
	q.lastScan = time.Now()
	q.mu.Unlock()
}

// ShouldScan: true bila scan background boleh jalan (>= interval sejak
// scan terakhir). Throttle 1x/5 menit (SPEC §7.3).
func (q *MemQuota) ShouldScan() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return time.Since(q.lastScan) >= q.minScanInterval
}

// MarkScanned catat waktu scan terakhir.
func (q *MemQuota) MarkScanned() {
	q.mu.Lock()
	q.lastScan = time.Now()
	q.mu.Unlock()
}

// SetScanInterval override untuk test (default 5 menit).
func (q *MemQuota) SetScanInterval(d time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.minScanInterval = d
}

// MinScanInterval throttle saat ini (default 5 menit per SPEC).
func (q *MemQuota) MinScanInterval() time.Duration {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.minScanInterval
}
