package panel

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"garudapanel/internal/activity"
	"garudapanel/internal/auth"
)

// ---- Eggs CRUD (admin) — Pterodactyl-style format ----

// eggReq mirrors the Pterodactyl egg structure: basic info, docker images map,
// startup/stop configuration and install script block.
type eggReq struct {
	NestID            int64             `json:"nest_id"`
	Name              string            `json:"name"`
	Author            string            `json:"author"`
	Description       string            `json:"description"`
	DockerImages      map[string]string `json:"docker_images"`
	Startup           string            `json:"startup"`
	ConfigStartupDone string            `json:"config_startup_done"`
	ConfigStop        string            `json:"config_stop"`
	ConfigFiles       string            `json:"config_files"`
	ConfigLogs        string            `json:"config_logs"`
	InstallScript     string            `json:"install_script"`
	InstallContainer  string            `json:"install_container"`
	InstallEntrypoint string            `json:"install_entrypoint"`
	// Legacy aliases (accepted on write, mapped to the new fields).
	DockerImage string `json:"docker_image"`
	StartupCmd  string `json:"startup_cmd"`
	StopSignal  string `json:"stop_signal"`
}

func (s *Server) handleEggsList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`SELECT id, nest_id, name, author, description, docker_images,
		startup_cmd, config_startup_done, config_stop, install_script, created_at
		FROM eggs ORDER BY id`)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, nest, created int64
		var name, author, desc, images, startup, done, stop, install string
		if err := rows.Scan(&id, &nest, &name, &author, &desc, &images,
			&startup, &done, &stop, &install, &created); err != nil {
			writeErr(w, 500, "scan error")
			return
		}
		out = append(out, eggJSON(id, nest, name, author, desc, images,
			startup, done, stop, install, created))
	}
	writeJSON(w, 200, out)
}

// eggJSON builds the API representation, including legacy aliases so older
// clients keep working (docker_image = first entry of docker_images).
func eggJSON(id, nest int64, name, author, desc, imagesJSON, startup,
	done, stop, install string, created int64) map[string]any {
	images := map[string]string{}
	_ = json.Unmarshal([]byte(imagesJSON), &images)
	firstImage := ""
	for _, v := range images {
		firstImage = v
		break
	}
	return map[string]any{
		"id": id, "nest_id": nest, "name": name, "author": author,
		"description": desc, "docker_images": images,
		"startup": startup, "config_startup_done": done, "config_stop": stop,
		"install_script": install, "created_at": created,
		// legacy aliases
		"docker_image": firstImage, "startup_cmd": startup, "stop_signal": stop,
	}
}

func normalizeEggReq(req *eggReq) {
	// Legacy aliases.
	if len(req.DockerImages) == 0 && req.DockerImage != "" {
		req.DockerImages = map[string]string{"Default": req.DockerImage}
	}
	if req.Startup == "" {
		req.Startup = req.StartupCmd
	}
	if req.ConfigStop == "" {
		if req.StopSignal == "SIGINT" {
			req.ConfigStop = "^C"
		} else if req.StopSignal != "" {
			req.ConfigStop = req.StopSignal
		}
	}
	if req.ConfigStop == "" {
		req.ConfigStop = "^C"
	}
	if req.InstallEntrypoint == "" {
		req.InstallEntrypoint = "ash"
	}
	if req.ConfigFiles == "" {
		req.ConfigFiles = "{}"
	}
	if req.ConfigLogs == "" {
		req.ConfigLogs = "{}"
	}
}

func (s *Server) handleEggsCreate(w http.ResponseWriter, r *http.Request) {
	var req eggReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	normalizeEggReq(&req)
	if req.Name == "" || len(req.DockerImages) == 0 || req.Startup == "" || req.NestID <= 0 {
		writeErr(w, 422, "nest_id, name, docker_images, startup required")
		return
	}
	var nestExists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM nests WHERE id=?`, req.NestID).Scan(&nestExists); err != nil || nestExists == 0 {
		writeErr(w, 422, "nest_id does not exist")
		return
	}
	imagesJSON, _ := json.Marshal(req.DockerImages)
	// Keep legacy columns populated for backward compatibility.
	legacyImage := ""
	for _, v := range req.DockerImages {
		legacyImage = v
		break
	}
	legacyStop := req.ConfigStop
	if legacyStop == "^C" {
		legacyStop = "SIGINT"
	}
	res, err := s.DB.Exec(`INSERT INTO eggs
		(nest_id, name, author, description, docker_image, docker_images, startup_cmd,
		 stop_signal, config_startup_done, config_stop, config_files, config_logs,
		 install_script, install_container, install_entrypoint)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		req.NestID, req.Name, req.Author, req.Description, legacyImage, string(imagesJSON), req.Startup,
		legacyStop, req.ConfigStartupDone, req.ConfigStop, req.ConfigFiles, req.ConfigLogs,
		req.InstallScript, req.InstallContainer, req.InstallEntrypoint)
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
	var name, author, desc, images, startup, done, stop, files, logs, install, container, entrypoint string
	var created int64
	err := s.DB.QueryRow(`SELECT nest_id, name, author, description, docker_images, startup_cmd,
		config_startup_done, config_stop, config_files, config_logs,
		install_script, install_container, install_entrypoint, created_at
		FROM eggs WHERE id=?`, id).Scan(&nest, &name, &author, &desc, &images, &startup,
		&done, &stop, &files, &logs, &install, &container, &entrypoint, &created)
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
	out := eggJSON(id, nest, name, author, desc, images, startup, done, stop, install, created)
	out["config_files"] = files
	out["config_logs"] = logs
	out["install_container"] = container
	out["install_entrypoint"] = entrypoint
	out["variables"] = vars
	writeJSON(w, 200, out)
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
	normalizeEggReq(&req)
	set, args := []string{}, []any{}
	if req.Name != "" {
		set = append(set, "name=?")
		args = append(args, req.Name)
	}
	if req.Author != "" {
		set = append(set, "author=?")
		args = append(args, req.Author)
	}
	if req.Description != "" {
		set = append(set, "description=?")
		args = append(args, req.Description)
	}
	if len(req.DockerImages) > 0 {
		imagesJSON, _ := json.Marshal(req.DockerImages)
		set = append(set, "docker_images=?")
		args = append(args, string(imagesJSON))
	}
	if req.Startup != "" {
		set = append(set, "startup_cmd=?")
		args = append(args, req.Startup)
	}
	if req.ConfigStartupDone != "" {
		set = append(set, "config_startup_done=?")
		args = append(args, req.ConfigStartupDone)
	}
	if req.ConfigStop != "" {
		set = append(set, "config_stop=?")
		args = append(args, req.ConfigStop)
	}
	if req.ConfigFiles != "" {
		set = append(set, "config_files=?")
		args = append(args, req.ConfigFiles)
	}
	if req.ConfigLogs != "" {
		set = append(set, "config_logs=?")
		args = append(args, req.ConfigLogs)
	}
	if req.InstallScript != "" {
		set = append(set, "install_script=?")
		args = append(args, req.InstallScript)
	}
	if req.InstallContainer != "" {
		set = append(set, "install_container=?")
		args = append(args, req.InstallContainer)
	}
	if req.InstallEntrypoint != "" {
		set = append(set, "install_entrypoint=?")
		args = append(args, req.InstallEntrypoint)
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

// ---- Pterodactyl egg.json import ----

// pteroEgg mirrors the PTDL_v2 egg export format from Pterodactyl.
// config.files / config.logs / config.startup accept either JSON strings,
// plain strings or objects — community eggs are inconsistent.
type pteroEgg struct {
	Name         string            `json:"name"`
	Author       string            `json:"author"`
	Description  string            `json:"description"`
	DockerImages map[string]string `json:"docker_images"`
	Startup      string            `json:"startup"`
	Config       struct {
		Files   any `json:"files"`
		Startup any `json:"startup"`
		Logs    any `json:"logs"`
		Stop    string `json:"stop"`
	} `json:"config"`
	Scripts struct {
		Installation struct {
			Script     string `json:"script"`
			Container  string `json:"container"`
			Entrypoint string `json:"entrypoint"`
		} `json:"installation"`
	} `json:"scripts"`
	Variables []struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		EnvVariable  string `json:"env_variable"`
		DefaultValue string `json:"default_value"`
		UserViewable bool   `json:"user_viewable"`
		UserEditable bool   `json:"user_editable"`
	} `json:"variables"`
}

// pteroStartupDone extracts the "done" text from config.startup, which may be
// {"done": "..."}, a plain string, or a JSON-encoded string.
func pteroStartupDone(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		// Might be a JSON-encoded {"done": "..."} — try to decode.
		var obj struct {
			Done string `json:"done"`
		}
		if err := json.Unmarshal([]byte(s), &obj); err == nil && obj.Done != "" {
			return obj.Done
		}
		return s
	}
	if m, ok := v.(map[string]any); ok {
		if done, ok := m["done"].(string); ok {
			return done
		}
	}
	return ""
}

type eggImportReq struct {
	NestID int64           `json:"nest_id"`
	Egg    json.RawMessage `json:"egg"`
}

// asJSONString normalizes a value that may be a JSON string or an object
// into its JSON string representation.
func asJSONString(v any) string {
	if v == nil {
		return "{}"
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return "{}"
		}
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// handleEggsImport imports a Pterodactyl egg.json (PTDL_v2). The frontend
// reads the uploaded file client-side and POSTs {nest_id, egg: {...}}.
func (s *Server) handleEggsImport(w http.ResponseWriter, r *http.Request) {
	var req eggImportReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if req.NestID <= 0 {
		writeErr(w, 422, "nest_id required")
		return
	}
	var nestExists int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM nests WHERE id=?`, req.NestID).Scan(&nestExists); err != nil || nestExists == 0 {
		writeErr(w, 422, "nest_id does not exist")
		return
	}
	var pe pteroEgg
	if err := json.Unmarshal(req.Egg, &pe); err != nil {
		writeErr(w, 422, "egg is not valid JSON: "+err.Error())
		return
	}
	if pe.Name == "" || len(pe.DockerImages) == 0 || pe.Startup == "" {
		writeErr(w, 422, "egg.json must contain name, docker_images and startup")
		return
	}

	imagesJSON, _ := json.Marshal(pe.DockerImages)
	legacyImage := ""
	for _, v := range pe.DockerImages {
		legacyImage = v
		break
	}
	entrypoint := pe.Scripts.Installation.Entrypoint
	if entrypoint == "" {
		entrypoint = "ash"
	}
	stop := pe.Config.Stop
	if stop == "" {
		stop = "^C"
	}
	legacyStop := stop
	if legacyStop == "^C" {
		legacyStop = "SIGINT"
	}

	tx, err := s.DB.Begin()
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO eggs
		(nest_id, name, author, description, docker_image, docker_images, startup_cmd,
		 stop_signal, config_startup_done, config_stop, config_files, config_logs,
		 install_script, install_container, install_entrypoint)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		req.NestID, pe.Name, pe.Author, pe.Description, legacyImage, string(imagesJSON), pe.Startup,
		legacyStop, pteroStartupDone(pe.Config.Startup), stop,
		asJSONString(pe.Config.Files), asJSONString(pe.Config.Logs),
		pe.Scripts.Installation.Script, pe.Scripts.Installation.Container, entrypoint)
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	eggID, _ := res.LastInsertId()

	imported := 0
	for _, v := range pe.Variables {
		envName := v.EnvVariable
		if envName == "" {
			envName = v.Name
		}
		desc := v.Description
		if v.Name != "" && v.Name != envName {
			desc = v.Name + "\n" + desc
		}
		if _, err := tx.Exec(`INSERT INTO egg_variables
			(egg_id, name, default_value, user_viewable, user_editable, description)
			VALUES (?,?,?,?,?,?)`,
			eggID, envName, v.DefaultValue, v.UserViewable, v.UserEditable, desc); err != nil {
			continue
		}
		imported++
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, 500, "db error")
		return
	}
	actor := auth.SessionFrom(r.Context())
	activity.Log(s.DB, actor.UserID, 0, "admin.eggs.import",
		fmt.Sprintf("egg=%s variables=%d", pe.Name, imported))
	writeJSON(w, 201, map[string]any{"id": eggID, "name": pe.Name, "variables": imported})
}
