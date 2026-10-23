package panel

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
)

const joinTokenTTL = 15 * time.Minute // SPEC §8

// ---- Nodes CRUD (admin) ----

type nodeReq struct {
	LocationID    int64  `json:"location_id"`
	Name          string `json:"name"`
	FQDN          string `json:"fqdn"`
	MemoryLimitMB int64  `json:"memory_limit_mb"`
	DiskLimitMB   int64  `json:"disk_limit_mb"`
}

// nodeJSON: baris node untuk response (daemon_token_hash TIDAK pernah dikirim).
func (s *Server) nodeOut(id int64) (map[string]any, error) {
	var locID int64
	var name, fqdn string
	var mem, disk int64
	var created int64
	var tokenSet bool
	err := s.DB.QueryRow(`SELECT location_id, name, fqdn, memory_limit_mb, disk_limit_mb, created_at,
		(daemon_token_hash IS NOT NULL AND daemon_token_hash != '') FROM nodes WHERE id=?`, id).
		Scan(&locID, &name, &fqdn, &mem, &disk, &created, &tokenSet)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": id, "location_id": locID, "name": name, "fqdn": fqdn,
		"memory_limit_mb": mem, "disk_limit_mb": disk,
		"created_at": created, "joined": tokenSet,
		"online": s.Hub.IsOnline(id),
	}, nil
}

func (s *Server) handleNodesList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT id FROM nodes ORDER BY id`)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		n, err := s.nodeOut(id)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	writeJSON(w, 200, out)
}

// Create node → balikan join token sekali pakai (SPEC §4: POST /api/admin/nodes).
func (s *Server) handleNodesCreate(w http.ResponseWriter, r *http.Request) {
	var req nodeReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if req.Name == "" || req.FQDN == "" || req.LocationID <= 0 {
		writeErr(w, 422, "name, fqdn, location_id required")
		return
	}
	var locExists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM locations WHERE id=?`, req.LocationID).Scan(&locExists); err != nil || locExists == 0 {
		writeErr(w, 422, "location_id does not exist")
		return
	}
	res, err := s.DB.Exec(`INSERT INTO nodes (location_id, name, fqdn, memory_limit_mb, disk_limit_mb) VALUES (?,?,?,?,?)`,
		req.LocationID, req.Name, req.FQDN, req.MemoryLimitMB, req.DiskLimitMB)
	if err != nil {
		writeErr(w, 409, "node name already exists")
		return
	}
	id, _ := res.LastInsertId()

	// buat join token sekali pakai (15 menit).
	joinRaw, joinHash, err := auth.NewToken()
	if err != nil {
		writeErr(w, 500, "token gen failed")
		return
	}
	if _, err := s.DB.Exec(`INSERT INTO node_join_tokens (token_hash, node_id, expires_at) VALUES (?,?,?)`,
		joinHash, id, time.Now().Add(joinTokenTTL).Unix()); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.nodes.create", "node="+req.Name)
	writeJSON(w, 201, map[string]any{
		"id":                    id,
		"name":                  req.Name,
		"join_token":            joinRaw, // hanya tampil sekali
		"join_token_expires_in": int(joinTokenTTL.Seconds()),
	})
}

func (s *Server) handleNodesGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	n, err := s.nodeOut(id)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "node not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	writeJSON(w, 200, n)
}

func (s *Server) handleNodesUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req nodeReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	set, args := []string{}, []any{}
	if req.Name != "" {
		set = append(set, "name=?")
		args = append(args, req.Name)
	}
	if req.FQDN != "" {
		set = append(set, "fqdn=?")
		args = append(args, req.FQDN)
	}
	if req.LocationID > 0 {
		set = append(set, "location_id=?")
		args = append(args, req.LocationID)
	}
	if req.MemoryLimitMB > 0 {
		set = append(set, "memory_limit_mb=?")
		args = append(args, req.MemoryLimitMB)
	}
	if req.DiskLimitMB > 0 {
		set = append(set, "disk_limit_mb=?")
		args = append(args, req.DiskLimitMB)
	}
	if len(set) == 0 {
		writeErr(w, 400, "nothing to update")
		return
	}
	args = append(args, id)
	if _, err := s.DB.Exec(`UPDATE nodes SET `+stringsJoin(set, ",")+` WHERE id=?`, args...); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.nodes.update", fmt.Sprintf("node_id=%d", id))
	writeJSON(w, 200, map[string]string{"ok": "updated"})
}

// RevokeNodeToken: cabut daemon token (node harus join ulang).
func (s *Server) handleNodesRevokeToken(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.DB.Exec(`UPDATE nodes SET daemon_token_hash=NULL WHERE id=?`, id); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.nodes.revoke_token", fmt.Sprintf("node_id=%d", id))
	writeJSON(w, 200, map[string]string{"ok": "node token revoked"})
}

func (s *Server) handleNodesDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	actor := auth.SessionFrom(r.Context())
	res, err := s.DB.Exec(`DELETE FROM nodes WHERE id=?`, id)
	if err != nil {
		writeErr(w, 409, "node still has servers referencing it")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 404, "node not found")
		return
	}
	activity.Log(s.DB, actor.UserID, 0, "admin.nodes.delete", fmt.Sprintf("node_id=%d", id))
	writeJSON(w, 200, map[string]string{"ok": "deleted"})
}

func stringsJoin(elems []string, sep string) string {
	out := ""
	for i, e := range elems {
		if i > 0 {
			out += sep
		}
		out += e
	}
	return out
}

// ---- Locations CRUD (ringkas; dibutuhkan node create) ----

type locationReq struct {
	ShortCode   string `json:"short_code"`
	Description string `json:"description"`
}

func (s *Server) handleLocationsList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT id, short_code, description FROM locations ORDER BY id`)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var c, d string
		_ = rows.Scan(&id, &c, &d)
		out = append(out, map[string]any{"id": id, "short_code": c, "description": d})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleLocationsCreate(w http.ResponseWriter, r *http.Request) {
	var req locationReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if len(req.ShortCode) < 2 || len(req.ShortCode) > 16 {
		writeErr(w, 422, "short_code 2-16 chars")
		return
	}
	res, err := s.DB.Exec(`INSERT INTO locations (short_code, description) VALUES (?,?)`, req.ShortCode, req.Description)
	if err != nil {
		writeErr(w, 409, "short_code exists")
		return
	}
	id, _ := res.LastInsertId()
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.locations.create", req.ShortCode)
	writeJSON(w, 201, map[string]any{"id": id, "short_code": req.ShortCode})
}

func (s *Server) handleLocationsDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.DB.Exec(`DELETE FROM locations WHERE id=?`, id); err != nil {
		writeErr(w, 409, "location still used by nodes")
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "deleted"})
}

// ---- Allocations ----

type allocReq struct {
	NodeID int64  `json:"node_id"`
	IP     string `json:"ip"`
	Port   int    `json:"port"`
}

func (s *Server) handleAllocationsCreate(w http.ResponseWriter, r *http.Request) {
	var req allocReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if req.Port < 1 || req.Port > 65535 || req.IP == "" || req.NodeID <= 0 {
		writeErr(w, 422, "node_id, ip, port(1-65535) required")
		return
	}
	res, err := s.DB.Exec(`INSERT INTO allocations (node_id, ip, port) VALUES (?,?,?)`,
		req.NodeID, req.IP, req.Port)
	if err != nil {
		writeErr(w, 409, "allocation conflict (duplicate ip:port on node)")
		return
	}
	id, _ := res.LastInsertId()
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.allocations.create", fmt.Sprintf("node=%d port=%d", req.NodeID, req.Port))
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Server) handleAllocationsList(w http.ResponseWriter, r *http.Request) {
	q := `SELECT id, node_id, ip, port, COALESCE(server_id,0) FROM allocations`
	args := []any{}
	if nodeID := chi.URLParam(r, "id"); nodeID != "" {
		if nid, err := strconv.ParseInt(nodeID, 10, 64); err == nil && nid > 0 {
			q += ` WHERE node_id=?`
			args = append(args, nid)
		}
	}
	q += ` ORDER BY id`
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, nid, srv int64
		var ip string
		var port int
		_ = rows.Scan(&id, &nid, &ip, &port, &srv)
		out = append(out, map[string]any{"id": id, "node_id": nid, "ip": ip, "port": port, "server_id": srv})
	}
	writeJSON(w, 200, out)
}
