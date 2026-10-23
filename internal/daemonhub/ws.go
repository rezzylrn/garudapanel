// Package daemonhub: WebSocket endpoint penerima koneksi reverse dari daemon.
package daemonhub

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"garudapanel/internal/auth"
)

// Frame: format pesan WS panel↔daemon (SPEC §4).
type Frame struct {
	Ch      string          `json:"ch"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// WSHandler mengembalikan handler endpoint /api/daemon/ws.
// Daemon auth: node token long-lived via header X-Node-Token.
func WSHandler(db *sql.DB, hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawToken := r.Header.Get("X-Node-Token")
		if rawToken == "" {
			http.Error(w, "missing node token", http.StatusUnauthorized)
			return
		}
		var nodeID int64
		var name string
		err := db.QueryRow(`SELECT id, name FROM nodes WHERE daemon_token_hash = ?`,
			auth.TokenHash(rawToken)).Scan(&nodeID, &name)
		if err == sql.ErrNoRows {
			http.Error(w, "invalid node token", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if err != nil {
			logf("accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusInternalError, "closing")

		c := &Conn{NodeID: nodeID, Name: name}
		wasOffline := hub.Add(c)
		if wasOffline {
			logf("node %d (%s) online", nodeID, name)
		}
		defer func() {
			if hub.Remove(nodeID, c) {
				logf("node %d (%s) offline", nodeID, name)
				// server di node ini dianggap offline (status milik daemon; saat reconnect daemon kirim ulang state)
				markServersOffline(db, nodeID)
			}
		}()

		// Loop baca frame daemon. Fase 1: hello/state/heartbeat dicatat, sisanya diabaikan.
		ctx := r.Context()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return // koneksi tutup (normal atau error)
			}
			var f Frame
			if err := json.Unmarshal(data, &f); err != nil {
				logf("node %d: bad frame: %v", nodeID, err)
				continue
			}
			switch f.Type {
			case "hello":
				logf("node %d hello", nodeID)
			case "state":
				var st struct {
					ServerUUID string `json:"server_uuid"`
					Status     string `json:"status"`
				}
				if json.Unmarshal(f.Payload, &st) == nil && st.ServerUUID != "" {
					updateServerStatus(db, st.ServerUUID, st.Status)
				}
			case "heartbeat":
				// cukup diterima; node dianggap online selama koneksi hidup
			default:
				// fase 1: abaikan (console, stats, rpc) — fase berikutnya
			}
		}
	}
}

func markServersOffline(db *sql.DB, nodeID int64) {
	if _, err := db.Exec(`UPDATE servers SET status='offline' WHERE node_id=? AND status != 'suspended'`, nodeID); err != nil {
		log.Printf("markServersOffline: %v", err)
	}
}

func updateServerStatus(db *sql.DB, uuid, status string) {
	valid := map[string]bool{"offline": true, "starting": true, "running": true, "stopping": true, "installing": true, "suspended": true}
	if !valid[status] {
		return
	}
	if _, err := db.Exec(`UPDATE servers SET status=? WHERE uuid=?`, status, uuid); err != nil {
		log.Printf("updateServerStatus: %v", err)
	}
}

var _ = time.Now
