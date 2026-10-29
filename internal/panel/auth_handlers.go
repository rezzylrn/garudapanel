package panel

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
	"garudapanel/internal/daemonhub"
)

// Server menampung dependensi handler REST panel.
type Server struct {
	Sessions       *auth.SessionStore
	DB             *sql.DB
	Hub            *daemonhub.Hub // koneksi daemon aktif (online/offline)
	MaxUploadBytes int64          // batas ukuran upload (default 100MB, configurable via flag/env)
}

// ---- helper ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20)) // 1MB max body
	// Unknown fields are IGNORED (Pterodactyl-compatible payloads carry
	// extra fields like env_variable/rules); type errors still fail.
	return dec.Decode(dst)
}

// clientIP mengambil IP dari X-Forwarded-For bila ada, fallback RemoteAddr.
func clientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---- auth handlers ----

type loginReq struct {
	Username   string `json:"username"`
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// Login: POST /api/auth/login — rate limit 5/menit/IP (SPEC §8).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	ident := req.Username
	if ident == "" {
		ident = req.Identifier
	}
	ip := clientIP(r)
	if !loginLimiter.Allow(ip) {
		writeErr(w, http.StatusTooManyRequests, "too many login attempts, wait a minute")
		return
	}
	var (
		id      int64
		hash    string
		isAdmin bool
	)
	err := s.DB.QueryRow(`SELECT id, password_hash, is_admin FROM users WHERE username = ? OR email = ?`,
		ident, ident).Scan(&id, &hash, &isAdmin)
	if err == sql.ErrNoRows || (err == nil && !auth.VerifyPassword(req.Password, hash)) {
		// hashing tetap dijalankan untuk anti timing oracle saat user tidak ada
		if err == sql.ErrNoRows {
			auth.VerifyPassword(req.Password, dummyHash)
		}
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		log.Printf("login: db query user error for identifier=%q: %v", ident, err)
		writeErr(w, http.StatusInternalServerError, "db error")
		return
	}
	token, err := s.Sessions.Create(id, ip, r.UserAgent())
	if err != nil {
		log.Printf("login: session create failed for user_id=%d: %v", id, err)
		writeErr(w, http.StatusInternalServerError, "session create failed")
		return
	}
	activity.Log(s.DB, id, 0, "auth.login", "ip="+ip)
	// cookie juga diset agar frontend embed bisa pakai cookie; API tetap support Bearer.
	http.SetCookie(w, &http.Cookie{
		Name: "garuda_session", Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int((24 * time.Hour * 7).Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "is_admin": isAdmin})
}

var dummyHash = "$argon2id$v=19$m=65536,t=3,p=2$MDAwMDAwMDAwMDAwMDAwMA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// Logout: revoke session aktif.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	raw := auth.TokenFrom(r.Context())
	if err := s.Sessions.Revoke(raw); err != nil {
		log.Printf("logout: session revoke failed: %v", err)
		writeErr(w, http.StatusInternalServerError, "revoke failed")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "garuda_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"ok": "logged out"})
}

// Me: info user yang sedang login.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess := auth.SessionFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": sess.UserID, "username": sess.Username, "is_admin": sess.IsAdmin,
	})
}

// ---- middleware ----

// limiter: sederhana per-IP sliding window (5/menit), thread-safe.
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

const loginLimit = 5

func newLimiter() *limiter { return &limiter{hits: map[string][]time.Time{}} }

func (l *limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	window := now.Add(-time.Minute)
	fresh := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(window) {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) >= loginLimit {
		l.hits[key] = fresh
		return false
	}
	l.hits[key] = append(fresh, now)
	// GC ringan: kalau map membengkak, hapus entri kosong
	if len(l.hits) > 10000 {
		for k, v := range l.hits {
			if len(v) == 0 || allBefore(v, window) {
				delete(l.hits, k)
			}
		}
	}
	return true
}

func allBefore(ts []time.Time, cutoff time.Time) bool {
	for _, t := range ts {
		if t.After(cutoff) {
			return false
		}
	}
	return true
}

var loginLimiter = newLimiter()

// RequestLogger: log ringkas tiap request (skip static).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if !strings.HasPrefix(r.URL.Path, "/assets") {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

// jsonBody: middleware pembatas ukuran body 1MB untuk request JSON.
// Request upload file (/files PUT) dilewatkan karena di-handle dengan batas upload tersendiri.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/files") {
			next.ServeHTTP(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}
