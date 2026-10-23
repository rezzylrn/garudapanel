// Console WS: browser ↔ panel ↔ daemon (SPEC §4).
//
// Protokol browser↔panel:
//
//	Client→server: {t:"auth", token}, {t:"in", d:"..."}, {t:"resize", cols, rows}
//	Server→client: {t:"out", d:"..."}, {t:"stat", d:{...}}, {t:"status", s:"..."}
//
// Token WS sekali pakai, expiry 60 detik. Maks 30 sesi per server.
// Throttle output: 2000 baris / 100ms (burst diantrekan lalu dibatasi).
package panel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"garudapanel/internal/daemonhub"
)

// consoleRouter: registry sesi browser aktif.
type consoleRouter struct {
	mu     sync.Mutex
	bySess map[string]*consoleSession // sessID → sesi
	counts map[string]int             // uuid server → jumlah sesi aktif
}

var consoleRouterInst = &consoleRouter{
	bySess: map[string]*consoleSession{},
	counts: map[string]int{},
}

// consoleSession: satu sesi browser console di panel.
type consoleSession struct {
	uuid    string
	nodeID  int64
	conn    *websocket.Conn
	writeCh chan []byte
	sessID  string
	// dConn: koneksi daemon (dibaca sekali saat open; kalau daemon reconnect
	// sesi ini ditutup oleh daemon close frame / browser reconnect).
}

// ConsoleDaemonFrame: router frame daemon → browser (dipasang via Hub.SetOnFrame).
// console:<uuid> → semua sesi browser server uuid itu.
func ConsoleRouter(f daemonhub.Frame) {
	if !strings.HasPrefix(f.Ch, "console:") {
		return
	}
	uuid := strings.TrimPrefix(f.Ch, "console:")
	consoleRouterInst.mu.Lock()
	sessions := make([]*consoleSession, 0, 4)
	for _, sess := range consoleRouterInst.bySess {
		if sess.uuid == uuid {
			sessions = append(sessions, sess)
		}
	}
	consoleRouterInst.mu.Unlock()
	for _, sess := range sessions {
		sess.deliver(f)
	}
}

// deliver: frame dari daemon → antre tulis browser (drop bila penuh).
func (c *consoleSession) deliver(f daemonhub.Frame) {
	switch f.Type {
	case "out":
		var p struct {
			D string `json:"d"`
		}
		if json.Unmarshal(f.Payload, &p) == nil {
			b, _ := json.Marshal(map[string]string{"t": "out", "d": p.D})
			select {
			case c.writeCh <- b:
			default: // browser lambat: drop (log tetap di ring buffer daemon)
			}
		}
	case "stat":
		b, _ := json.Marshal(map[string]any{"t": "stat", "d": json.RawMessage(f.Payload)})
		select {
		case c.writeCh <- b:
		default:
		}
	}
}

// handleServerWS: GET /api/servers/:id/ws
// Auth: frame pertama browser {t:"auth", token} — token sekali pakai 60 detik.
func (s *Server) handleServerWS(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "")

	ctx := r.Context()

	// tunggu frame auth (maks 5 detik)
	var tok *wsToken
	authC := make(chan struct{})
	go func() {
		defer close(authC)
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var m struct {
			T     string `json:"t"`
			Token string `json:"token"`
		}
		if json.Unmarshal(data, &m) != nil || m.T != "auth" {
			return
		}
		t := consumeConsoleToken(m.Token)
		if t == nil || t.serverID != id {
			return // token invalid/expired/server salah
		}
		tok = t
	}()
	select {
	case <-authC:
	case <-time.After(5 * time.Second):
		conn.Close(websocket.StatusPolicyViolation, "auth timeout")
		return
	}
	if tok == nil {
		conn.Close(websocket.StatusPolicyViolation, "invalid or expired token")
		return
	}

	// (dilanjutkan di bawah: runConsoleSession)
	s.runConsoleSession(ctx, conn, tok)
}

// runConsoleSession: sesi console aktif setelah auth lolos.
func (s *Server) runConsoleSession(ctx context.Context, conn *websocket.Conn, tok *wsToken) {
	var uuid string
	var nodeID int64
	err := s.DB.QueryRow(`SELECT uuid, node_id FROM servers WHERE id=?`, tok.serverID).Scan(&uuid, &nodeID)
	if err != nil {
		_ = sendBrowser(conn, "status", map[string]string{"s": "server not found"})
		return
	}
	dConn := s.Hub.Get(nodeID)
	if dConn == nil {
		_ = sendBrowser(conn, "status", map[string]string{"s": "node offline"})
		return
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	sessID := uuid + ":" + hex.EncodeToString(b)

	sess := &consoleSession{uuid: uuid, nodeID: nodeID, conn: conn, writeCh: make(chan []byte, 1024), sessID: sessID}
	// batasi 30 sesi per server
	consoleRouterInst.mu.Lock()
	if consoleRouterInst.counts[uuid] >= maxConsoleSessions {
		consoleRouterInst.mu.Unlock()
		_ = sendBrowser(conn, "status", map[string]string{"s": "too many sessions"})
		return
	}
	consoleRouterInst.counts[uuid]++
	consoleRouterInst.bySess[sessID] = sess
	consoleRouterInst.mu.Unlock()
	defer func() {
		consoleRouterInst.mu.Lock()
		consoleRouterInst.counts[uuid]--
		if consoleRouterInst.counts[uuid] <= 0 {
			delete(consoleRouterInst.counts, uuid)
		}
		delete(consoleRouterInst.bySess, sessID)
		consoleRouterInst.mu.Unlock()
	}()

	// daemon buka sesi console + replay log terakhir
	openP, _ := json.Marshal(map[string]string{"id": sessID})
	if err := dConn.Send(daemonhub.Frame{Ch: "console:" + uuid, Type: "open", Payload: openP}); err != nil {
		_ = sendBrowser(conn, "status", map[string]string{"s": "daemon send failed"})
		return
	}
	replayP, _ := json.Marshal(map[string]string{"server_uuid": uuid})
	_ = dConn.Send(daemonhub.Frame{Ch: "rpc", Type: "replay", Payload: replayP})

	// goroutine baca browser: in/resize → daemon
	go func() {
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return // browser putus
			}
			var m struct {
				T    string `json:"t"`
				D    string `json:"d"`
				Cols uint   `json:"cols"`
				Rows uint   `json:"rows"`
			}
			if json.Unmarshal(data, &m) != nil {
				continue
			}
			switch m.T {
			case "in":
				p, _ := json.Marshal(map[string]string{"id": sessID, "d": m.D})
				_ = dConn.Send(daemonhub.Frame{Ch: "console:" + uuid, Type: "in", Payload: p})
			case "resize":
				p, _ := json.Marshal(map[string]any{"id": sessID, "cols": m.Cols, "rows": m.Rows})
				_ = dConn.Send(daemonhub.Frame{Ch: "console:" + uuid, Type: "resize", Payload: p})
			}
		}
	}()

	// loop tulis: kirim frame {t:out} ke browser dengan throttle
	// (maks 2000 baris per 100ms — SPEC §4).
	ticker := time.NewTicker(throttleWindow)
	defer ticker.Stop()
	lines := 0
	for {
		select {
		case <-ctx.Done():
			// browser putus → beri tahu daemon tutup sesi console
			closeP, _ := json.Marshal(map[string]string{"id": sessID})
			_ = dConn.Send(daemonhub.Frame{Ch: "console:" + uuid, Type: "close", Payload: closeP})
			return
		case msg := <-sess.writeCh:
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := conn.Write(wctx, websocket.MessageText, msg); err != nil {
				cancel()
				return
			}
			cancel()
			lines++
			if lines >= throttleMaxLines {
				<-ticker.C // tunggu window baru
				lines = 0
			}
		}
	}
}

// sendBrowser: kirim satu frame browser {t, d}.
func sendBrowser(conn *websocket.Conn, t string, d any) error {
	data, err := json.Marshal(map[string]any{"t": t, "d": d})
	if err != nil {
		return err
	}
	return conn.Write(context.Background(), websocket.MessageText, data)
}
