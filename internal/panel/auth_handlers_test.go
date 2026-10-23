package panel

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"garudapanel/internal/auth"
	"garudapanel/internal/db"
)

func TestHandleLogin_SuccessAndErrors(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	hash, err := auth.HashPassword("validpass123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	_, err = sqlDB.Exec(`INSERT INTO users (username, email, password_hash, is_admin) VALUES (?, ?, ?, ?)`,
		"bob", "bob@example.com", hash, 0)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	s := &Server{
		DB:       sqlDB,
		Sessions: auth.NewSessionStore(sqlDB),
	}

	// 1. Success login via username
	body, _ := json.Marshal(map[string]string{
		"username": "bob",
		"password": "validpass123",
	})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.0.2.1:12345"
	w := httptest.NewRecorder()
	s.handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	token, ok := res["token"].(string)
	if !ok || token == "" {
		t.Fatalf("expected token in response, got %v", res)
	}

	// 2. Success login via identifier (email)
	body, _ = json.Marshal(map[string]string{
		"identifier": "bob@example.com",
		"password":   "validpass123",
	})
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.0.2.2:12345"
	w = httptest.NewRecorder()
	s.handleLogin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for email login, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Invalid credentials
	body, _ = json.Marshal(map[string]string{
		"username": "bob",
		"password": "wrongpassword",
	})
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "192.0.2.3:12345"
	w = httptest.NewRecorder()
	s.handleLogin(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
