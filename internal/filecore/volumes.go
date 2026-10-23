// fsmap: registry volume FS + quota per server di daemon.
// FS asli (osFS) dan fake (MemFS untuk test) diakses lewat interface FS
// yang sama (pola ContainerRuntime, milestone 2).
package filecore

import (
	"errors"
	"sync"
)

// FSFactory: buat FS untuk volume root tertentu (daemon pakai NewOS;
// test pakai factory MemFS).
type FSFactory func(root string) (FS, error)

// VolumeRegistry: FS + Quota per server uuid.
type Volumes struct {
	mu      sync.Mutex
	fs      map[string]FS
	quota   map[string]*MemQuota
	limits  map[string]int64
	factory FSFactory
}

// NewVolumes registry dengan factory (nil = osFS asli).
func NewVolumes(f FSFactory) *Volumes {
	if f == nil {
		f = func(root string) (FS, error) { return NewOS(root) }
	}
	return &Volumes{
		fs:      map[string]FS{},
		quota:   map[string]*MemQuota{},
		limits:  map[string]int64{},
		factory: f,
	}
}

// For ambil/buat FS + quota server (diskMB dari servers.disk_mb).
func (v *Volumes) For(uuid string, dataDir string, diskMB int64) (FS, *MemQuota, error) {
	if uuid == "" {
		return nil, nil, errors.New("empty uuid")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if f, ok := v.fs[uuid]; ok {
		return f, v.quota[uuid], nil
	}
	if dataDir == "" {
		return nil, nil, errors.New("filecore: empty data dir")
	}
	f, err := v.factory(dataDir + "/" + uuid)
	if err != nil {
		return nil, nil, err
	}
	v.fs[uuid] = f
	q := NewQuota(diskMB * 1024 * 1024)
	v.quota[uuid] = q
	v.limits[uuid] = diskMB
	return f, q, nil
}

// QuotaOf ambil quota uuid (nil bila belum ada).
func (v *Volumes) Quota(uuid string) *MemQuota {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.quota[uuid]
}

// SetDiskLimit update limit saat admin ubah disk_mb server.
func (v *Volumes) SetDiskLimit(uuid string, diskMB int64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if q, ok := v.quota[uuid]; ok {
		q.SetLimit(diskMB * 1024 * 1024)
	}
	v.limits[uuid] = diskMB
}
