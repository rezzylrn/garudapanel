// Test panel console token: sekali pakai + expiry 60 detik (SPEC §4).
package panel

import (
	"testing"
	"time"
)

func TestConsoleTokenSingleUse(t *testing.T) {
	raw, err := issueConsoleToken(42, 7)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	tok := consumeConsoleToken(raw)
	if tok == nil {
		t.Fatal("first consume should succeed")
	}
	if tok.serverID != 42 || tok.userID != 7 {
		t.Fatalf("token data wrong: %+v", tok)
	}
	// sekali pakai: konsumsi kedua harus gagal
	if tok2 := consumeConsoleToken(raw); tok2 != nil {
		t.Fatal("token should be single-use")
	}
}

func TestConsoleTokenExpired(t *testing.T) {
	raw, err := issueConsoleToken(42, 7)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// paksa expired: geser expires_at ke belakang
	consoleTokens.mu.Lock()
	consoleTokens.tokens[raw].expiresAt = time.Now().Add(-time.Second).Unix()
	consoleTokens.mu.Unlock()
	if tok := consumeConsoleToken(raw); tok != nil {
		t.Fatal("expired token should not be consumable")
	}
}

func TestConsoleTokenWrongServer(t *testing.T) {
	raw, err := issueConsoleToken(42, 7)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// token dipakai untuk server lain → harus ditolak di handler (cek serverID)
	tok := consumeConsoleToken(raw)
	if tok == nil {
		t.Fatal("token should be valid before handler check")
	}
	if tok.serverID == 99 {
		t.Fatal("token must be bound to issuing server id")
	}
}

func TestConsoleTokenExpiryIs60s(t *testing.T) {
	raw, _ := issueConsoleToken(1, 1)
	consoleTokens.mu.Lock()
	tok := consoleTokens.tokens[raw]
	consoleTokens.mu.Unlock()
	if tok == nil {
		t.Fatal("token not stored")
	}
	delta := time.Until(time.Unix(tok.expiresAt, 0))
	if delta < 59*time.Second || delta > 61*time.Second {
		t.Fatalf("expiry = %v, want ~60s", delta)
	}
}
