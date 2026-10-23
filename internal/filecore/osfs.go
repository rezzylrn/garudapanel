// osFS: implementasi FS asli di atas package os. Keamanan path (SPEC §7.5):
// Go 1.22 toolchain ini belum punya os.Root, jadi dipakai validasi symlink
// KETAT dua lapis:
//  1. path user dinormalisasi lexically — ".." atau absolute path ditolak;
//  2. sebelum operasi, seluruh komponen jalur dievaluasi via Lstat/EvalSymlinks
//     dan hasil resolusi WAJIB tetap di dalam root (symlink yang menunjuk
//     keluar volume → ErrOutsideVolume, ditolak, bukan diikuti).
//
// Catatan: saat toolchain project naik ke Go 1.24+, ganti internals ke os.Root
// (OpenRoot/OpenInRoot) tanpa mengubah interface FS.
package filecore

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// osFS filesystem volume server pada direktori Root.
type osFS struct {
	Root string
}

// NewOS membuat FS asli dengan root direktori volume (dibuat bila belum ada).
func NewOS(root string) (FS, error) {
	if root == "" {
		return nil, errors.New("filecore: empty root")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &osFS{Root: abs}, nil
}

// cleanPath: validasi lexical path user (relatif root).
// Kebijakan KETAT: komponen ".." DILARANG di posisi mana pun (bukan di-resolve),
// path absolut juga ditolak. Return path bersih internal ("a/b") atau error.
func cleanPath(p string) (string, error) {
	if p == "" || p == "/" || p == "." {
		return "", nil
	}
	if strings.Contains(p, "\x00") {
		return "", errors.New("invalid path")
	}
	p = strings.TrimPrefix(filepath.ToSlash(p), "/")
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return "", ErrOutsideVolume // traversal eksplisit: tolak, jangan dinormalisasi
		}
	}
	clean := path.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, "../") || clean == "." {
		return "", ErrOutsideVolume
	}
	return clean, nil
}

// full: path user → path absolut dalam volume, TANPA mengikuti symlink.
func (f *osFS) full(p string) (string, error) {
	c, err := cleanPath(p)
	if err != nil {
		return "", err
	}
	return filepath.Join(f.Root, filepath.FromSlash(c)), nil
}

// withinRoot: pastikan path absolut (hasil resolve symlink) tetap di root.
func (f *osFS) withinRoot(abs string) (string, error) {
	rp, err := filepath.EvalSymlinks(f.Root)
	if err != nil {
		return "", err
	}
	if abs == rp {
		return abs, nil
	}
	rel, err := filepath.Rel(rp, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrOutsideVolume
	}
	return abs, nil
}

// resolve: path user → path absolut yang SUDAH terverifikasi tidak keluar
// volume setelah symlink di-resolve (lapisan 2 validasi §7.5).
// optionalParent=true: bila file/parent belum ada, resolve bagian yang ada.
func (f *osFS) resolve(p string) (string, error) {
	full, err := f.full(p)
	if err != nil {
		return "", err
	}
	// resolve seluruh jalur yang ADA (bagian belum ada diabaikan —
	// keberadaannya dicek oleh operasi masing-masing).
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		// resolve parent TERDALAM yang ada (naik sampai root; parent bisa
		// berantai belum-exist: a/b/c.txt dengan a/ dan b/ belum dibuat).
		rp, remaining, rerr := f.evalDeepestExisting(full)
		if rerr != nil {
			return "", rerr
		}
		resolved = filepath.Join(rp, remaining)
	}
	return f.withinRoot(resolved)
}

// evalDeepestExisting: resolve symlink bagian jalur yang ADA.
// Return (path resolved terdalam, sisa komponen yang belum ada).
func (f *osFS) evalDeepestExisting(full string) (string, string, error) {
	// kumpulkan bagian yang belum ada dari ekor
	tail := ""
	cur := full
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		}
		if cur == f.Root || cur == filepath.Dir(cur) {
			return "", "", ErrOutsideVolume // sampai root pun tidak ada — mustahil
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		cur = filepath.Dir(cur)
	}
	rp, err := filepath.EvalSymlinks(cur)
	if err != nil {
		return "", "", ErrOutsideVolume
	}
	if _, err := f.withinRoot(rp); err != nil {
		return "", "", err
	}
	return rp, tail, nil
}

// resolveLstat: seperti resolve, tapi komponen TERAKHIR tidak di-follow
// (dipakai operasi yang tidak boleh mengikuti symlink akhir: list/read dir).
func (f *osFS) resolveNoFollowLast(p string) (string, error) {
	full, err := f.full(p)
	if err != nil {
		return "", err
	}
	if fi, err := os.Lstat(full); err == nil && fi.Mode()&os.ModeSymlink == 0 {
		// komponen akhir bukan symlink: cukup validasi parent
		rp, perr := filepath.EvalSymlinks(filepath.Dir(full))
		if perr != nil {
			return "", ErrOutsideVolume
		}
		return f.withinRoot(filepath.Join(rp, filepath.Base(full)))
	}
	// komponen akhir symlink atau belum ada → resolve penuh (menolak escape)
	return f.resolve(p)
}

func entryMode(d fs.DirEntry) (string, string) {
	switch {
	case d.IsDir():
		return "d", "d"
	case d.Type()&fs.ModeSymlink != 0:
		return "s", "s"
	case d.Type().IsRegular():
		return "f", fmt.Sprintf("%04o", d.Type().Perm())
	default:
		return "o", fmt.Sprintf("%04o", uint32(d.Type()))
	}
}

func (f *osFS) List(p string) ([]Entry, error) {
	dir, err := f.resolveNoFollowLast(p)
	if err != nil {
		return nil, err
	}
	d, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(names))
	for _, n := range names {
		fi, err := os.Lstat(filepath.Join(dir, n))
		if err != nil {
			continue // file hilang saat listing — lewati
		}
		e := Entry{Name: n, Size: fi.Size()}
		switch {
		case fi.IsDir():
			e.Mode = "d"
			e.ModeT = fmt.Sprintf("%04o", fi.Mode().Perm())
		case fi.Mode()&os.ModeSymlink != 0:
			e.Mode = "s"
			e.ModeT = "s"
		case fi.Mode().IsRegular():
			e.Mode = "f"
			e.ModeT = fmt.Sprintf("%04o", fi.Mode().Perm())
		default:
			e.Mode = "o"
			e.ModeT = fmt.Sprintf("%04o", uint32(fi.Mode()))
		}
		out = append(out, e)
	}
	return out, nil
}

func (f *osFS) Read(p string) (io.ReadCloser, error) {
	rp, err := f.resolve(p)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(rp) // follow ok — resolve sudah menjaga di dalam volume
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, errors.New("is a directory")
	}
	return os.Open(rp)
}

// FileInfo minimal lintas implementasi FS (osFS & MemFS) — dipakai FileOps
// untuk akuntansi delta (ukuran lama saat overwrite/delete).
type FileInfo interface {
	Size() int64
	IsDir() bool
}

func (m *MemFS) stat(p string) (FileInfo, error) {
	n, err := m.resolveTarget(p)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.files[n]
	if !ok {
		return nil, errNotFound
	}
	return &memStat{size: int64(len(e.data)), isDir: e.isDir}, nil
}

type memStat struct {
	size  int64
	isDir bool
}

func (s *memStat) Size() int64 { return s.size }
func (s *memStat) IsDir() bool { return s.isDir }

// Stat info satu path (dipakai FileOps untuk akuntansi delta).
// Return ukuran file/dir; dir dihitung 0 (scan background mengoreksi).
func Stat(f FS, p string) (FileInfo, error) {
	if o, ok := f.(*osFS); ok {
		rp, err := o.resolveNoFollowLast(p)
		if err != nil {
			return nil, err
		}
		return os.Lstat(rp)
	}
	if m, ok := f.(*MemFS); ok {
		return m.stat(p)
	}
	return nil, errors.New("stat: unsupported FS")
}

func (f *osFS) Write(p string, _ int64, r io.Reader) (int64, error) {
	rp, err := f.resolve(p)
	if err != nil {
		return 0, err
	}
	if fi, err := os.Lstat(rp); err == nil && fi.IsDir() {
		return 0, errors.New("cannot write: is a directory")
	}
	// parent belum ada → buat (mkdirAll; chown menyusul via FileOps)
	if err := os.MkdirAll(filepath.Dir(rp), 0o755); err != nil {
		return 0, err
	}
	tmp := rp + ".garuda-tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	n, cpErr := io.Copy(out, r)
	cErr := out.Close()
	if cpErr != nil {
		_ = os.Remove(tmp)
		return n, cpErr
	}
	if cErr != nil {
		return n, cErr
	}
	if err := os.Rename(tmp, rp); err != nil {
		_ = os.Remove(tmp)
		return n, err
	}
	return n, nil
}

func (f *osFS) Mkdir(p string) error {
	rp, err := f.resolve(p)
	if err != nil {
		return err
	}
	return os.MkdirAll(rp, 0o755)
}

// Delete hapus file/dir. Symlink dihapus SEBAGAI LINK (bukan targetnya) —
// Lstat dulu; rmdir/unlink langsung. Dir dihapus rekursif via OpenRoot-style
// manual walk pada path yang SUDAH tervalidasi resolve.
func (f *osFS) Delete(p string) error {
	rp, err := f.resolveNoFollowLast(p)
	if err != nil {
		return err
	}
	fi, err := os.Lstat(rp)
	if err != nil {
		return err
	}
	if fi.IsDir() && fi.Mode()&os.ModeSymlink == 0 {
		return f.removeDir(rp)
	}
	return os.Remove(rp)
}

// removeDir hapus direktori rekursif. rp SUDAH tervalidasi dalam volume;
// tiap anak di-validate ulang (dir anak bisa berisi symlink keluar, yang
// dihapus sebagai link — bukan isi targetnya).
func (f *osFS) removeDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	names, err := d.Readdirnames(-1)
	d.Close()
	if err != nil {
		return err
	}
	for _, n := range names {
		child := filepath.Join(dir, n)
		fi, err := os.Lstat(child)
		if err != nil {
			continue
		}
		if fi.IsDir() && fi.Mode()&os.ModeSymlink == 0 {
			if err := f.removeDir(child); err != nil {
				return err
			}
		} else {
			if err := os.Remove(child); err != nil {
				return err
			}
		}
	}
	return os.Remove(dir)
}

func (f *osFS) Rename(from, to string) error {
	src, err := f.resolveNoFollowLast(from)
	if err != nil {
		return err
	}
	dst, err := f.resolve(to)
	if err != nil {
		return err
	}
	if dst == src {
		return errors.New("rename: same path")
	}
	return os.Rename(src, dst)
}

func (f *osFS) ExtractZip(src, dst string) (int64, error) {
	s, err := f.resolve(src)
	if err != nil {
		return 0, err
	}
	d, err := f.resolve(dst)
	if err != nil {
		return 0, err
	}
	zr, err := zip.OpenReader(s)
	if err != nil {
		return 0, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()
	var total int64
	for _, zf := range zr.File {
		name, err := cleanPath(zf.Name)
		if err != nil {
			// entri zip menunjuk keluar (zip slip) → tolak seluruh ekstraksi
			return total, fmt.Errorf("zip entry %q: %w", zf.Name, ErrOutsideVolume)
		}
		target := filepath.Join(d, filepath.FromSlash(name))
		// anti zip-slip: target resolve harus tetap di dalam dst
		rp, perr := filepath.EvalSymlinks(filepath.Dir(target))
		if perr != nil {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return total, err
			}
			rp, perr = filepath.EvalSymlinks(filepath.Dir(target))
			if perr != nil {
				return total, ErrOutsideVolume
			}
		}
		if _, err := f.withinRoot(rp); err != nil {
			return total, err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return total, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return total, err
		}
		rc, err := zf.Open()
		if err != nil {
			return total, err
		}
		n, err := writeFileGuard(target, rc, zf.FileInfo().Size())
		rc.Close()
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// writeFileGuard: streaming write ke path final (sudah tervalidasi caller).
func writeFileGuard(path string, r io.Reader, size int64) (int64, error) {
	tmp := path + ".garuda-tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	n, cpErr := io.Copy(out, r)
	cErr := out.Close()
	if cpErr != nil {
		_ = os.Remove(tmp)
		return n, cpErr
	}
	if cErr != nil {
		_ = os.Remove(tmp)
		return n, cErr
	}
	return n, os.Rename(tmp, path)
}

func (f *osFS) DiskUsage() (int64, error) {
	var total int64
	err := filepath.WalkDir(f.Root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, ierr := d.Info()
			if ierr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total, err
}

func (f *osFS) Chown(p string, uid, gid int) error {
	rp, err := f.resolve(p)
	if err != nil {
		return err
	}
	if uid < 0 && gid < 0 {
		return nil // tidak ada permintaan kepemilikan
	}
	return os.Chown(rp, uid, gid)
}
