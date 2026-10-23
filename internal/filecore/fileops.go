// fileops: jembatan FS ↔ daemon — operasi file dengan cek quota + pencatatan
// delta akuntansi (event-based accounting, SPEC §7.3) + chown UID/GID
// container (SPEC §7.4). Semua operasi menerima path RELATIF volume.
package filecore

import (
	"io"
	"log"
)

// FileOps: operasi file satu server, lengkap dengan quota + owner.
type FileOps struct {
	fs    FS
	quota *MemQuota
	uuid  string
	uid   int
	gid   int
	// ScanFn: fungsi background scan (dipasang daemon: DiskUsage dari FS).
	ScanFn func() (int64, error)
}

// NewFileOps: fs volume + quota + uid/gid container (owner.go).
func NewFileOps(uuid string, f FS, q *MemQuota, uid, gid int) *FileOps {
	return &FileOps{fs: f, quota: q, uuid: uuid, uid: uid, gid: gid}
}

// chownOwned: set kepemilikan hasil operasi ke UID/GID container (§7.4).
// File baru TIDAK boleh root-owned.
func (o *FileOps) chownOwned(p string) {
	if err := o.fs.Chown(p, o.uid, o.gid); err != nil {
		log.Printf("fileops %s: chown %s: %v", o.uuid, p, err)
	}
}

// List isi direktori.
func (o *FileOps) List(p string) ([]Entry, error) {
	return o.fs.List(p)
}

// Read buka file streaming.
func (o *FileOps) Read(p string) (io.ReadCloser, error) {
	f, err := o.fs.Read(p)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Write tulis file streaming (upload/write). Cek quota dulu bila size
// diketahui; delta akuntansi di-charge setelah berhasil. Quota juga
// diverifikasi ulang saat scan background (SetUsed).
func (o *FileOps) Write(p string, r io.Reader, size int64) (int64, error) {
	if size > 0 && !o.quota.CanFit(size) {
		return 0, ErrQuotaExceeded
	}
	// overwrite lama: kurangi dulu pemakaian entry lama (bila ada)
	if st, err := Stat(o.fs, p); err == nil {
		o.quota.Charge(-st.Size())
	}
	n, err := o.fs.Write(p, size, r)
	if err != nil {
		// gagal: kembalikan delta entry lama (masih ada di disk bila overwrite)
		if st, serr := Stat(o.fs, p); serr == nil {
			o.quota.Charge(st.Size())
		}
		return n, err
	}
	o.quota.Charge(n)
	o.chownOwned(p)
	if o.quota.Limit() > 0 && !o.quota.CanFit(0) {
		log.Printf("fileops %s: quota exceeded after write %s (%d bytes)", o.uuid, p, n)
	}
	return n, nil
}

// Mkdir buat direktori.
func (o *FileOps) Mkdir(p string) error {
	if err := o.fs.Mkdir(p); err != nil {
		return err
	}
	o.chownOwned(p)
	return nil
}

// Delete hapus (kurangi akuntansi).
func (o *FileOps) Delete(p string) error {
	if st, err := Stat(o.fs, p); err == nil {
		defer func() {
			o.quota.Charge(-st.Size())
		}()
	}
	return o.fs.Delete(p)
}

// Rename pindah/nama baru (akuntansi tidak berubah — dalam volume sama).
func (o *FileOps) Rename(from, to string) error {
	return o.fs.Rename(from, to)
}

// ExtractZip ekstrak arsip (charge total byte hasil ekstraksi).
func (o *FileOps) ExtractZip(src, dst string) (int64, error) {
	// cek quota kasar: ukuran zip × 4 (rasio kompresi tipikal maks ~4x).
	// Scan background akan mengoreksi drift.
	if st, err := Stat(o.fs, src); err == nil && st.Size()*4 > 0 {
		if !o.quota.CanFit(st.Size() * 4) {
			return 0, ErrQuotaExceeded
		}
	}
	n, err := o.fs.ExtractZip(src, dst)
	if err != nil {
		return n, err
	}
	o.quota.Charge(n)
	return n, nil
}

// UsedDisk pemakaian tercatat + limit.
func (o *FileOps) UsedDisk() (used, limit int64) {
	return o.quota.Used(), o.quota.Limit()
}

// ScanBackground: koreksi pemakaian dari DiskUsage nyata (dipanggil
// background scan THROTTLED — lihat quota.ShouldScan, 1x/5 menit).
func (o *FileOps) ScanBackground() (int64, bool) {
	if !o.quota.ShouldScan() {
		return o.quota.Used(), false
	}
	if o.ScanFn == nil {
		return o.quota.Used(), false
	}
	real, err := o.ScanFn()
	if err != nil {
		log.Printf("fileops %s: background scan: %v", o.uuid, err)
		return o.quota.Used(), false
	}
	o.quota.SetUsed(real)
	return real, true
}
