// Test egg variables: user_editable gate + user_viewable filter + RBAC
// (owner/admin; non-owner 403) — pola power endpoint (SPEC §10).
// Handler-level test pakai chi router + SQLite in-memory.
package panel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"garudapanel/internal/auth"
	"garudapanel/internal/daemonhub"
	"garudapanel/internal/db"
)

// newVarTestServer: panel Server dengan DB in-memory + data seed.
func newVarTestServer(t *testing.T) (*Server, int64, int64) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	s := &Server{DB: sqlDB}

	// seed owner + admin + nest + egg + server + variables
	mustExec(t, sqlDB, `INSERT INTO users (username, email, password_hash, is_admin) VALUES ('owner','o@x.io','x',0)`)
	ownerID := lastID(t, sqlDB, "users")
	mustExec(t, sqlDB, `INSERT INTO users (username, email, password_hash, is_admin) VALUES ('admin','a@x.io','x',1)`)
	mustExec(t, sqlDB, `INSERT INTO locations (short_code) VALUES ('sg')`)
	mustExec(t, sqlDB, `INSERT INTO nodes (location_id, name, fqdn) VALUES (1, 'n1', 'n1.x.io')`)
	mustExec(t, sqlDB, `INSERT INTO nests (name) VALUES ('minecraft')`)
	eggID := lastID(t, sqlDB, "nests") // egg id di bawah
	mustExec(t, sqlDB, `INSERT INTO eggs (nest_id, name, docker_image, startup_cmd) VALUES (?, 'mc', 'img:latest', 'sh')`, eggID)
	eggID = lastID(t, sqlDB, "eggs")
	mustExec(t, sqlDB, `INSERT INTO allocations (node_id, ip, port) VALUES (1, '10.0.0.1', 25565)`)
	mustExec(t, sqlDB, `INSERT INTO servers (uuid, name, owner_id, node_id, egg_id, allocation_id, memory_mb, disk_mb)
		VALUES ('uuid-var', 'srv', ?, 1, ?, 1, 1024, 1024)`, ownerID, eggID)
	serverID := lastID(t, sqlDB, "servers")

	seedVar(t, sqlDB, eggID, "MOTD", "halo", true, true)   // viewable + editable
	seedVar(t, sqlDB, eggID, "SECRET", "abc", true, false) // viewable only
	seedVar(t, sqlDB, eggID, "HIDDEN", "x", false, false)  // hidden
	s.Hub = daemonhub.NewHub()
	return s, ownerID, serverID
}

// ctxSession: injeksikan session ke request context — pakai helper auth
// (withSession unexported; kita pakai middleware RequireAuth approach:
// buat token sesi nyata via SessionStore lewat seedUser + login token).

// runVarRoute: panggil handler dengan chi URL param + session context.
// Session diinjeksi manual memakai context key sama dengan auth middleware.
func runVarRoute(t *testing.T, h http.HandlerFunc, method, pattern, path string, sess auth.SessionRow, body ...string) *httptest.ResponseRecorder {
	t.Helper()
	rctx := chi.NewRouteContext()
	if !chiMatch(pattern, path, rctx) {
		t.Fatalf("route %s tidak match %s", pattern, path)
	}
	bodyStr := `{"value":"20"}`
	if len(body) > 0 {
		bodyStr = body[0]
	}
	req := httptest.NewRequest(method, path, strings.NewReader(bodyStr))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(auth.WithSessionForTest(req.Context(), sess))
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

// chiMatch: cocokkan pattern chi {param} dengan path (abaikan query string).
func chiMatch(pattern, path string, rctx *chi.Context) bool {
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	xp := strings.Split(strings.Trim(path, "/"), "/")
	if len(pp) != len(xp) {
		return false
	}
	for i, seg := range pp {
		if strings.HasPrefix(seg, "{") {
			rctx.URLParams.Add(strings.Trim(seg, "{}"), xp[i])
			continue
		}
		if seg != xp[i] {
			return false
		}
	}
	return true
}

func TestServerVariableSetEditableGate(t *testing.T) {
	s, ownerID, serverID := newVarTestServer(t)

	// owner ubah variable EDITABLE → 200
	_ = s
	_ = ownerID
	w := runVarRoute(t, s.handleServerVariableSet, "PATCH",
		"/api/servers/{id}/variables/{name}",
		fmt.Sprintf("/api/servers/%d/variables/MOTD", serverID),
		auth.SessionRow{UserID: ownerID})
	if w.Code != 200 {
		t.Fatalf("set editable: %d %s", w.Code, w.Body.String())
	}

	// owner ubah variable NON-editable (SECRET) → 403
	w = runVarRoute(t, s.handleServerVariableSet, "PATCH",
		"/api/servers/{id}/variables/{name}",
		fmt.Sprintf("/api/servers/%d/variables/SECRET", serverID),
		auth.SessionRow{UserID: ownerID})
	if w.Code != 403 {
		t.Fatalf("set non-editable harus 403, got %d %s", w.Code, w.Body.String())
	}

	// variable tidak ada → 404
	w = runVarRoute(t, s.handleServerVariableSet, "PATCH",
		"/api/servers/{id}/variables/{name}",
		fmt.Sprintf("/api/servers/%d/variables/TIDAKADA", serverID),
		auth.SessionRow{UserID: ownerID})
	if w.Code != 404 {
		t.Fatalf("variable tidak ada: %d, want 404", w.Code)
	}
}

func TestServerVariablesGetViewableOnly(t *testing.T) {
	s, ownerID, serverID := newVarTestServer(t)
	w := runVarRoute(t, s.handleServerVariablesGet, "GET",
		"/api/servers/{id}/variables",
		fmt.Sprintf("/api/servers/%d/variables", serverID),
		auth.SessionRow{UserID: ownerID})
	if w.Code != 200 {
		t.Fatalf("get: %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "MOTD") || !strings.Contains(body, "SECRET") {
		t.Fatalf("harus ada MOTD & SECRET: %s", body)
	}
	if strings.Contains(body, "HIDDEN") {
		t.Fatalf("HIDDEN (non-viewable) tidak boleh muncul: %s", body)
	}
}

func TestServerVariableNonOwner403(t *testing.T) {
	s, _, serverID := newVarTestServer(t)
	// user lain (bukan owner, bukan admin) → 403
	w := runVarRoute(t, s.handleServerVariablesGet, "GET",
		"/api/servers/{id}/variables",
		fmt.Sprintf("/api/servers/%d/variables", serverID),
		auth.SessionRow{UserID: 999})
	if w.Code != 403 {
		t.Fatalf("non-owner harus 403, got %d", w.Code)
	}
	// admin boleh
	w = runVarRoute(t, s.handleServerVariablesGet, "GET",
		"/api/servers/{id}/variables",
		fmt.Sprintf("/api/servers/%d/variables", serverID),
		auth.SessionRow{UserID: 999, IsAdmin: true})
	if w.Code != 200 {
		t.Fatalf("admin harus 200, got %d", w.Code)
	}
}

func TestAdminEggVariableCreate(t *testing.T) {
	s, _, _ := newVarTestServer(t)
	// cari egg id dari DB
	var eggID int64
	if err := s.DB.QueryRow(`SELECT id FROM eggs LIMIT 1`).Scan(&eggID); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"NEW_VAR","default_value":"1","user_viewable":true,"user_editable":true,"description":"d"}`
	w := runVarRoute(t, s.handleEggVariablesCreate, "POST",
		"/api/admin/eggs/{id}/variables",
		fmt.Sprintf("/api/admin/eggs/%d/variables", eggID),
		auth.SessionRow{UserID: 1, IsAdmin: true}, body)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	// duplikat → 409
	w = runVarRoute(t, s.handleEggVariablesCreate, "POST",
		"/api/admin/eggs/{id}/variables",
		fmt.Sprintf("/api/admin/eggs/%d/variables", eggID),
		auth.SessionRow{UserID: 1, IsAdmin: true}, body)
	if w.Code != 409 {
		t.Fatalf("duplikat harus 409, got %d", w.Code)
	}
}

// ---- helpers seed ----
// (mustExec & lastID ada di varseed_test.go)
