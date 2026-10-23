// Package auth: argon2id password hashing, session token stateful revocable,
// dan middleware RBAC (admin/owner) untuk net/http.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// ---- Password hashing (argon2id, off-thread via semaphore) ----

const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

var hashSem = make(chan struct{}, 2) // batasi paralelisme hashing

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hashSem <- struct{}{}
	defer func() { <-hashSem }()
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var v, m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	if v != argon2.Version {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) != argonKeyLen {
		return false
	}
	hashSem <- struct{}{}
	defer func() { <-hashSem }()
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ---- Token helpers: raw token dibagikan, hanya SHA-256 hash disimpan ----

func NewToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, TokenHash(raw), nil
}

func TokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ---- Session store (stateful, revocable) ----

var ErrInvalidToken = errors.New("invalid or expired session token")

type SessionRow struct {
	TokenHash string
	UserID    int64
	IsAdmin   bool
	Username  string
	ExpiresAt time.Time
}

// SessionStore menyimpan & memvalidasi session di DB (revocable).
type SessionStore struct {
	DB      *sql.DB
	LifeTTL time.Duration
}

type querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{DB: db, LifeTTL: 24 * 7 * time.Hour}
}

// Create membuat session baru untuk user, return raw token (hanya tampil sekali).
func (s *SessionStore) Create(userID int64, ip, userAgent string) (string, error) {
	raw, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	exp := time.Now().Add(s.LifeTTL)
	_, err = s.DB.Exec(`INSERT INTO sessions (token_hash, user_id, expires_at, ip, user_agent) VALUES (?,?,?,?,?)`,
		hash, userID, exp.Unix(), ip, userAgent)
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (s *SessionStore) Validate(raw string) (SessionRow, error) {
	var sess SessionRow
	var expires int64
	err := s.DB.QueryRow(`SELECT s.token_hash, s.user_id, s.expires_at, u.is_admin, u.username
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ?`,
		TokenHash(raw)).Scan(&sess.TokenHash, &sess.UserID, &expires, &sess.IsAdmin, &sess.Username)
	if err == sql.ErrNoRows {
		return sess, ErrInvalidToken
	}
	if err != nil {
		return sess, err
	}
	if time.Now().Unix() >= expires {
		// expired: revoke langsung, jangan beri akses.
		_, _ = s.DB.Exec(`DELETE FROM sessions WHERE token_hash = ?`, sess.TokenHash)
		return sess, ErrInvalidToken
	}
	sess.ExpiresAt = time.Unix(expires, 0)
	return sess, nil
}

// Revoke menghapus satu session (logout).
func (s *SessionStore) Revoke(raw string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token_hash = ?`, TokenHash(raw))
	return err
}

// RevokeAllFor menghapus semua session milik seorang user.
func (s *SessionStore) RevokeAllFor(userID int64) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// ---- HTTP middleware: context auth + RBAC ----

type ctxKey int

const ctxSession ctxKey = 0
const ctxToken ctxKey = 1

// WithToken menaruh raw token di context (untuk logout).
func WithToken(ctx context.Context, raw string) context.Context {
	return context.WithValue(ctx, ctxToken, raw)
}

// TokenFrom mengambil raw token dari context.
func TokenFrom(ctx context.Context) string {
	tok, _ := ctx.Value(ctxToken).(string)
	return tok
}

const bearerPrefix = "Bearer "

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, bearerPrefix) {
		return strings.TrimPrefix(h, bearerPrefix)
	}
	// fallback cookie untuk frontend embed.
	if c, err := r.Cookie("garuda_session"); err == nil {
		return c.Value
	}
	return ""
}

// RequireAuth memvalidasi session token lalu menyuntik SessionRow ke context.
func (s *SessionStore) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		if raw == "" || !utf8.ValidString(raw) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid token"})
			return
		}
		sess, err := s.Validate(raw)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired session"})
			return
		}
		ctx := withSession(r.Context(), sess)
		next.ServeHTTP(w, r.WithContext(WithToken(ctx, raw)))
	})
}

// RequireAdmin hanya mengizinkan user is_admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := SessionFrom(r.Context())
		if sess == nil || !sess.IsAdmin {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// IsOwnerOrAdmin: owner-only resource boleh diakses pemiliknya atau admin.
func IsOwnerOrAdmin(sess *SessionRow, ownerID int64) bool {
	if sess == nil {
		return false
	}
	return sess.IsAdmin || sess.UserID == ownerID
}

// ---- context helpers (dipisah biar bisa dites) ----

func withSession(ctx context.Context, s SessionRow) context.Context {
	return context.WithValue(ctx, ctxSession, s)
}

// WithSessionForTest: injeksi SessionRow langsung ke context — HANYA untuk
// unit test handler yang mengemulasi middleware RequireAuth.
func WithSessionForTest(ctx context.Context, s SessionRow) context.Context {
	return withSession(ctx, s)
}

// SessionFrom mengambil session aktif dari request context (nil kalau tidak ada).
func SessionFrom(ctx context.Context) *SessionRow {
	s, _ := ctx.Value(ctxSession).(SessionRow)
	return &s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode: %v", err)
	}
}
