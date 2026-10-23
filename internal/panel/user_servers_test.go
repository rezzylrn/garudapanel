package panel

import (
	"encoding/json"
	"fmt"
	"testing"

	"garudapanel/internal/auth"
)

func TestUserServerEndpoints(t *testing.T) {
	s, ownerID, serverID := newVarTestServer(t)

	// 1. GET /api/servers (sebagai owner)
	w := runVarRoute(t, s.handleUserServersList, "GET", "/api/servers", "/api/servers",
		auth.SessionRow{UserID: ownerID, IsAdmin: false})
	if w.Code != 200 {
		t.Fatalf("handleUserServersList owner expected 200, got %d", w.Code)
	}
	var srvList []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &srvList); err != nil {
		t.Fatalf("json unmarshal servers: %v", err)
	}
	if len(srvList) != 1 {
		t.Fatalf("expected 1 server, got %d", len(srvList))
	}
	if srvList[0]["name"] != "srv" {
		t.Fatalf("expected srv, got %v", srvList[0]["name"])
	}

	// 2. GET /api/servers/{id} (sebagai non-owner -> 403)
	w = runVarRoute(t, s.handleServerUserGet, "GET", "/api/servers/{id}", fmt.Sprintf("/api/servers/%d", serverID),
		auth.SessionRow{UserID: 999, IsAdmin: false})
	if w.Code != 403 {
		t.Fatalf("expected 403 for non-owner, got %d", w.Code)
	}

	// 3. GET /api/servers/{id} (sebagai owner -> 200)
	w = runVarRoute(t, s.handleServerUserGet, "GET", "/api/servers/{id}", fmt.Sprintf("/api/servers/%d", serverID),
		auth.SessionRow{UserID: ownerID, IsAdmin: false})
	if w.Code != 200 {
		t.Fatalf("expected 200 for owner, got %d", w.Code)
	}

	// 4. GET /api/servers/{id}/activity
	w = runVarRoute(t, s.handleServerActivity, "GET", "/api/servers/{id}/activity", fmt.Sprintf("/api/servers/%d/activity", serverID),
		auth.SessionRow{UserID: ownerID, IsAdmin: false})
	if w.Code != 200 {
		t.Fatalf("expected 200 for activity, got %d", w.Code)
	}

	// 5. GET /api/servers/{id}/allocations
	w = runVarRoute(t, s.handleServerAllocationsList, "GET", "/api/servers/{id}/allocations", fmt.Sprintf("/api/servers/%d/allocations", serverID),
		auth.SessionRow{UserID: ownerID, IsAdmin: false})
	if w.Code != 200 {
		t.Fatalf("expected 200 for allocations, got %d", w.Code)
	}
	var allocs []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &allocs); err != nil {
		t.Fatalf("json unmarshal allocs: %v", err)
	}
	// Di newVarTestServer, allocation 1 di-assign ke server 1
	mustExec(t, s.DB, `UPDATE allocations SET server_id=? WHERE id=1`, serverID)
	w = runVarRoute(t, s.handleServerAllocationsList, "GET", "/api/servers/{id}/allocations", fmt.Sprintf("/api/servers/%d/allocations", serverID),
		auth.SessionRow{UserID: ownerID, IsAdmin: false})
	_ = json.Unmarshal(w.Body.Bytes(), &allocs)
	if len(allocs) != 1 || allocs[0]["is_primary"] != true {
		t.Fatalf("expected 1 primary alloc, got %+v", allocs)
	}
}
