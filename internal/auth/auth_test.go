package auth

import (
	"path/filepath"
	"testing"

	"garudapanel/internal/db"
)

func TestSessionStore_CreateAndValidate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	res, err := sqlDB.Exec(`INSERT INTO users (username, email, password_hash, is_admin) VALUES (?, ?, ?, ?)`,
		"alice", "alice@example.com", hash, 1)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	userID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	store := NewSessionStore(sqlDB)
	token, err := store.Create(userID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("session store create failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	sess, err := store.Validate(token)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if sess.UserID != userID || sess.Username != "alice" || !sess.IsAdmin {
		t.Fatalf("unexpected session row: %+v", sess)
	}

	// FK violation check: user ID tidak ada
	_, err = store.Create(99999, "127.0.0.1", "test-agent")
	if err == nil {
		t.Fatal("expected foreign key error when creating session for non-existent user")
	}
}
