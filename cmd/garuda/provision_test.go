package main

import (
	"path/filepath"
	"testing"

	"garudapanel/internal/db"
)

func TestAdminProvisionNode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db open failed: %v", err)
	}
	defer sqlDB.Close()

	// Run provision-node first time
	cmdAdminProvisionNode([]string{"--db", dbPath, "--name", "test-node-1", "--memory-mb", "4096", "--disk-mb", "51200"})

	// Check location created
	var locCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM locations WHERE short_code = 'local'`).Scan(&locCount); err != nil || locCount != 1 {
		t.Fatalf("expected 1 default location, got %d, err: %v", locCount, err)
	}

	// Check node created
	var nodeID int64
	var mem, disk int64
	if err := sqlDB.QueryRow(`SELECT id, memory_limit_mb, disk_limit_mb FROM nodes WHERE name = 'test-node-1'`).Scan(&nodeID, &mem, &disk); err != nil {
		t.Fatalf("query node failed: %v", err)
	}
	if mem != 4096 || disk != 51200 {
		t.Fatalf("expected mem=4096 disk=51200, got mem=%d disk=%d", mem, disk)
	}

	// Check token created
	var tokenCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM node_join_tokens WHERE node_id = ? AND used = 0`, nodeID).Scan(&tokenCount); err != nil || tokenCount != 1 {
		t.Fatalf("expected 1 unused token, got %d, err: %v", tokenCount, err)
	}

	// Run provision-node second time on same node name (should reuse node and issue new token)
	cmdAdminProvisionNode([]string{"--db", dbPath, "--name", "test-node-1"})
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM node_join_tokens WHERE node_id = ? AND used = 0`, nodeID).Scan(&tokenCount); err != nil || tokenCount != 2 {
		t.Fatalf("expected 2 unused tokens, got %d, err: %v", tokenCount, err)
	}

	var nodeTotal int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&nodeTotal); err != nil || nodeTotal != 1 {
		t.Fatalf("expected node count 1, got %d", nodeTotal)
	}
}
