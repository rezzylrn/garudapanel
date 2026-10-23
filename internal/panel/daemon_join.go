package panel

import (
	"net/http"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
)

// handleDaemonJoin: POST /api/daemon/join
// Body: {"join_token": "..."} → tukar jadi node token long-lived.
// Join token sekali pakai + expiry 15 menit (SPEC §8).
func (s *Server) handleDaemonJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JoinToken string `json:"join_token"`
	}
	if err := decodeJSON(r, &req); err != nil || req.JoinToken == "" {
		writeErr(w, 400, "join_token required")
		return
	}
	tx, err := s.DB.Begin()
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer tx.Rollback()

	// Atomik: hanya baris yang belum dipakai dan belum expired yang boleh dikonsumsi.
	res, err := tx.Exec(`UPDATE node_join_tokens SET used=1
		WHERE token_hash=? AND used=0 AND expires_at > strftime('%s','now')`,
		auth.TokenHash(req.JoinToken))
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 401, "join token invalid, expired, or already used")
		return
	}
	var nodeID int64
	err = tx.QueryRow(`SELECT node_id FROM node_join_tokens WHERE token_hash=?`,
		auth.TokenHash(req.JoinToken)).Scan(&nodeID)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}

	// node token long-lived baru (revoke implisit token lama).
	rawToken, tokenHash, err := auth.NewToken()
	if err != nil {
		writeErr(w, 500, "token gen failed")
		return
	}
	var nodeName string
	err = tx.QueryRow(`UPDATE nodes SET daemon_token_hash=? WHERE id=? RETURNING name`,
		tokenHash, nodeID).Scan(&nodeName)
	if err != nil {
		writeErr(w, 404, "node not found")
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	activity.Log(s.DB, 0, 0, "daemon.join", "node="+nodeName)
	writeJSON(w, 200, map[string]any{
		"node_id":    nodeID,
		"node_name":  nodeName,
		"node_token": rawToken,
	})
}
