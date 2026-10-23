package panel

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
)

// ---- Eggs CRUD (admin) ----

type eggReq struct {
	NestID        int64  `json:"nest_id"`
	Name          string `json:"name"`
	DockerImage   string `json:"docker_image"`
	StartupCmd    string `json:"startup_cmd"`
	StopSignal    string `json:"stop_signal"`
	InstallScript string `json:"install_script"`
}

func (s *Server) handleEggsList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT id, nest_id, name, docker_image, startup_cmd, stop_signal, install_script, created_at FROM eggs ORDER BY id`)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, nest, created int64
		var name, img, startup, sig, install string
		if err := rows.Scan(&id, &nest, &name, &img, &startup, &sig, &install, &created); err != nil {
			writeErr(w, 500, "scan error")
			return
		}
		out = append(out, map[string]any{
			"id": id, "nest_id": nest, "name": name, "docker_image": img,
			"startup_cmd": startup, "stop_signal": sig, "install_script": install, "created_at": created,
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleEggsCreate(w http.ResponseWriter, r *http.Request) {
	var req eggReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if req.Name == "" || req.DockerImage == "" || req.StartupCmd == "" || req.NestID <= 0 {
		writeErr(w, 422, "nest_id, name, docker_image, startup_cmd required")
		return
	}
	var nestExists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM nests WHERE id=?`, req.NestID).Scan(&nestExists); err != nil || nestExists == 0 {
		writeErr(w, 422, "nest_id does not exist")
		return
	}
	if req.StopSignal == "" {
		req.StopSignal = "SIGTERM"
	}
	res, err := s.DB.Exec(`INSERT INTO eggs (nest_id, name, docker_image, startup_cmd, stop_signal, install_script)
		VALUES (?,?,?,?,?,?)`, req.NestID, req.Name, req.DockerImage, req.StartupCmd, req.StopSignal, req.InstallScript)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	id, _ := res.LastInsertId()
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.eggs.create", "egg="+req.Name)
	writeJSON(w, 201, map[string]any{"id": id, "name": req.Name})
}

func (s *Server) handleEggsGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var nest int64
	var name, img, startup, sig, install string
	var created int64
	err := s.DB.QueryRow(`SELECT nest_id, name, docker_image, startup_cmd, stop_signal, install_script, created_at
		FROM eggs WHERE id=?`, id).Scan(&nest, &name, &img, &startup, &sig, &install, &created)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "egg not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	vars, err := s.eggVariables(id)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	writeJSON(w, 200, map[string]any{
		"id": id, "nest_id": nest, "name": name, "docker_image": img,
		"startup_cmd": startup, "stop_signal": sig, "install_script": install,
		"created_at": created, "variables": vars,
	})
}

func (s *Server) eggVariables(eggID int64) ([]map[string]any, error) {
	rows, err := s.DB.Query(`SELECT id, name, default_value, user_viewable, user_editable, description
		FROM egg_variables WHERE egg_id=? ORDER BY id`, eggID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var n, dv, desc string
		var uv, ue bool
		if err := rows.Scan(&id, &n, &dv, &uv, &ue, &desc); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": id, "name": n, "default_value": dv,
			"user_viewable": uv, "user_editable": ue, "description": desc,
		})
	}
	return out, rows.Err()
}

func (s *Server) handleEggsUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req eggReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	set, args := []string{}, []any{}
	if req.Name != "" {
		set = append(set, "name=?")
		args = append(args, req.Name)
	}
	if req.DockerImage != "" {
		set = append(set, "docker_image=?")
		args = append(args, req.DockerImage)
	}
	if req.StartupCmd != "" {
		set = append(set, "startup_cmd=?")
		args = append(args, req.StartupCmd)
	}
	if req.StopSignal != "" {
		set = append(set, "stop_signal=?")
		args = append(args, req.StopSignal)
	}
	if req.InstallScript != "" {
		set = append(set, "install_script=?")
		args = append(args, req.InstallScript)
	}
	if len(set) == 0 {
		writeErr(w, 400, "nothing to update")
		return
	}
	args = append(args, id)
	if _, err := s.DB.Exec(`UPDATE eggs SET `+stringsJoin(set, ",")+` WHERE id=?`, args...); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.eggs.update", eggIDLog(id))
	writeJSON(w, 200, map[string]string{"ok": "updated"})
}

func (s *Server) handleEggsDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	actor := auth.SessionFrom(r.Context())
	if _, err := s.DB.Exec(`DELETE FROM eggs WHERE id=?`, id); err != nil {
		writeErr(w, 409, "egg still referenced by servers")
		return
	}
	activity.Log(s.DB, actor.UserID, 0, "admin.eggs.delete", eggIDLog(id))
	writeJSON(w, 200, map[string]string{"ok": "deleted"})
}

func eggIDLog(id int64) string { return fmt.Sprintf("egg_id=%d", id) }

// ---- Nests ----

type nestReq struct {
	Name string `json:"name"`
}

func (s *Server) handleNestsList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT id, name FROM nests ORDER BY id`)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var name string
		_ = rows.Scan(&id, &name)
		out = append(out, map[string]any{"id": id, "name": name})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleNestsCreate(w http.ResponseWriter, r *http.Request) {
	var req nestReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if req.Name == "" {
		writeErr(w, 422, "name required")
		return
	}
	res, err := s.DB.Exec(`INSERT INTO nests (name) VALUES (?)`, req.Name)
	if err != nil {
		writeErr(w, 409, "nest name exists")
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, 201, map[string]any{"id": id, "name": req.Name})
}

var _ = chi.URLParam // keep chi import for pathID pattern
