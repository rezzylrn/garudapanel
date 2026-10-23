// memFS: fake FS in-memory untuk unit test daemon/logika file manager tanpa
// disk asli (pola FakeRuntime). Path traversal tetap divalidasi lexically
// supaya perilaku keamanan sama dengan osFS.
package filecore

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
)

// MemFS fake filesystem: map path → konten/flag dir/symlink.
type MemFS struct {
	mu    sync.Mutex
	files map[string]*memEntry // path bersih → entry
}

type memEntry struct {
	data   []byte
	isDir  bool
	isLink bool
	target string // target symlink (bisa keluar volume — untuk test penolakan)
	uid    int
	gid    int
}

func NewMem() *MemFS {
	return &MemFS{files: map[string]*memEntry{"": {isDir: true}}}
}

// norm: normalisasi path user (relatif root). Aturan sama dengan osFS.
func norm(p string) (string, error) {
	if p == "" || p == "/" || p == "." {
		return "", nil
	}
	if strings.Contains(p, "\x00") {
		return "", ErrOutsideVolume
	}
	p = strings.TrimPrefix(p, "/")
	// lexical clean manual: ".." di level mana pun = traversal
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", ErrOutsideVolume
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/"), nil
}

// resolveTarget: path user → path bersih resolved (symlink diikuti;
// link yang menunjuk keluar volume ditolak dengan ErrOutsideVolume).
func (m *MemFS) resolveTarget(p string) (string, error) {
	n, err := norm(p)
	if err != nil {
		return "", err
	}
	parts := strings.Split(n, "/")
	cur := ""
	for i, part := range parts {
		if part == "" {
			continue
		}
		if cur == "" {
			cur = part
		} else {
			cur = cur + "/" + part
		}
		e, ok := m.files[cur]
		if !ok {
			break
		}
		if e.isLink {
			ln, lerr := norm(e.target)
			if lerr != nil {
				return "", ErrOutsideVolume
			}
			rest := strings.Join(parts[i+1:], "/")
			if rest != "" {
				rest = "/" + rest
			}
			return m.resolveTarget(ln + rest)
		}
	}
	return n, nil
}

func (m *MemFS) parentOf(n string) string {
	parts := strings.Split(n, "/")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

func (m *MemFS) List(p string) ([]Entry, error) {
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
	if !e.isDir {
		return nil, errors.New("not a directory")
	}
	prefix := n + "/"
	seen := map[string]bool{}
	out := []Entry{}
	for k, ent := range m.files {
		if n == "" {
			if k == "" || strings.Contains(k, "/") {
				continue
			}
		} else {
			if !strings.HasPrefix(k, prefix) || k == n {
				continue
			}
			if strings.Contains(strings.TrimPrefix(k, prefix), "/") {
				continue
			}
		}
		name := strings.TrimPrefix(strings.TrimPrefix(k, n), "/")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		mode := "f"
		if ent.isDir {
			mode = "d"
		} else if ent.isLink {
			mode = "s"
		}
		out = append(out, Entry{Name: name, Size: int64(len(ent.data)), Mode: mode, ModeT: "0644"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *MemFS) Read(p string) (io.ReadCloser, error) {
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
	if e.isDir {
		return nil, errors.New("is a directory")
	}
	cp := make([]byte, len(e.data))
	copy(cp, e.data)
	return io.NopCloser(bytes.NewReader(cp)), nil
}

func (m *MemFS) Write(p string, _ int64, r io.Reader) (int64, error) {
	n, err := m.resolveTarget(p)
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.files[n]; ok && e.isDir {
		return 0, errors.New("cannot write: is a directory")
	}
	if dir := m.parentOf(n); dir != "" {
		if de, ok := m.files[dir]; !ok || !de.isDir {
			// parent belum ada → buat rantai parent (semantik mkdirAll,
			// konsisten dengan osFS.Write)
			parts := strings.Split(n, "/")
			for i := 1; i < len(parts); i++ {
				d := strings.Join(parts[:i], "/")
				if _, exists := m.files[d]; !exists {
					m.files[d] = &memEntry{isDir: true}
				}
			}
		}
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	m.files[n] = &memEntry{data: data}
	return int64(len(data)), nil
}

func (m *MemFS) Mkdir(p string) error {
	n, err := m.resolveTarget(p)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.files[n]; ok && e.isDir {
		return nil // idempotent
	}
	if _, ok := m.files[n]; ok {
		return errors.New("file exists")
	}
	m.files[n] = &memEntry{isDir: true}
	return nil
}

func (m *MemFS) Delete(p string) error {
	n, err := m.resolveTarget(p)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[n]; !ok {
		return errNotFound
	}
	prefix := n + "/"
	for k := range m.files {
		if strings.HasPrefix(k, prefix) {
			delete(m.files, k)
		}
	}
	delete(m.files, n)
	return nil
}

func (m *MemFS) Rename(from, to string) error {
	src, err := m.resolveTarget(from)
	if err != nil {
		return err
	}
	dst, err := m.resolveTarget(to)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[src]; !ok {
		return errNotFound
	}
	if src == dst {
		return errors.New("rename: same path")
	}
	// pindah entry + seluruh subtree (prefix src/ → dst/)
	moved := map[string]*memEntry{}
	prefix := src + "/"
	for k, v := range m.files {
		if k == src || strings.HasPrefix(k, prefix) {
			moved[k] = v
		}
	}
	for k, v := range moved {
		nk := dst + strings.TrimPrefix(k, src)
		m.files[nk] = v
		delete(m.files, k)
	}
	return nil
}

func (m *MemFS) ExtractZip(src, dst string) (int64, error) {
	n, err := m.resolveTarget(src)
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	e, ok := m.files[n]
	if !ok {
		m.mu.Unlock()
		return 0, errNotFound
	}
	data := e.data
	m.mu.Unlock()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, err
	}
	var total int64
	for _, zf := range zr.File {
		name, err := norm(zf.Name)
		if err != nil {
			return total, ErrOutsideVolume
		}
		if zf.FileInfo().IsDir() {
			m.mu.Lock()
			m.files[name] = &memEntry{isDir: true}
			m.mu.Unlock()
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return total, err
		}
		var b bytes.Buffer
		n2, _ := io.Copy(&b, rc)
		rc.Close()
		m.mu.Lock()
		m.files[name] = &memEntry{data: b.Bytes()}
		m.mu.Unlock()
		total += n2
	}
	return total, nil
}

func (m *MemFS) DiskUsage() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, e := range m.files {
		if !e.isDir {
			total += int64(len(e.data))
		}
	}
	return total, nil
}

func (m *MemFS) Chown(p string, uid, gid int) error {
	n, err := m.resolveTarget(p)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.files[n]; ok {
		e.uid, e.gid = uid, gid
	}
	return nil
}

// ---- helper test/setup ----

// Put tulis file langsung (setup test).
func (m *MemFS) Put(p string, data []byte) error {
	n, err := norm(p)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	parts := strings.Split(n, "/")
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		if _, ok := m.files[dir]; !ok {
			m.files[dir] = &memEntry{isDir: true}
		}
	}
	m.files[n] = &memEntry{data: data}
	return nil
}

// Symlink buat link (test escape).
func (m *MemFS) Symlink(target, link string) error {
	n, err := norm(link)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[n] = &memEntry{isLink: true, target: target}
	return nil
}

// MkdirFake buat dir (setup test).
func (m *MemFS) MkdirFake(p string) {
	n, _ := norm(p)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[n] = &memEntry{isDir: true}
}

// Has cek path ada.
func (m *MemFS) Has(p string) bool {
	n, err := norm(p)
	if err != nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.files[n]
	return ok
}

// Get ambil isi file (assert test).
func (m *MemFS) Get(p string) ([]byte, bool) {
	n, err := m.resolveTarget(p)
	if err != nil {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.files[n]
	if !ok {
		return nil, false
	}
	return e.data, true
}

var errNotFound = errors.New("file not found")
