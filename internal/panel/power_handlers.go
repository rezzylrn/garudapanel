// Power handler: REST panel → frame WS rpc ke daemon → ContainerRuntime.
// Panel TIDAK mengubah state container — hanya meneruskan perintah (SPEC §1/§7.1).
package panel

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
	"garudapanel/internal/daemonhub"
)

// powerReq body POST /api/servers/:id/power
type powerReq struct {
	Action string `json:"action"` // start|stop|restart|kill
}

// State guard — pola Pterodactyl PowerButtons.tsx (Start hanya saat offline,
// Stop jadi Kill saat stopping), DIPERKETAT atas permintaan rezz: Stop/Restart
// DILARANG saat status masih "starting" (harus error, bukan diam-diam
// di-disable), dan Kill adalah escape hatch saat starting macet.
// Backend guard ini pertahanan lapis kedua; frontend menerapkan aturan yang
// sama sebelum menampilkan dialog konfirmasi.
func powerAllowed(status, action string, lastStart time.Time, now time.Time) (bool, string) {
	switch action {
	case "start":
		if status != "offline" {
			return false, "server is " + status + ", cannot start"
		}
	case "stop":
		if status == "starting" {
			return false, "server is still starting — wait until it is running before stopping"
		}
		if status == "stopping" {
			return false, "server is already stopping — use kill to force stop"
		}
		if status != "running" {
			return false, "server is " + status + ", cannot stop"
		}
		// Jendela balapan: daemon belum sempat melaporkan status "starting"
		// setelah perintah start/restart baru saja diteruskan. Jendela
		// dibuat PENDEK (3 detik): lebih dari itu, status "running" di DB
		// berarti start sudah selesai dan stop harus diizinkan — jendela
		// 20 detik sebelumnya salah memblokir stop yang sah sesaat
		// setelah server mencapai running.
		if !lastStart.IsZero() && now.Sub(lastStart) < 3*time.Second {
			return false, "server is still starting — wait until it is running before stopping"
		}
	case "restart":
		if status == "starting" {
			return false, "server is still starting — wait until it is running before restarting"
		}
		if status != "running" {
			return false, "server is " + status + ", cannot restart"
		}
		if !lastStart.IsZero() && now.Sub(lastStart) < 3*time.Second {
			return false, "server is still starting — wait until it is running before restarting"
		}
	case "kill":
		if status == "offline" {
			return false, "server is already offline"
		}
		// Kill boleh kapan saja selain offline: escape hatch untuk
		// server yang macet di "starting".
	}
	return true, ""
}

// lastPowerStart: kapan terakhir perintah start/restart diteruskan per server
// (menutup jendela balapan sebelum daemon melaporkan status "starting").
var (
	lastPowerStartMu sync.Mutex
	lastPowerStart   = map[string]time.Time{}
)

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

	// server ada + ambil node, uuid, owner, status (cek akses: owner atau admin)
	var uuid string
	var nodeID, ownerID int64
	var status string
	err := s.DB.QueryRow(`SELECT uuid, node_id, owner_id, status FROM servers WHERE id=?`, id).Scan(&uuid, &nodeID, &ownerID, &status)
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

	// State guard: tolak aksi yang tidak masuk akal untuk status saat ini.
	lastPowerStartMu.Lock()
	lastStart := lastPowerStart[uuid]
	lastPowerStartMu.Unlock()
	if ok, reason := powerAllowed(status, req.Action, lastStart, time.Now()); !ok {
		writeErr(w, 409, reason)
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
		if errors.Is(err, daemonhub.ErrNotOnline) {
			writeErr(w, 503, "node offline")
			return
		}
		writeErr(w, 502, "send to daemon: "+err.Error())
		return
	}
	if req.Action == "start" || req.Action == "restart" {
		lastPowerStartMu.Lock()
		lastPowerStart[uuid] = time.Now()
		lastPowerStartMu.Unlock()
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "server.power", "action="+req.Action)
	// balasan status mengalir via WS daemon → update servers.status (asinkron)
	writeJSON(w, 202, map[string]string{"ok": "command sent", "action": req.Action})
}
