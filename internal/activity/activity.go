// Package activity: helper pencatatan activity_logs (SPEC §5.9).
package activity

import (
	"database/sql"
	"log"
)

// Log mencatat aksi ke activity_logs. Error di-log lalu diabaikan —
// kegagalan logging tidak boleh menggagalkan aksi utama.
// actorID/serverID = 0 berarti tidak ada (disimpan NULL).
func Log(db *sql.DB, actorID, serverID int64, action, detail string) {
	if db == nil {
		return
	}
	var actor, srv any
	if actorID > 0 {
		actor = actorID
	}
	if serverID > 0 {
		srv = serverID
	}
	_, err := db.Exec(`INSERT INTO activity_logs (actor_id, server_id, action, detail) VALUES (?,?,?,?)`,
		actor, srv, action, detail)
	if err != nil {
		// log saja; jangan panic
		log.Printf("activity log error: %v", err)
	}
}

// List mengambil N log terakhir (opsional filter server_id > 0).
func List(db *sql.DB, serverID int64, limit int) ([]map[string]any, error) {
	q := `SELECT l.id, COALESCE(l.actor_id,0), COALESCE(l.server_id,0), l.action, l.detail, l.created_at
		FROM activity_logs l`
	args := []any{}
	if serverID > 0 {
		q += ` WHERE l.server_id = ?`
		args = append(args, serverID)
	}
	q += ` ORDER BY l.id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, actor, srv int64
		var action, detail string
		var created int64
		if err := rows.Scan(&id, &actor, &srv, &action, &detail, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "actor_id": actor, "server_id": srv,
			"action": action, "detail": detail, "created_at": created,
		})
	}
	return out, rows.Err()
}
