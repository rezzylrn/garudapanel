package panel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"garudapanel/internal/auth"
	"garudapanel/internal/daemonhub"
	"garudapanel/internal/db"
)

func TestHandleNodesListNoDeadlock(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	// Pakai SetMaxOpenConns(1) secara eksplisit untuk membuktikan bahwa
	// handler tidak deadlock bahkan dalam kondisi pool paling ketat (1 koneksi).
	sqlDB.SetMaxOpenConns(1)

	mustExec(t, sqlDB, `INSERT INTO locations (short_code, description) VALUES ('id', 'Indonesia')`)
	mustExec(t, sqlDB, `INSERT INTO nodes (location_id, name, fqdn, memory_limit_mb, disk_limit_mb)
		VALUES (1, 'node-1', 'node1.example.com', 4096, 50000)`)
	mustExec(t, sqlDB, `INSERT INTO nodes (location_id, name, fqdn, memory_limit_mb, disk_limit_mb)
		VALUES (1, 'node-2', 'node2.example.com', 8192, 100000)`)

	hub := daemonhub.NewHub()
	s := &Server{DB: sqlDB, Hub: hub}

	done := make(chan struct{})
	var w *httptest.ResponseRecorder

	go func() {
		defer close(done)
		req := httptest.NewRequest(http.MethodGet, "/api/admin/nodes", nil)
		req = req.WithContext(auth.WithSessionForTest(req.Context(), auth.SessionRow{UserID: 1, IsAdmin: true}))
		w = httptest.NewRecorder()
		s.handleNodesList(w, req)
	}()

	select {
	case <-done:
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var list []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(list))
		}
		if list[0]["name"] != "node-1" || list[1]["name"] != "node-2" {
			t.Fatalf("unexpected nodes: %+v", list)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("DEADLOCK: handleNodesList tidak selesai dalam 5 detik")
	}
}
