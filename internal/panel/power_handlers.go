// Power handler: REST panel → frame WS rpc ke daemon → ContainerRuntime.
// Panel TIDAK mengubah state container — hanya meneruskan perintah (SPEC §1/§7.1).
package panel

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
	"garudapanel/internal/daemonhub"
)

// powerReq body POST /api/servers/:id/power
type powerReq struct {
	Action string `json:"action"` // start|stop|restart|kill
}

// handleServerPower: POST /api/servers/:id/power
func (s *Server) handleServerPower(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req powerReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	switch req.Action {
	case "start", "stop", "restart", "kill":
	default:
		writeErr(w, 422, "action must be start|stop|restart|kill")
		return
	}

	// server ada + ambil node, uuid, owner (cek akses: owner atau admin)
	var uuid string
	var nodeID, ownerID int64
	err := s.DB.QueryRow(`SELECT uuid, node_id, owner_id FROM servers WHERE id=?`, id).Scan(&uuid, &nodeID, &ownerID)
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

	// node harus online
	conn := s.Hub.Get(nodeID)
	if conn == nil {
		writeErr(w, 503, "node offline")
		return
	}

	payload, _ := json.Marshal(map[string]string{"server_uuid": uuid, "action": req.Action})
	if err := conn.Send(daemonhub.Frame{Ch: "rpc", Type: "power", Payload: payload}); err != nil {
		writeErr(w, 502, "send to daemon: "+err.Error())
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "server.power", "action="+req.Action)
	// balasan status mengalir via WS daemon → update servers.status (asinkron)
	writeJSON(w, 202, map[string]string{"ok": "command sent", "action": req.Action})
}
