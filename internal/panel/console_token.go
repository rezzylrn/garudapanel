// Console panel: WS browser↔panel (SPEC §4).
//
// Protokol browser↔panel:
//
//	Client→server: {t:"auth", token}, {t:"in", d:"..."}, {t:"resize", cols, rows}
//	Server→client: {t:"out", d:"..."}, {t:"stat", d:{...}}, {t:"status", s:"..."}
//
// Keamanan: token WS sekali pakai, expiry 60 detik (dibuat via REST
// POST /api/servers/:id/console-token). Maks 30 sesi per server.
// Throttle output 2000 baris/100ms — pump goroutine memotong burst.
package panel

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"garudapanel/internal/auth"
)

const (
	consoleTokenTTL    = 60 * time.Second
	maxConsoleSessions = 30
	throttleWindow     = 100 * time.Millisecond
	throttleMaxLines   = 2000
)

// ---- token console sekali pakai ----

type wsToken struct {
	serverID  int64
	userID    int64
	expiresAt int64
}

// consoleTokens: registry token sekali pakai (in-memory — expiry pendek;
// hilang saat panel restart tidak masalah, user minta token baru).
var consoleTokens = struct {
	mu     sync.Mutex
	tokens map[string]*wsToken
}{tokens: map[string]*wsToken{}}

// issueConsoleToken buat token WS console sekali pakai.
func issueConsoleToken(serverID, userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	consoleTokens.mu.Lock()
	// sambil di sini bersihkan token kadaluarsa biar map tidak membengkak
	now := time.Now().Unix()
	for k, t := range consoleTokens.tokens {
		if now >= t.expiresAt {
			delete(consoleTokens.tokens, k)
		}
	}
	consoleTokens.tokens[raw] = &wsToken{
		serverID:  serverID,
		userID:    userID,
		expiresAt: now + int64(consoleTokenTTL.Seconds()),
	}
	consoleTokens.mu.Unlock()
	return raw, nil
}

// consumeConsoleToken: atomik sekali-pakai + cek expiry. Nil bila invalid.
func consumeConsoleToken(raw string) *wsToken {
	consoleTokens.mu.Lock()
	defer consoleTokens.mu.Unlock()
	t, ok := consoleTokens.tokens[raw]
	if !ok || time.Now().Unix() >= t.expiresAt {
		if ok {
			delete(consoleTokens.tokens, raw) // expired: buang
		}
		return nil
	}
	delete(consoleTokens.tokens, raw) // sekali pakai
	return t
}

// handleConsoleToken: POST /api/servers/:id/console-token
// Session auth; token balikan dipakai SEKALI oleh WS console (60 detik).
func (s *Server) handleConsoleToken(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sess := auth.SessionFrom(r.Context())
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
	if !auth.IsOwnerOrAdmin(sess, ownerID) {
		writeErr(w, 403, "not owner")
		return
	}
	raw, err := issueConsoleToken(id, sess.UserID)
	if err != nil {
		writeErr(w, 500, "token gen failed")
		return
	}
	writeJSON(w, 200, map[string]any{"token": raw, "expires_in": int(consoleTokenTTL.Seconds())})
}
