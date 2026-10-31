package panel

import (
	"strings"
	"testing"
	"time"
)

func TestPowerAllowed(t *testing.T) {
	now := time.Now()
	never := time.Time{}
	recent := now.Add(-1 * time.Second)
	old := now.Add(-60 * time.Second)

	cases := []struct {
		status, action string
		lastStart      time.Time
		allowed        bool
		wantSubstr     string
	}{
		// start: hanya dari offline
		{"offline", "start", never, true, ""},
		{"running", "start", never, false, "running"},
		{"starting", "start", never, false, "starting"},
		{"stopping", "start", never, false, "stopping"},
		// stop: hanya dari running; starting WAJIB error (perintah rezz)
		{"running", "stop", old, true, ""},
		{"starting", "stop", never, false, "still starting"},
		{"stopping", "stop", never, false, "already stopping"},
		{"offline", "stop", never, false, "offline"},
		// jendela balapan: restart baru saja dikirim, status DB masih "running"
		// (daemon belum melaporkan "starting") — stop tetap diblokir.
		{"running", "stop", recent, false, "still starting"},
		// ...tapi stop 5 detik setelah start yang sudah mencapai running
		// HARUS diizinkan (jendela balapan hanya 3 detik).
		{"running", "stop", now.Add(-5 * time.Second), true, ""},
		{"offline", "stop", recent, false, "offline"},
		// restart: hanya dari running
		{"running", "restart", old, true, ""},
		{"starting", "restart", never, false, "still starting"},
		{"offline", "restart", never, false, "offline"},
		{"stopping", "restart", never, false, "stopping"},
		{"running", "restart", recent, false, "still starting"},
		// kill: escape hatch, semua kecuali offline
		{"running", "kill", never, true, ""},
		{"stopping", "kill", never, true, ""},
		{"starting", "kill", never, true, ""},
		{"offline", "kill", never, false, "offline"},
	}
	for _, c := range cases {
		ok, reason := powerAllowed(c.status, c.action, c.lastStart, now)
		if ok != c.allowed {
			t.Errorf("powerAllowed(%q,%q,lastStart=%v) = %v, want %v (reason %q)",
				c.status, c.action, c.lastStart, ok, c.allowed, reason)
		}
		if !c.allowed && !strings.Contains(reason, c.wantSubstr) {
			t.Errorf("powerAllowed(%q,%q) reason %q, want substring %q",
				c.status, c.action, reason, c.wantSubstr)
		}
	}
}

func TestPowerCooldownRemaining(t *testing.T) {
	now := time.Now()
	if d := powerCooldownRemaining(time.Time{}, now); d != 0 {
		t.Errorf("never acted: got %v, want 0", d)
	}
	if d := powerCooldownRemaining(now.Add(-10*time.Second), now); d != 0 {
		t.Errorf("10s ago: got %v, want 0", d)
	}
	if d := powerCooldownRemaining(now.Add(-1*time.Second), now); d <= 0 || d > powerCooldown {
		t.Errorf("1s ago: got %v, want (0, %v]", d, powerCooldown)
	}
	if d := powerCooldownRemaining(now, now); d != powerCooldown {
		t.Errorf("just now: got %v, want %v", d, powerCooldown)
	}
}
