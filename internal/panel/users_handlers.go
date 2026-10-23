package panel

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
)

// ---- Users CRUD (admin) ----

type userReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"is_admin"`
}

func (s *Server) handleUsersList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT id, username, email, is_admin, created_at FROM users ORDER BY id`)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, created int64
		var u, e string
		var admin bool
		if err := rows.Scan(&id, &u, &e, &admin, &created); err != nil {
			writeErr(w, 500, "scan error")
			return
		}
		out = append(out, map[string]any{"id": id, "username": u, "email": e, "is_admin": admin, "created_at": created})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleUsersCreate(w http.ResponseWriter, r *http.Request) {
	var req userReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if err := validateUserReq(req); err != nil {
		writeErr(w, 422, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, 500, "hash failed")
		return
	}
	adminInt := 0
	if req.IsAdmin {
		adminInt = 1
	}
	res, err := s.DB.Exec(`INSERT INTO users (username, email, password_hash, is_admin) VALUES (?,?,?,?)`,
		req.Username, req.Email, hash, adminInt)
	if err != nil {
		writeErr(w, 409, "username or email already exists")
		return
	}
	id, _ := res.LastInsertId()
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.users.create", "user="+req.Username)
	writeJSON(w, 201, map[string]any{"id": id, "username": req.Username, "email": req.Email, "is_admin": req.IsAdmin})
}

func (s *Server) handleUsersGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var u, e string
	var admin bool
	var created int64
	err := s.DB.QueryRow(`SELECT username, email, is_admin, created_at FROM users WHERE id=?`, id).
		Scan(&u, &e, &admin, &created)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "user not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "username": u, "email": e, "is_admin": admin, "created_at": created})
}

func (s *Server) handleUsersUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req userReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	// Field opsional: kalau password kosong, jangan ganti.
	if req.Password != "" {
		if len(req.Password) < 10 {
			writeErr(w, 422, "password too short (min 10)")
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeErr(w, 500, "hash failed")
			return
		}
		if _, err := s.DB.Exec(`UPDATE users SET password_hash=? WHERE id=?`, hash, id); err != nil {
			writeErr(w, 500, "db error")
			return
		}
		// revoke semua session user lama setelah ganti password (SEC).
		_ = s.Sessions.RevokeAllFor(id)
	}
	if req.Username != "" || req.Email != "" {
		set, args := []string{}, []any{}
		if req.Username != "" {
			set = append(set, "username=?")
			args = append(args, req.Username)
		}
		if req.Email != "" {
			set = append(set, "email=?")
			args = append(args, req.Email)
		}
		args = append(args, id)
		if _, err := s.DB.Exec(`UPDATE users SET `+strings.Join(set, ",")+` WHERE id=?`, args...); err != nil {
			writeErr(w, 409, "username or email already taken")
			return
		}
	}
	if req.IsAdmin {
		if _, err := s.DB.Exec(`UPDATE users SET is_admin=1 WHERE id=?`, id); err != nil {
			writeErr(w, 500, "db error")
			return
		}
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.users.update", fmt.Sprintf("user_id=%d", id))
	writeJSON(w, 200, map[string]string{"ok": "updated"})
}

func (s *Server) handleUsersDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	actor := auth.SessionFrom(r.Context())
	if actor.UserID == id {
		writeErr(w, 400, "cannot delete yourself")
		return
	}
	res, err := s.DB.Exec(`DELETE FROM users WHERE id=?`, id)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 404, "user not found")
		return
	}
	activity.Log(s.DB, actor.UserID, 0, "admin.users.delete", fmt.Sprintf("user_id=%d", id))
	writeJSON(w, 200, map[string]string{"ok": "deleted"})
}

func validateUserReq(req userReq) error {
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return fmt.Errorf("username must be 3-32 chars")
	}
	if !strings.Contains(req.Email, "@") || len(req.Email) > 255 {
		return fmt.Errorf("invalid email")
	}
	if len(req.Password) < 10 {
		return fmt.Errorf("password too short (min 10)")
	}
	return nil
}

// pathID mengambil :id dari chi URL param.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "invalid id")
		return 0, false
	}
	return id, true
}
