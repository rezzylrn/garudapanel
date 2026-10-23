// filemgr: layer operasi file di sisi daemon (SPEC §4 rpc, §7.3-7.5).
// Menerima perintah RPC file:<op> dari panel (payload JSON) dan menjalankan
// FileOps terhadap volume server — dengan validasi path (filecore), cek
// quota, dan chown UID/GID container. Read/write/download MURNI streaming
// lewat goroutine pipe (bukan frame JSON): file 500MB tidak masuk RAM.
package daemon

import (
	"errors"
	"fmt"
	"log"

	"garudapanel/internal/filecore"
)

// filePayload perintah RPC file (payload frame {ch:"rpc", type:"file:<op>"}).
type filePayload struct {
	ServerUUID string `json:"server_uuid"`
	Path       string `json:"path"`
	To         string `json:"to"` // rename target / extract dst
}

// fileopsEntry: pembungkus filecore.FileOps + disk usage scan (throttled).
type fileopsEntry struct {
	*filecore.FileOps
	uuid string
}

func newFileopsEntry(uuid string, f filecore.FS, q *filecore.MemQuota, uid, gid int) *fileopsEntry {
	e := &fileopsEntry{FileOps: filecore.NewFileOps(uuid, f, q, uid, gid), uuid: uuid}
	e.ScanFn = f.DiskUsage
	return e
}

// fileOpsFor: FileOps server (volume + quota + owner). FileOwner di spec
// di-set dari image config oleh servers.sync (fallback 1000:1000 di filecore).
func (s *Supervisor) fileOps(uuid string) (*fileopsEntry, error) {
	s.mu.Lock()
	st, ok := s.servers[uuid]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("server %s not registered on this node", uuid)
	}
	if s.cfg.DataDir == "" {
		return nil, errors.New("daemon data dir not configured")
	}
	f, q, err := s.volumes.For(uuid, s.cfg.DataDir, st.spec.DiskMB)
	if err != nil {
		return nil, err
	}
	uid, gid := st.spec.FileOwner()
	return newFileopsEntry(uuid, f, q, uid, gid), nil
}

// handleFileRPC: eksekusi operasi file metadata (list/mkdir/delete/rename/
// extract/usage) via RPC frame {ch:"rpc", type:"file", payload:{op:...}}.
// Operasi streaming (read/write) lewat jalur pipe terpisah (fileio.go).
func (s *Supervisor) handleFileRPC(payload []byte) (any, error) {
	var p struct {
		ServerUUID string `json:"server_uuid"`
		Op         string `json:"op"`
		Path       string `json:"path"`
		To         string `json:"to"`
	}
	if err := unmarshal(payload, &p); err != nil || p.ServerUUID == "" {
		return nil, errors.New("bad file payload")
	}
	fops, err := s.fileOps(p.ServerUUID)
	if err != nil {
		return nil, err
	}
	switch p.Op {
	case "list":
		ents, err := fops.List(p.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"entries": ents}, nil
	case "mkdir":
		if err := fops.Mkdir(p.Path); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "delete":
		if err := fops.Delete(p.Path); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "rename":
		if p.To == "" {
			return nil, errors.New("rename: missing to")
		}
		if err := fops.Rename(p.Path, p.To); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "extract":
		n, err := fops.ExtractZip(p.Path, p.To)
		if err != nil {
			return nil, err
		}
		return map[string]any{"bytes": n}, nil
	case "usage":
		used, limit := fops.UsedDisk()
		// scan background THROTTLED (1x/5 menit, SPEC §7.3) — koreksi drift.
		if real, ran := fops.ScanBackground(); ran {
			log.Printf("fileops %s: background scan → %d bytes", p.ServerUUID, real)
		}
		return map[string]any{"used": used, "limit": limit}, nil
	default:
		return nil, fmt.Errorf("unknown file op %q", p.Op)
	}
}
