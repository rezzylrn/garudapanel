package panel

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
)

// ---- Servers CRUD (admin) — fase 1: status offline, tanpa daemon ops ----

type serverReq struct {
	Name         string `json:"name"`
	OwnerID      int64  `json:"owner_id"`
	NodeID       int64  `json:"node_id"`
	EggID        int64  `json:"egg_id"`
	AllocationID int64  `json:"allocation_id"`
	MemoryMB     int64  `json:"memory_mb"`
	DiskMB       int64  `json:"disk_mb"`
	CPUPercent   int    `json:"cpu_percent"`
}

func (s *Server) handleServersList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT sv.id, sv.uuid, sv.name, sv.owner_id, sv.node_id, sv.egg_id,
		sv.allocation_id, sv.memory_mb, sv.disk_mb, sv.cpu_percent, sv.status, sv.created_at
		FROM servers sv ORDER BY sv.id`)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, owner, node, egg, alloc, mem, disk, created int64
		var uuid, name, status string
		var cpu int64
		if err := rows.Scan(&id, &uuid, &name, &owner, &node, &egg, &alloc, &mem, &disk, &cpu, &status, &created); err != nil {
			writeErr(w, 500, "scan error")
			return
		}
		out = append(out, map[string]any{
			"id": id, "uuid": uuid, "name": name, "owner_id": owner, "node_id": node,
			"egg_id": egg, "allocation_id": alloc, "memory_mb": mem, "disk_mb": disk,
			"cpu_percent": cpu, "status": status, "created_at": created,
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleServersCreate(w http.ResponseWriter, r *http.Request) {
	var req serverReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if req.Name == "" || req.OwnerID <= 0 || req.NodeID <= 0 || req.EggID <= 0 || req.AllocationID <= 0 {
		writeErr(w, 422, "name, owner_id, node_id, egg_id, allocation_id required")
		return
	}
	if req.MemoryMB <= 0 || req.DiskMB <= 0 {
		writeErr(w, 422, "memory_mb and disk_mb must be > 0")
		return
	}
	if req.CPUPercent <= 0 {
		req.CPUPercent = 100
	}
	// validasi FK + allocation bebas
	var cnt int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, req.OwnerID).Scan(&cnt); err != nil || cnt == 0 {
		writeErr(w, 422, "owner_id does not exist")
		return
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM nodes WHERE id=?`, req.NodeID).Scan(&cnt); err != nil || cnt == 0 {
		writeErr(w, 422, "node_id does not exist")
		return
	}
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM eggs WHERE id=?`, req.EggID).Scan(&cnt); err != nil || cnt == 0 {
		writeErr(w, 422, "egg_id does not exist")
		return
	}
	var allocNode int64
	var allocServer sql.NullInt64
	err := s.DB.QueryRow(`SELECT node_id, server_id FROM allocations WHERE id=?`, req.AllocationID).
		Scan(&allocNode, &allocServer)
	if err == sql.ErrNoRows {
		writeErr(w, 422, "allocation_id does not exist")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	if allocNode != req.NodeID {
		writeErr(w, 422, "allocation belongs to different node")
		return
	}
	if allocServer.Valid {
		writeErr(w, 409, "allocation already assigned to a server")
		return
	}
	uuid, err := newUUID()
	if err != nil {
		writeErr(w, 500, "uuid gen failed")
		return
	}
	tx, err := s.DB.Begin()
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO servers (uuid, name, owner_id, node_id, egg_id, allocation_id, memory_mb, disk_mb, cpu_percent, status)
		VALUES (?,?,?,?,?,?,?,?,?, 'offline')`,
		uuid, req.Name, req.OwnerID, req.NodeID, req.EggID, req.AllocationID, req.MemoryMB, req.DiskMB, req.CPUPercent)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	id, _ := res.LastInsertId()
	if _, err := tx.Exec(`UPDATE allocations SET server_id=? WHERE id=?`, id, req.AllocationID); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "admin.servers.create", "uuid="+uuid)
	writeJSON(w, 201, map[string]any{"id": id, "uuid": uuid, "status": "offline"})
}

func (s *Server) handleServersGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var id2, owner, node, egg, alloc, mem, disk, cpu, created int64
	var uuid, name, status string
	err := s.DB.QueryRow(`SELECT id, uuid, name, owner_id, node_id, egg_id, allocation_id,
		memory_mb, disk_mb, cpu_percent, status, created_at FROM servers WHERE id=?`, id).
		Scan(&id2, &uuid, &name, &owner, &node, &egg, &alloc, &mem, &disk, &cpu, &status, &created)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "server not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	writeJSON(w, 200, map[string]any{
		"id": id2, "uuid": uuid, "name": name, "owner_id": owner, "node_id": node,
		"egg_id": egg, "allocation_id": alloc, "memory_mb": mem, "disk_mb": disk,
		"cpu_percent": cpu, "status": status, "created_at": created,
	})
}

func (s *Server) handleServersDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	actor := auth.SessionFrom(r.Context())
	tx, err := s.DB.Begin()
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer tx.Rollback()
	// bebaskan allocation
	if _, err := tx.Exec(`UPDATE allocations SET server_id=NULL WHERE server_id=?`, id); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	res, err := tx.Exec(`DELETE FROM servers WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 404, "server not found")
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	activity.Log(s.DB, actor.UserID, 0, "admin.servers.delete", fmt.Sprintf("server_id=%d", id))
	writeJSON(w, 200, map[string]string{"ok": "deleted"})
}

// newUUID: RFC 4122 v4 via crypto/rand (tanpa dependensi eksternal).
func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// ---- Server user routes (Milestone 4: User Server List, Detail, Activity, Allocations) ----

// handleUserServersList: GET /api/servers
// Menampilkan semua server milik user (atau semua jika admin), beserta info node & primary allocation.
func (s *Server) handleUserServersList(w http.ResponseWriter, r *http.Request) {
	sess := auth.SessionFrom(r.Context())
	q := `SELECT sv.id, sv.uuid, sv.name, sv.owner_id, sv.node_id, sv.egg_id,
		sv.allocation_id, sv.memory_mb, sv.disk_mb, sv.cpu_percent, sv.status, sv.created_at,
		COALESCE(n.name, ''), COALESCE(a.ip, ''), COALESCE(a.port, 0)
		FROM servers sv
		LEFT JOIN nodes n ON n.id = sv.node_id
		LEFT JOIN allocations a ON a.id = sv.allocation_id`
	args := []any{}
	if !sess.IsAdmin {
		q += ` WHERE sv.owner_id = ?`
		args = append(args, sess.UserID)
	}
	q += ` ORDER BY sv.id DESC`

	rows, err := s.DB.Query(q, args...)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, owner, node, egg, alloc, mem, disk, created int64
		var cpu int
		var uuid, name, status, nodeName, ip string
		var port int
		if err := rows.Scan(&id, &uuid, &name, &owner, &node, &egg, &alloc, &mem, &disk, &cpu, &status, &created,
			&nodeName, &ip, &port); err != nil {
			writeErr(w, 500, "scan error")
			return
		}
		out = append(out, map[string]any{
			"id":            id,
			"uuid":          uuid,
			"name":          name,
			"owner_id":      owner,
			"node_id":       node,
			"node_name":     nodeName,
			"egg_id":        egg,
			"allocation_id": alloc,
			"ip":            ip,
			"port":          port,
			"memory_mb":     mem,
			"disk_mb":       disk,
			"cpu_percent":   cpu,
			"status":        status,
			"created_at":    created,
		})
	}
	writeJSON(w, 200, out)
}

// handleServerUserGet: GET /api/servers/{id}
// Detail server untuk owner atau admin.
func (s *Server) handleServerUserGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sess := auth.SessionFrom(r.Context())
	var id2, owner, node, egg, alloc, mem, disk, created int64
	var cpu int
	var uuid, name, status, nodeName, eggName, ip string
	var port int
	err := s.DB.QueryRow(`SELECT sv.id, sv.uuid, sv.name, sv.owner_id, sv.node_id, sv.egg_id, sv.allocation_id,
		sv.memory_mb, sv.disk_mb, sv.cpu_percent, sv.status, sv.created_at,
		COALESCE(n.name, ''), COALESCE(e.name, ''), COALESCE(a.ip, ''), COALESCE(a.port, 0)
		FROM servers sv
		LEFT JOIN nodes n ON n.id = sv.node_id
		LEFT JOIN eggs e ON e.id = sv.egg_id
		LEFT JOIN allocations a ON a.id = sv.allocation_id
		WHERE sv.id=?`, id).
		Scan(&id2, &uuid, &name, &owner, &node, &egg, &alloc, &mem, &disk, &cpu, &status, &created,
			&nodeName, &eggName, &ip, &port)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "server not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	if !auth.IsOwnerOrAdmin(sess, owner) {
		writeErr(w, 403, "not owner")
		return
	}
	writeJSON(w, 200, map[string]any{
		"id":            id2,
		"uuid":          uuid,
		"name":          name,
		"owner_id":      owner,
		"node_id":       node,
		"node_name":     nodeName,
		"egg_id":        egg,
		"egg_name":      eggName,
		"allocation_id": alloc,
		"ip":            ip,
		"port":          port,
		"memory_mb":     mem,
		"disk_mb":       disk,
		"cpu_percent":   cpu,
		"status":        status,
		"created_at":    created,
	})
}

// handleServerActivity: GET /api/servers/{id}/activity
// Mengambil log aktivitas server (maks 50).
func (s *Server) handleServerActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var ownerID int64
	err := s.DB.QueryRow(`SELECT owner_id FROM servers WHERE id=?`, id).Scan(&ownerID)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "server not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	sess := auth.SessionFrom(r.Context())
	if !auth.IsOwnerOrAdmin(sess, ownerID) {
		writeErr(w, 403, "not owner")
		return
	}

	logs, err := activity.List(s.DB, id, 50)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	writeJSON(w, 200, logs)
}

// handleServerAllocationsList: GET /api/servers/{id}/allocations
// Mengambil semua allocation milik server ini dan menandai yang primary.
func (s *Server) handleServerAllocationsList(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var ownerID, primaryAllocID int64
	err := s.DB.QueryRow(`SELECT owner_id, allocation_id FROM servers WHERE id=?`, id).Scan(&ownerID, &primaryAllocID)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "server not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	sess := auth.SessionFrom(r.Context())
	if !auth.IsOwnerOrAdmin(sess, ownerID) {
		writeErr(w, 403, "not owner")
		return
	}

	rows, err := s.DB.Query(`SELECT id, node_id, ip, port FROM allocations WHERE server_id=? ORDER BY id`, id)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var allocID, nodeID int64
		var ip string
		var port int
		if err := rows.Scan(&allocID, &nodeID, &ip, &port); err != nil {
			writeErr(w, 500, "scan error")
			return
		}
		out = append(out, map[string]any{
			"id":         allocID,
			"node_id":    nodeID,
			"ip":         ip,
			"port":       port,
			"is_primary": allocID == primaryAllocID,
		})
	}
	writeJSON(w, 200, out)
}

// handleServerSetPrimaryAllocation: POST /api/servers/{id}/allocations/primary
// Mengganti primary allocation server ke salah satu allocation yang sudah ter-assign.
func (s *Server) handleServerSetPrimaryAllocation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var ownerID int64
	err := s.DB.QueryRow(`SELECT owner_id FROM servers WHERE id=?`, id).Scan(&ownerID)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "server not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	sess := auth.SessionFrom(r.Context())
	if !auth.IsOwnerOrAdmin(sess, ownerID) {
		writeErr(w, 403, "not owner")
		return
	}

	var req struct {
		AllocationID int64 `json:"allocation_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.AllocationID <= 0 {
		writeErr(w, 400, "allocation_id required")
		return
	}

	// Pastikan allocation tersebut memang assigned ke server ini
	var cnt int
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM allocations WHERE id=? AND server_id=?`, req.AllocationID, id).Scan(&cnt)
	if err != nil || cnt == 0 {
		writeErr(w, 422, "allocation not assigned to this server")
		return
	}

	if _, err := s.DB.Exec(`UPDATE servers SET allocation_id=? WHERE id=?`, req.AllocationID, id); err != nil {
		writeErr(w, 500, "db error")
		return
	}

	activity.Log(s.DB, sess.UserID, id, "server.allocation.primary", fmt.Sprintf("allocation_id=%d", req.AllocationID))
	writeJSON(w, 200, map[string]string{"ok": "primary allocation updated"})
}
