// Egg variables (milestone 3, SPEC §3/§10):
//   - Admin: CRUD variable di egg — /api/admin/eggs/{id}/variables[/{vid}]
//   - User : GET/PATCH /api/servers/{id}/variables[/{name}] — LIHAT hanya
//     variable user_viewable; UBAH hanya user_editable (validasi server-side).
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

// pathIDNamed: ambil chi URL param bernama sebagai int64 > 0.
func pathIDNamed(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	idStr := chi.URLParam(r, name)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "invalid "+name)
		return 0, false
	}
	return id, true
}

// varNameValid: nama variable aman (A-Z a-z 0-9 _ -).
func varNameValid(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, ch := range name {
		ok := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' || ch == '-'
		if !ok {
			return false
		}
	}
	return true
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- admin CRUD ----

// handleEggVariablesCreate: POST /api/admin/eggs/{id}/variables
func (s *Server) handleEggVariablesCreate(w http.ResponseWriter, r *http.Request) {
	eggID, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Name         string `json:"name"`
		DefaultValue string `json:"default_value"`
		UserViewable bool   `json:"user_viewable"`
		UserEditable bool   `json:"user_editable"`
		Description  string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if !varNameValid(req.Name) {
		writeErr(w, 422, "invalid variable name (A-Z a-z 0-9 _ - , maks 64)")
		return
	}
	var cnt int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM eggs WHERE id=?`, eggID).Scan(&cnt); err != nil || cnt == 0 {
		writeErr(w, 404, "egg not found")
		return
	}
	res, err := s.DB.Exec(`INSERT INTO egg_variables (egg_id, name, default_value, user_viewable, user_editable, description)
		VALUES (?,?,?,?,?,?)`,
		eggID, req.Name, req.DefaultValue, boolInt(req.UserViewable), boolInt(req.UserEditable), req.Description)
	if err != nil {
		writeErr(w, 409, "variable name already exists on this egg")
		return
	}
	varID, _ := res.LastInsertId()
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.egg_variables.create",
		fmt.Sprintf("egg_id=%d name=%s", eggID, req.Name))
	writeJSON(w, 201, map[string]any{"id": varID, "name": req.Name})
}

// handleEggVariableUpdate: PATCH /api/admin/eggs/{eggid}/variables/{vid}
func (s *Server) handleEggVariableUpdate(w http.ResponseWriter, r *http.Request) {
	eggID, ok := pathID(w, r)
	if !ok {
		return
	}
	vid, ok := pathIDNamed(w, r, "vid")
	if !ok {
		return
	}
	var req struct {
		Name         *string `json:"name"`
		DefaultValue *string `json:"default_value"`
		UserViewable *bool   `json:"user_viewable"`
		UserEditable *bool   `json:"user_editable"`
		Description  *string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	set, args := []string{}, []any{}
	if req.Name != nil {
		if !varNameValid(*req.Name) {
			writeErr(w, 422, "invalid variable name")
			return
		}
		set = append(set, "name=?")
		args = append(args, *req.Name)
	}
	if req.DefaultValue != nil {
		set = append(set, "default_value=?")
		args = append(args, *req.DefaultValue)
	}
	if req.UserViewable != nil {
		set = append(set, "user_viewable=?")
		args = append(args, boolInt(*req.UserViewable))
	}
	if req.UserEditable != nil {
		set = append(set, "user_editable=?")
		args = append(args, boolInt(*req.UserEditable))
	}
	if req.Description != nil {
		set = append(set, "description=?")
		args = append(args, *req.Description)
	}
	if len(set) == 0 {
		writeErr(w, 400, "nothing to update")
		return
	}
	args = append(args, vid, eggID)
	if _, err := s.DB.Exec(`UPDATE egg_variables SET `+strings.Join(set, ",")+
		` WHERE id=? AND egg_id=?`, args...); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.egg_variables.update", fmt.Sprintf("egg_id=%d var_id=%d", eggID, vid))
	writeJSON(w, 200, map[string]string{"ok": "updated"})
}

// handleEggVariableDelete: DELETE /api/admin/eggs/{eggID}/variables/{vid}
func (s *Server) handleEggVariableDelete(w http.ResponseWriter, r *http.Request) {
	eggID, ok := pathID(w, r)
	if !ok {
		return
	}
	vid, ok := pathIDNamed(w, r, "vid")
	if !ok {
		return
	}
	res, err := s.DB.Exec(`DELETE FROM egg_variables WHERE id=? AND egg_id=?`, vid, eggID)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 404, "variable not found")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.egg_variables.delete", fmt.Sprintf("egg_id=%d var_id=%d", eggID, vid))
	writeJSON(w, 200, map[string]string{"ok": "deleted"})
}

// ---- user endpoints (server variables) ----

// handleServerVariablesGet: GET /api/servers/{id}/variables
// Hanya variable user_viewable yang ditampilkan.
func (s *Server) handleServerVariablesGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var ownerID, eggID int64
	err := s.DB.QueryRow(`SELECT owner_id, egg_id FROM servers WHERE id=?`, id).
		Scan(&ownerID, &eggID)
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
	rows, err := s.DB.Query(`SELECT v.name, COALESCE(sv.value, v.default_value), v.default_value,
		v.user_editable, v.description
		FROM egg_variables v
		LEFT JOIN server_variables sv ON sv.variable_id = v.id AND sv.server_id = ?
		WHERE v.egg_id = ? AND v.user_viewable = 1 ORDER BY v.id`, id, eggID)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var name, val, def, desc string
		var editable bool
		if err := rows.Scan(&name, &val, &def, &editable, &desc); err != nil {
			continue
		}
		out = append(out, map[string]any{
			"name": name, "value": val, "default_value": def,
			"user_editable": editable, "description": desc,
		})
	}
	writeJSON(w, 200, map[string]any{"variables": out})
}

// handleServerVariableSet: PATCH /api/servers/{id}/variables/{name} {value}
// Validasi user_editable — variable non-editable DITOLAK (403).
func (s *Server) handleServerVariableSet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	name := chi.URLParam(r, "name")
	if !varNameValid(name) {
		writeErr(w, 422, "invalid variable name")
		return
	}
	var ownerID, eggID int64
	err := s.DB.QueryRow(`SELECT owner_id, egg_id FROM servers WHERE id=?`, id).
		Scan(&ownerID, &eggID)
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
	var req struct {
		Value string `json:"value"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	// variable harus ada di egg DAN user_editable=1
	var vid int64
	var editable bool
	err = s.DB.QueryRow(`SELECT id, user_editable FROM egg_variables WHERE egg_id=? AND name=?`, eggID, name).
		Scan(&vid, &editable)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "variable not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	if !editable {
		writeErr(w, 403, "variable is not user-editable")
		return
	}
	if _, err := s.DB.Exec(`INSERT INTO server_variables (server_id, variable_id, value) VALUES (?,?,?)
		ON CONFLICT(server_id, variable_id) DO UPDATE SET value=excluded.value`, id, vid, req.Value); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, id, "server.variable.set", name)
	writeJSON(w, 200, map[string]string{"ok": "updated"})
}
