package daemonhub

import (
	"encoding/json"
	"testing"
	"time"

	"garudapanel/internal/db"
)

func TestServersSyncPayloadNoDeadlock(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	// SetMaxOpenConns(1) untuk memastikan pola collect-then-enrich tidak deadlock
	// pada 1 koneksi SQLite.
	sqlDB.SetMaxOpenConns(1)

	// Seed user, location, node, nest, egg, allocation, server, variable
	if _, err := sqlDB.Exec(`INSERT INTO users (username, email, password_hash, is_admin) VALUES ('u1', 'u1@test.local', 'hash', 0)`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO locations (short_code, description) VALUES ('loc1', 'Location 1')`); err != nil {
		t.Fatalf("seed loc: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO nodes (location_id, name, fqdn) VALUES (1, 'node-1', 'node1.test.local')`); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO nests (name) VALUES ('nest-1')`); err != nil {
		t.Fatalf("seed nest: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO eggs (nest_id, name, docker_image, startup_cmd, stop_signal) VALUES (1, 'egg-1', 'alpine:latest', 'echo 1', 'SIGTERM')`); err != nil {
		t.Fatalf("seed egg: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO allocations (node_id, ip, port) VALUES (1, '127.0.0.1', 8080)`); err != nil {
		t.Fatalf("seed alloc: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO servers (uuid, name, owner_id, node_id, egg_id, allocation_id, memory_mb, disk_mb, cpu_percent, status)
		VALUES ('srv-uuid-1', 'srv-1', 1, 1, 1, 1, 1024, 2048, 100, 'offline')`); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO egg_variables (egg_id, name, default_value, user_viewable, user_editable, description)
		VALUES (1, 'SERVER_NAME', 'DefaultServer', 1, 1, 'Server Name')`); err != nil {
		t.Fatalf("seed egg var: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO server_variables (server_id, variable_id, value) VALUES (1, 1, 'MyCustomServer')`); err != nil {
		t.Fatalf("seed server var: %v", err)
	}

	done := make(chan struct{})
	var payload []byte

	go func() {
		defer close(done)
		payload = ServersSyncPayload(sqlDB, 1)
	}()

	select {
	case <-done:
		if payload == nil {
			t.Fatal("expected payload, got nil")
		}
		var parsed struct {
			Servers []struct {
				UUID string   `json:"uuid"`
				Port int      `json:"port"`
				Env  []string `json:"env"`
			} `json:"servers"`
		}
		if err := json.Unmarshal(payload, &parsed); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if len(parsed.Servers) != 1 {
			t.Fatalf("expected 1 server, got %d", len(parsed.Servers))
		}
		srv := parsed.Servers[0]
		if srv.UUID != "srv-uuid-1" || srv.Port != 8080 {
			t.Fatalf("unexpected server data: %+v", srv)
		}
		foundCustomVar := false
		for _, env := range srv.Env {
			if env == "SERVER_NAME=MyCustomServer" {
				foundCustomVar = true
				break
			}
		}
		if !foundCustomVar {
			t.Fatalf("expected SERVER_NAME=MyCustomServer in env, got: %+v", srv.Env)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("DEADLOCK: ServersSyncPayload tidak selesai dalam 5 detik")
	}
}
