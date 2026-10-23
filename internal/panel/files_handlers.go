// files_handlers.go: endpoint REST file manager (owner/admin — SPEC §10).
// Upload/download MURNI streaming via chunk (32KB) — tanpa buffer penuh RAM.
package panel

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
	"garudapanel/internal/daemonhub"
)

const (
	defaultMaxUploadBytes = 100 * 1024 * 1024 // 100MB default upload limit
	fileChunkSize         = 32 * 1024         // 32KB per chunk (sama dgn daemon)
	fileRPCTimeout        = 30 * time.Second
	fileReadTimeout       = 10 * time.Minute // download besar
	fileWriteTimeout      = 60 * time.Second
)

// cleanRelPath: validasi path user di panel (lapisan 1; daemon validasi ulang).
// ".." di posisi mana pun ditolak; absolute path dinormalisasi.
func cleanRelPath(p string) (string, error) {
	if p == "" || p == "/" || p == "." {
		return "", nil
	}
	if strings.Contains(p, "\x00") {
		return "", errors.New("invalid path")
	}
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", errors.New("path traversal not allowed")
		default:
			out = append(out, part)
		}
	}
	return strings.Join(out, "/"), nil
}

// serverFileAccess: cek server ada + owner; return (uuid, nodeID, ok).
func (s *Server) serverFileAccess(w http.ResponseWriter, r *http.Request, id int64) (string, int64, bool) {
	var uuid string
	var nodeID, ownerID int64
	err := s.DB.QueryRow(`SELECT uuid, node_id, owner_id FROM servers WHERE id=?`, id).
		Scan(&uuid, &nodeID, &ownerID)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "server not found")
		return "", 0, false
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return "", 0, false
	}
	sess := auth.SessionFrom(r.Context())
	if !auth.IsOwnerOrAdmin(sess, ownerID) {
		writeErr(w, 403, "not owner")
		return "", 0, false
	}
	return uuid, nodeID, true
}

// writeJSONRaw: kirim raw JSON hasil rpc:res daemon apa adanya.
func writeJSONRaw(w http.ResponseWriter, status int, raw json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// ---- handlers ----

// daemonRPCError translates DaemonRPC errors to consistent HTTP responses.
// If the node is offline, it returns 503 "node offline".
// Other errors return 502 with the error message.
func daemonRPCError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	if err.Error() == "node offline" {
		writeErr(w, 503, "node offline")
		return
	}
	writeErr(w, 502, err.Error())
}

// handleFileList: GET /api/servers/{id}/files?path=...
func (s *Server) handleFileList(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	uuid, nodeID, ok := s.serverFileAccess(w, r, id)
	if !ok {
		return
	}
	p, _ := cleanRelPath(r.URL.Query().Get("path"))
	raw, err := DaemonRPC(s, nodeID, "file", map[string]any{
		"server_uuid": uuid, "op": "list", "path": p,
	}, 15*time.Second, nil)
	if err != nil {
		daemonRPCError(w, err)
		return
	}
	writeJSONRaw(w, http.StatusOK, raw)
}

// handleFileMkdir: POST /api/servers/{id}/files/mkdir {path}
func (s *Server) handleFileMkdir(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	uuid, nodeID, ok := s.serverFileAccess(w, r, id)
	if !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	p, err := cleanRelPath(req.Path)
	if err != nil || p == "" {
		writeErr(w, 422, "path required (no traversal)")
		return
	}
	if _, err := DaemonRPC(s, nodeID, "file", map[string]any{
		"server_uuid": uuid, "op": "mkdir", "path": p,
	}, fileRPCTimeout, nil); err != nil {
		daemonRPCError(w, err)
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "files.mkdir", p)
	writeJSON(w, 200, map[string]string{"ok": "created"})
}

// handleFileDelete: DELETE /api/servers/{id}/files?path=...
func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	uuid, nodeID, ok := s.serverFileAccess(w, r, id)
	if !ok {
		return
	}
	p, err := cleanRelPath(r.URL.Query().Get("path"))
	if err != nil || p == "" {
		writeErr(w, 422, "path required (no traversal)")
		return
	}
	if _, err := DaemonRPC(s, nodeID, "file", map[string]any{
		"server_uuid": uuid, "op": "delete", "path": p,
	}, fileRPCTimeout, nil); err != nil {
		daemonRPCError(w, err)
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "files.delete", p)
	writeJSON(w, 200, map[string]string{"ok": "deleted"})
}

// handleFileRename: POST /api/servers/{id}/files/rename {from, to}
func (s *Server) handleFileRename(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	uuid, nodeID, ok := s.serverFileAccess(w, r, id)
	if !ok {
		return
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	from, err := cleanRelPath(req.From)
	if err != nil || from == "" {
		writeErr(w, 422, "from path invalid")
		return
	}
	to, err := cleanRelPath(req.To)
	if err != nil || to == "" {
		writeErr(w, 422, "to path required (no traversal)")
		return
	}
	if _, err := DaemonRPC(s, nodeID, "file", map[string]any{
		"server_uuid": uuid, "op": "rename", "path": from, "to": to,
	}, fileRPCTimeout, nil); err != nil {
		daemonRPCError(w, err)
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "files.rename", from+" -> "+to)
	writeJSON(w, 200, map[string]string{"ok": "renamed"})
}

// handleFileExtract: POST /api/servers/{id}/files/extract {path, to}
func (s *Server) handleFileExtract(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	uuid, nodeID, ok := s.serverFileAccess(w, r, id)
	if !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
		To   string `json:"to"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	src, err := cleanRelPath(req.Path)
	if err != nil || src == "" {
		writeErr(w, 422, "path required (zip file)")
		return
	}
	dst, err := cleanRelPath(req.To)
	if err != nil {
		writeErr(w, 422, "invalid destination")
		return
	}
	res, err := DaemonRPC(s, nodeID, "file", map[string]any{
		"server_uuid": uuid, "op": "extract", "path": src, "to": dst,
	}, 120*time.Second, nil) // extract bisa lama
	if err != nil {
		daemonRPCError(w, err)
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "files.extract", src)
	writeJSONRaw(w, http.StatusOK, res)
}

// handleFileUsage: GET /api/servers/{id}/files/usage
func (s *Server) handleFileUsage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	uuid, nodeID, ok := s.serverFileAccess(w, r, id)
	if !ok {
		return
	}
	raw, err := DaemonRPC(s, nodeID, "file", map[string]any{
		"server_uuid": uuid, "op": "usage",
	}, fileRPCTimeout, nil)
	if err != nil {
		daemonRPCError(w, err)
		return
	}
	writeJSONRaw(w, http.StatusOK, raw)
}

// handleFileRead (download): GET /api/servers/{id}/files/content?path=...
// Streaming: chunk dari daemon langsung di-pump ke ResponseWriter.
func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	uuid, nodeID, ok := s.serverFileAccess(w, r, id)
	if !ok {
		return
	}
	p, err := cleanRelPath(r.URL.Query().Get("path"))
	if err != nil || p == "" {
		writeErr(w, 422, "path required (no traversal)")
		return
	}
	flusher, _ := w.(http.Flusher)
	firstChunk := true
	_, err = DaemonRPC(s, nodeID, "file:read-begin", map[string]any{
		"server_uuid": uuid, "path": p,
	}, fileReadTimeout, func(chunkB64 string) error {
		data, derr := base64.StdEncoding.DecodeString(chunkB64)
		if derr != nil {
			return derr
		}
		if firstChunk {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			firstChunk = false
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	})
	if err != nil {
		if firstChunk {
			daemonRPCError(w, err)
			return
		}
		// header sudah terkirim — log saja (client melihat koneksi putus)
		log.Printf("file download %s/%s: %v", uuid, p, err)
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "files.download", p)
}

// handleFileWrite (upload): PUT /api/servers/{id}/files?path=...
// Body mentah (raw bytes) dari browser di-pump sebagai chunk 32KB ke daemon.
// Streaming murni — TIDAK ada buffer seluruh body (bisa 500MB).
func (s *Server) handleFileWrite(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	uuid, nodeID, ok := s.serverFileAccess(w, r, id)
	if !ok {
		return
	}
	p, err := cleanRelPath(r.URL.Query().Get("path"))
	if err != nil || p == "" {
		writeErr(w, 422, "path required (no traversal)")
		return
	}
	conn := s.Hub.Get(nodeID)
	if conn == nil {
		writeErr(w, 503, "node offline")
		return
	}

	maxUpload := s.MaxUploadBytes
	if maxUpload <= 0 {
		maxUpload = defaultMaxUploadBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)

	reqID := newReqID()
	// daftarkan waiter SEBELUM begin dikirim — rpc:res bisa datang cepat.
	rpcWait.mu.Lock()
	ch := make(chan rpcEvent, 1)
	rpcWait.wait[reqID] = ch
	rpcWait.mu.Unlock()
	defer func() {
		rpcWait.mu.Lock()
		delete(rpcWait.wait, reqID)
		rpcWait.mu.Unlock()
	}()
	// 1. write-begin
	begin, _ := json.Marshal(map[string]any{
		"req_id": reqID, "server_uuid": uuid, "path": p,
	})
	if err := conn.Send(daemonhub.Frame{Ch: "rpc", Type: "file:write-begin", Payload: begin}); err != nil {
		if errors.Is(err, daemonhub.ErrNotOnline) {
			writeErr(w, 503, "node offline")
			return
		}
		writeErr(w, 502, err.Error())
		return
	}
	// 2. pump body → chunk frames (base64). Buffer 32KB — tidak pernah
	// menampung seluruh body di RAM.
	chunk := make([]byte, fileChunkSize)
	for {
		n, rerr := r.Body.Read(chunk)
		if n > 0 {
			cp, _ := json.Marshal(map[string]string{
				"req_id": reqID, "server_uuid": uuid, "path": p,
				"d": base64.StdEncoding.EncodeToString(chunk[:n]),
			})
			if err := conn.Send(daemonhub.Frame{Ch: "rpc", Type: "file:write-chunk", Payload: cp}); err != nil {
				if errors.Is(err, daemonhub.ErrNotOnline) {
					writeErr(w, 503, "node offline")
					return
				}
				writeErr(w, 502, "daemon send failed")
				return
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			var maxErr *http.MaxBytesError
			if errors.As(rerr, &maxErr) {
				writeErr(w, http.StatusRequestEntityTooLarge, "upload size exceeds limit")
				return
			}
			writeErr(w, 400, "read body: "+rerr.Error())
			return
		}
	}
	// 3. write-end → tunggu hasil (total byte terulis, dari daemon)
	end, _ := json.Marshal(map[string]any{
		"req_id": reqID, "server_uuid": uuid, "path": p,
	})
	if err := conn.Send(daemonhub.Frame{Ch: "rpc", Type: "file:write-end", Payload: end}); err != nil {
		if errors.Is(err, daemonhub.ErrNotOnline) {
			writeErr(w, 503, "node offline")
			return
		}
		writeErr(w, 502, err.Error())
		return
	}
	res, err := waitForWriteEnd(conn, reqID, fileWriteTimeout)
	if err != nil {
		daemonRPCError(w, err)
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "files.write", p)
	writeJSONRaw(w, http.StatusOK, res)
}

// waitForWriteEnd: tunggu rpc:res untuk req_id upload (channel sudah
// terdaftar sejak write-begin; hasil diinject resolveRPCWait).
func waitForWriteEnd(conn *daemonhub.Conn, reqID string, timeout time.Duration) (json.RawMessage, error) {
	rpcWait.mu.Lock()
	ch, ok := rpcWait.wait[reqID]
	rpcWait.mu.Unlock()
	if !ok {
		return nil, errors.New("write waiter missing")
	}
	select {
	case ev := <-ch:
		if ev.Final == nil {
			return nil, errors.New("unexpected rpc event")
		}
		if ev.Final.Err != nil {
			return nil, ev.Final.Err
		}
		return ev.Final.Result, nil
	case <-time.After(timeout):
		return nil, errors.New("daemon write timeout")
	}
}
