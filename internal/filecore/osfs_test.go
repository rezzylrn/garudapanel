// Test filecore: path traversal + symlink escape + quota + chown + memFS.
// KRITIS (SPEC §7.5/§9): ../../../etc/passwd dan symlink keluar volume
// HARUS ditolak dengan ErrOutsideVolume — di osFS asli dan di MemFS.
package filecore

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ---- path traversal lexically (cleanPath) ----

func TestCleanPathRejectsTraversal(t *testing.T) {
	for _, c := range []string{
		"../../../etc/passwd",
		"../../../../etc/shadow",
		"..",
		"../",
		"/../../etc/passwd",
		"dir/../../../etc/passwd",
		"a/../../b",
	} {
		if _, err := cleanPath(c); err == nil {
			t.Errorf("cleanPath(%q) harus ditolak", c)
		}
	}
	for _, c := range []string{"plugin/config.yml", "var/data", "./server.properties", ""} {
		if _, err := cleanPath(c); err != nil {
			t.Errorf("cleanPath(%q) tidak boleh gagal: %v", c, err)
		}
	}
}

// ---- osFS asli: traversal + symlink escape + CRUD ----

func newTestOS(t *testing.T) (*osFS, FS) {
	t.Helper()
	f, err := NewOS(t.TempDir())
	if err != nil {
		t.Fatalf("NewOS: %v", err)
	}
	return f.(*osFS), f
}

func mustFailOutside(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: harus ditolak (ErrOutsideVolume)", what)
	}
	if !errors.Is(err, ErrOutsideVolume) {
		t.Fatalf("%s: err = %v, want ErrOutsideVolume", what, err)
	}
}

func TestOSFSTraversalReadWrite(t *testing.T) {
	_, f := newTestOS(t)
	_, err := f.Write("../../../etc/passwd", 0, strings.NewReader("root:x:0:0"))
	mustFailOutside(t, err, "write traversal")
	_, err = f.Read("../../../../etc/passwd")
	mustFailOutside(t, err, "read traversal")
	_, err = f.List("..")
	mustFailOutside(t, err, "list traversal")
	_, err = f.List("a/../../etc")
	mustFailOutside(t, err, "list nested traversal")
}

func TestOSFSSymlinkEscape(t *testing.T) {
	o, f := newTestOS(t)
	// file di luar volume
	outside := filepath.Join(filepath.Dir(o.Root), "outside-secret.txt")
	if err := os.WriteFile(outside, []byte("rahasia"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(outside) })

	// symlink dalam volume → keluar
	if err := os.Symlink(outside, filepath.Join(o.Root, "leak.txt")); err != nil {
		t.Fatal(err)
	}
	// read lewat symlink HARUS ditolak
	if _, err := f.Read("leak.txt"); err == nil {
		t.Fatal("read symlink keluar volume harus ditolak")
	}
	// write lewat symlink keluar juga ditolak
	if _, err := f.Write("leak.txt", 0, strings.NewReader("x")); err == nil {
		t.Fatal("write lewat symlink keluar harus ditolak")
	}
	// symlink di subdir menunjuk keluar
	if err := f.Mkdir("plugins"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc", filepath.Join(o.Root, "plugins", "evil")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Read("plugins/evil/passwd"); err == nil {
		t.Fatal("read via symlink dir keluar harus ditolak")
	}
	if _, err := f.List("plugins/evil"); err == nil {
		t.Fatal("list via symlink keluar harus ditolak")
	}
}

func TestOSFSSymlinkInsideOK(t *testing.T) {
	o, f := newTestOS(t)
	if _, err := f.Write("real.txt", 0, strings.NewReader("isi")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Symlink(filepath.Join(o.Root, "real.txt"), filepath.Join(o.Root, "alias.txt")); err != nil {
		t.Fatal(err)
	}
	r, err := f.Read("alias.txt")
	if err != nil {
		t.Fatalf("read symlink dalam volume harus boleh: %v", err)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "isi" {
		t.Fatalf("content = %q", b)
	}
}

func TestOSFSListReadWriteDeleteRename(t *testing.T) {
	_, f := newTestOS(t)
	if _, err := f.Write("configs/server.properties", 0, strings.NewReader("motd=halo")); err != nil {
		t.Fatalf("write: %v", err)
	}
	ents, err := f.List("configs")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ents) != 1 || ents[0].Name != "server.properties" || ents[0].Mode != "f" {
		t.Fatalf("list = %+v", ents)
	}
	rc, err := f.Read("configs/server.properties")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "motd=halo" {
		t.Fatalf("content = %q", b)
	}
	if err := f.Mkdir("world"); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := f.Rename("configs/server.properties", "world/server.properties"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := f.Read("world/server.properties"); err != nil {
		t.Fatalf("read after rename: %v", err)
	}
	if err := f.Delete("configs"); err != nil {
		t.Fatalf("delete dir rekursif: %v", err)
	}
	ents, _ = f.List(".")
	if len(ents) != 1 || ents[0].Name != "world" {
		t.Fatalf("list root = %+v, want hanya 'world'", ents)
	}
}

func TestOSFSZipSlipRejected(t *testing.T) {
	o, f := newTestOS(t)
	// zip dengan entri traversal + symlink-ish path
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../../../../etc/garuda-evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("evil"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write("evil.zip", 0, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	_, err = f.ExtractZip("evil.zip", ".")
	mustFailOutside(t, err, "zip-slip harus ditolak")
	_ = o
}

func TestOSFSZipExtractOK(t *testing.T) {
	_, f := newTestOS(t)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"plugins/a.txt", "data/sub/b.txt"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("isi-" + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write("pack.zip", 0, bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	n, err := f.ExtractZip("pack.zip", ".")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if n == 0 {
		t.Fatal("total ekstrak 0")
	}
	if rc, err := f.Read("data/sub/b.txt"); err != nil {
		t.Fatalf("read hasil extract: %v", err)
	} else {
		rc.Close()
	}
}

func TestOSFSChown(t *testing.T) {
	o, f := newTestOS(t)
	if _, err := f.Write("owned.txt", 0, strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if err := f.Chown("owned.txt", 1000, 1000); err != nil {
		t.Fatalf("chown: %v", err)
	}
	// verifikasi kepemilikan via syscall.Stat_t (Linux)
	if fi, err := os.Stat(filepath.Join(o.Root, "owned.txt")); err == nil {
		if st, ok := fi.Sys().(*syscall.Stat_t); ok && st.Uid != 1000 {
			t.Fatalf("uid = %d, want 1000", st.Uid)
		}
	}
}

// ---- quota (event-based + scan throttled) ----

func TestQuotaEventAccounting(t *testing.T) {
	q := NewQuota(1000)
	if !q.CanFit(999) {
		t.Fatal("999 harus muat dalam 1000")
	}
	if q.CanFit(1001) {
		t.Fatal("1001 tidak boleh muat dalam 1000")
	}
	q.Charge(600)
	if q.Used() != 600 {
		t.Fatalf("used = %d", q.Used())
	}
	q.Charge(-200)
	if q.Used() != 400 {
		t.Fatalf("used = %d", q.Used())
	}
	// tanpa limit: selalu muat
	unlim := NewQuota(0)
	if !unlim.CanFit(1 << 40) {
		t.Fatal("limit 0 harus unlimited")
	}
}

func TestQuotaScanThrottled(t *testing.T) {
	q := NewQuota(1000)
	q.SetScanInterval(50 * time.Millisecond)
	if !q.ShouldScan() {
		t.Fatal("scan pertama harus diizinkan")
	}
	q.MarkScanned()
	if q.ShouldScan() {
		t.Fatal("scan kedua harus di-throttle")
	}
	time.Sleep(60 * time.Millisecond)
	if !q.ShouldScan() {
		t.Fatal("setelah interval, scan harus boleh")
	}
}

func TestQuotaScanIntervalDefault5Minutes(t *testing.T) {
	q := NewQuota(0)
	if got := q.MinScanInterval(); got != 5*time.Minute {
		t.Fatalf("default scan interval = %v, want 5m", got)
	}
}

// ---- FileOps: akuntansi event ----

func TestFileOpsChargesAccounting(t *testing.T) {
	m := NewMem()
	q := NewQuota(100)
	o := NewFileOps("uuid-1", m, q, 1000, 1000)
	n, err := o.Write("a.txt", strings.NewReader("hello world"), 0)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 11 || q.Used() != 11 {
		t.Fatalf("n=%d used=%d, want 11/11", n, q.Used())
	}
	// write menimpa: delta benar (lama 11 diganti 2)
	if _, err := o.Write("a.txt", strings.NewReader("hi"), 0); err != nil {
		t.Fatal(err)
	}
	if q.Used() != 2 {
		t.Fatalf("used after overwrite = %d, want 2", q.Used())
	}
	// delete: kurangi
	if err := o.Delete("a.txt"); err != nil {
		t.Fatal(err)
	}
	if q.Used() != 0 {
		t.Fatalf("used after delete = %d, want 0", q.Used())
	}
}

func TestFileOpsQuotaRejectsWrite(t *testing.T) {
	m := NewMem()
	q := NewQuota(10)
	o := NewFileOps("uuid-1", m, q, 1000, 1000)
	// size diketahui, melebihi limit → ditolak sebelum tulis
	_, err := o.Write("big.txt", strings.NewReader(strings.Repeat("x", 100)), 100)
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
	// size tidak diketahui: boleh jalan (akuntansi tetap tercatat)
	n, err := o.Write("big2.txt", strings.NewReader(strings.Repeat("x", 100)), 0)
	if err != nil {
		t.Fatalf("write tanpa size tidak boleh gagal: %v", err)
	}
	if q.Used() != int64(n) {
		t.Fatalf("used = %d, want %d", q.Used(), n)
	}
}

func TestFileOpsExtractChargesQuota(t *testing.T) {
	m := NewMem()
	q := NewQuota(10 * 1024)
	o := NewFileOps("u", m, q, 1000, 1000)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("f.txt")
	w.Write([]byte(strings.Repeat("z", 5000)))
	zw.Close()
	if _, err := o.Write("p.zip", bytes.NewReader(buf.Bytes()), 0); err != nil {
		t.Fatal(err)
	}
	n, err := o.ExtractZip("p.zip", ".")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// used = zip (tetap ada) + hasil ekstrak
	if q.Used() != n+int64(buf.Len()) {
		t.Fatalf("used = %d, want %d (zip %d + ekstrak %d)", q.Used(), n+int64(buf.Len()), buf.Len(), n)
	}
}

func TestFileOpsBackgroundScanThrottled(t *testing.T) {
	m := NewMem()
	q := NewQuota(1000)
	o := NewFileOps("u", m, q, 1000, 1000)
	o.ScanFn = m.DiskUsage
	// scan ke-1: jalan
	if _, ran := o.ScanBackground(); !ran {
		t.Fatal("scan pertama harus jalan")
	}
	// langsung scan lagi: harus di-throttle (JANGAN walk tiap detik)
	if _, ran := o.ScanBackground(); ran {
		t.Fatal("scan kedua harus di-throttle (1x/5 menit)")
	}
}

// ---- owner (UID/GID container) ----

func TestOwnerFor(t *testing.T) {
	if uid, gid, ok := ParseImageUser(""); ok {
		t.Fatalf("empty user ok=%d,%d,%v", uid, gid, ok)
	}
	if uid, gid, ok := ParseImageUser("root"); ok {
		t.Fatalf("non-numeric ok=%d,%d,%v", uid, gid, ok)
	}
	if uid, gid, ok := ParseImageUser("1000"); !ok || uid != 1000 || gid != 1000 {
		t.Fatalf("1000 → %d,%d,%v", uid, gid, ok)
	}
	if uid, gid, ok := ParseImageUser("1000:2000"); !ok || uid != 1000 || gid != 2000 {
		t.Fatalf("1000:2000 → %d,%d,%v", uid, gid, ok)
	}
	uid, gid := OwnerFor("srv", "") // fallback
	if uid != DefaultUID || gid != DefaultGID {
		t.Fatalf("fallback = %d:%d, want %d:%d", uid, gid, DefaultUID, DefaultGID)
	}
	uid, gid = OwnerFor("srv", "33:33")
	if uid != 33 || gid != 33 {
		t.Fatalf("33:33 → %d:%d", uid, gid)
	}
}

// ---- MemFS dasar (dipakai test daemon file manager) ----

func TestMemFSBasic(t *testing.T) {
	m := NewMem()
	if err := m.Put("dir/a.txt", []byte("AAA")); err != nil {
		t.Fatal(err)
	}
	if !m.Has("dir/a.txt") {
		t.Fatal("Put harus tersimpan")
	}
	ents, err := m.List("dir")
	if err != nil || len(ents) != 1 || ents[0].Name != "a.txt" {
		t.Fatalf("list = %+v err=%v", ents, err)
	}
	if b, ok := m.Get("dir/a.txt"); !ok || string(b) != "AAA" {
		t.Fatalf("get = %q", b)
	}
	if err := m.Mkdir("dir2"); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename("dir/a.txt", "dir/b.txt"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !m.Has("dir/b.txt") || m.Has("dir/a.txt") {
		t.Fatal("rename tidak benar")
	}
	if err := m.Delete("dir"); err != nil {
		t.Fatalf("delete rekursif: %v", err)
	}
	if m.Has("dir/b.txt") {
		t.Fatal("subtree harus ikut terhapus")
	}
}

func TestMemFSTraversalRejected(t *testing.T) {
	m := NewMem()
	_, err := m.Read("../../../etc/passwd")
	if !errors.Is(err, ErrOutsideVolume) {
		t.Fatalf("read traversal = %v, want ErrOutsideVolume", err)
	}
	_, err = m.List("..")
	if !errors.Is(err, ErrOutsideVolume) {
		t.Fatalf("list traversal = %v", err)
	}
}

func TestMemFSSymlinkEscapeRejected(t *testing.T) {
	m := NewMem()
	_ = m.Symlink("../../../etc/passwd", "leak")
	if _, err := m.Read("leak"); !errors.Is(err, ErrOutsideVolume) {
		t.Fatalf("read symlink keluar = %v, want ErrOutsideVolume", err)
	}
}
