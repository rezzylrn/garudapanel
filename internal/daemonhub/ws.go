// Package daemonhub: WebSocket endpoint penerima koneksi reverse dari daemon.
package daemonhub

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"garudapanel/internal/auth"
)

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
			log.Printf("hub: accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusInternalError, "closing")

		c := &Conn{NodeID: nodeID, Name: name}
		conn.SetReadLimit(1 << 20) // 1MB max frame (console payload besar aman)
		wasOffline := hub.Add(c)
		if wasOffline {
			log.Printf("hub: node %d (%s) online", nodeID, name)
		}
		defer func() {
			c.BindSend(nil) // hentikan kirim: frame baru → ErrNotOnline
			if hub.Remove(nodeID, c) {
				log.Printf("hub: node %d (%s) offline", nodeID, name)
				// status server milik daemon; saat reconnect daemon kirim ulang state
				markServersOffline(db, nodeID)
			}
		}()

		// tulis frame: satu goroutine tulis + channel (Conn tidak boleh
		// ditulis concurrent dari banyak browser console sekaligus).
		writeCh := make(chan Frame, 256)
		c.BindSend(func(f Frame) error {
			select {
			case writeCh <- f:
				return nil
			default:
				return &sendError{"daemon send queue full"}
			}
		})
		go func() {
			for f := range writeCh {
				if err := writeFrame(r.Context(), conn, f); err != nil {
					// koneksi putus: tutup — read loop yang akan membersihkan.
					// JANGAN diam: error tulis di sini = semua frame panel→daemon
					// berikutnya hilang; log supaya ketahuan.
					log.Printf("hub: node %d: write frame %s: %v", nodeID, f.Type, err)
					return
				}
			}
		}()

		// Loop baca frame daemon → dispatch ke router panel.
		ctx := r.Context()
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return // koneksi tutup (normal atau error)
			}
			var f Frame
			if err := json.Unmarshal(data, &f); err != nil {
				log.Printf("hub: node %d: bad frame: %v", nodeID, err)
				continue
			}
			switch f.Type {
			case "state":
				var st struct {
					ServerUUID string `json:"server_uuid"`
					Status     string `json:"status"`
				}
				if json.Unmarshal(f.Payload, &st) == nil && st.ServerUUID != "" {
					updateServerStatus(db, st.ServerUUID, st.Status)
				}
				// teruskan juga ke router (biar console browser dapat status)
				hub.Dispatch(nodeID, f)
			case "hello":
				log.Printf("node %d hello", nodeID)
				// kirim daftar server milik node ini (servers.sync) LEWAT
				// writeCh (c.Send) — SATU-SATUNYA penulis WS. writeFrame
				// langsung di sini pernah bikin goroutine tulis mati diam-diam
				// (concurrent write) sehingga frame power berikutnya hilang.
				if payload := ServersSyncPayload(db, nodeID); payload != nil {
					_ = c.Send(Frame{Ch: "rpc", Type: "servers.sync", Payload: payload})
				}
			case "heartbeat":
				// cukup diterima; node dianggap online selama koneksi hidup
			case "rpc:res", "file:read-chunk":
				// router frame rpc file manager — dipasang panel (files_rpc.go)
				DispatchRPCFrame(f)
			default:
				// console:out, stats, rpc:res → router panel (console browser)
				hub.Dispatch(nodeID, f)
			}
		}
	}
}

// ServersSyncPayload: buat payload servers.sync untuk node (server yang
// terdaftar di node ini) — dikirim saat daemon hello.
func ServersSyncPayload(db *sql.DB, nodeID int64) []byte {
	rows, err := db.Query(`SELECT sv.uuid, e.docker_images, sv.memory_mb, sv.cpu_percent, e.config_stop, a.ip, a.port,
		sv.disk_mb, COALESCE(e.startup_cmd, ''), COALESCE(e.config_startup_done, '')
		FROM servers sv
		JOIN eggs e ON e.id = sv.egg_id
		JOIN allocations a ON a.id = sv.allocation_id
		WHERE sv.node_id = ?`, nodeID)
	if err != nil {
		log.Printf("ServersSyncPayload: %v", err)
		return nil
	}
	defer rows.Close()
	type syncServer struct {
		UUID        string   `json:"uuid"`
		Image       string   `json:"image"`
		Cmd         []string `json:"cmd"`
		Env         []string `json:"env"`
		MemoryMB    int64    `json:"memory_mb"`
		CPUPercent  int      `json:"cpu_percent"`
		StopCommand string   `json:"stop_command"`
		Port        int      `json:"port"`
		DiskMB      int64    `json:"disk_mb"`
		ImageUser   string   `json:"image_user"` // kosong → daemon resolve dari image config
		StartupDone string   `json:"startup_done"`
	}
	type rawSyncServer struct {
		syncServer
		imagesJSON string
		ip         string
		port       int
		startupCmd string
	}
	rawServers := []rawSyncServer{}
	for rows.Next() {
		var s rawSyncServer
		if err := rows.Scan(&s.UUID, &s.imagesJSON, &s.MemoryMB, &s.CPUPercent, &s.StopCommand, &s.ip, &s.port, &s.DiskMB, &s.startupCmd, &s.StartupDone); err != nil {
			continue
		}
		s.Port = s.port
		// docker_images adalah JSON map nama->image; pakai entry pertama.
		// Server bisa override image via kolomnya sendiri (phase berikutnya).
		var images map[string]string
		if json.Unmarshal([]byte(s.imagesJSON), &images) == nil {
			for _, img := range images {
				s.Image = img
				break
			}
		}
		rawServers = append(rawServers, s)
	}
	rows.Close() // pastikan rows ditutup sebelum query serverVariableEnv

	servers := make([]syncServer, 0, len(rawServers))
	for _, raw := range rawServers {
		s := raw.syncServer
		// env: SERVER_IP/PORT + egg variables (nilai server_variables override
		// default; SPEC §3/§5.8 — variable ikut env container).
		env := append([]string{"SERVER_IP=" + raw.ip, "SERVER_PORT=" + itoa(raw.port)},
			serverVariableEnv(db, s.UUID)...)
		s.Env = env
		// startup: egg startup_cmd dengan {{VAR}} disubstitusi nilai variabel.
		// ${VAR} dibiarkan — diekspansi shell dari env saat jalan. Bash wajib
		// (egg memakai sintaks [[ ]]).
		s.Cmd = []string{"/bin/bash", "-c", substituteStartupVars(raw.startupCmd, env)}
		servers = append(servers, s)
	}
	b, err := json.Marshal(map[string]any{"servers": servers})
	if err != nil {
		return nil
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// writeFrame: tulis frame ke conn (di daemonhub, timeout 5 detik).
func writeFrame(ctx context.Context, conn *websocket.Conn, f Frame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, data)
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
