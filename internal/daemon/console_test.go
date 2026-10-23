// Test console: buka-tutup 100x tanpa goroutine leak (SPEC §9) +
// replay mengirim ulang log terakhir + batas 30 sesi.
package daemon

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"garudapanel/internal/dockerrt"
)

// TestConsole100OpenCloseNoLeak: buka+tutup console 100x — jumlah goroutine
// daemon kembali ke baseline (SPEC §9/§12).
func TestConsole100OpenCloseNoLeak(t *testing.T) {
	sup, _, _ := newTestSupervisor(t)
	sup.Register("uuid-leak", dockerrt.ContainerSpec{Ref: "uuid-leak", Image: "t", Cmd: []string{"sh"}})

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		sup.consoleOpen("uuid-leak", "sess-1")
		if !waitCond(2*time.Second, func() bool {
			sup.mu.Lock()
			h, ok := sup.consoles["uuid-leak"]
			sup.mu.Unlock()
			if !ok {
				return false
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			return len(h.conns) == 1
		}) {
			t.Fatalf("iteration %d: session never registered", i)
		}
		sup.consoleClose("uuid-leak", "sess-1", "test")
		if !waitCond(2*time.Second, func() bool {
			sup.mu.Lock()
			h, exist := sup.consoles["uuid-leak"]
			sup.mu.Unlock()
			if !exist {
				return true
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			return len(h.conns) == 0
		}) {
			t.Fatalf("iteration %d: session not cleaned up", i)
		}
	}

	// goroutine harus kembali ke baseline (toleransi ±2 untuk goroutine test)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		if runtime.NumGoroutine() <= baseline+2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("goroutine leak: now %d, baseline %d", runtime.NumGoroutine(), baseline)
}

// TestConsoleReplay: replay mengirim ulang log terakhir dari ring buffer.
func TestConsoleReplay(t *testing.T) {
	sup, _, fc := newTestSupervisor(t)
	sup.Register("uuid-replay", dockerrt.ContainerSpec{Ref: "uuid-replay", Image: "t", Cmd: []string{"sh"}})

	ring := sup.ringFor("uuid-replay")
	ring.Write([]byte("line1\nline2\n"))
	sup.consoleReplay("uuid-replay")

	for _, f := range fc.all() {
		if f.Ch == "console:uuid-replay" && f.Type == "out" && strings.Contains(string(f.Payload), "line2") {
			return // ok
		}
	}
	t.Fatal("replay did not resend ring buffer content")
}

// TestConsoleMaxSessions: sesi ke-31 ditolak (SPEC §4: maks 30).
func TestConsoleMaxSessions(t *testing.T) {
	sup, _, fc := newTestSupervisor(t)
	sup.Register("uuid-cap", dockerrt.ContainerSpec{Ref: "uuid-cap", Image: "t", Cmd: []string{"sh"}})

	for i := 0; i < maxConsoleSessions+10; i++ {
		sup.consoleOpen("uuid-cap", "bulk-"+strings.Repeat("s", i)) // id unik
	}
	time.Sleep(200 * time.Millisecond)
	rejected := 0
	for _, f := range fc.all() {
		if f.Type == "stat" && strings.Contains(string(f.Payload), "too many sessions") {
			rejected++
		}
	}
	if rejected == 0 {
		t.Fatal("sessions beyond the 30-limit were not rejected")
	}
	sup.consoleCloseAll("test done")
}

// TestConsoleTeeToRingAndPanel: output console masuk ring buffer DAN panel.
func TestConsoleTeeToRingAndPanel(t *testing.T) {
	rt := dockerrt.NewFake()
	rt.FakeOutput = []byte("server started\n")
	sup := NewSupervisor(rt, SupervisorConfig{StatsInterval: time.Hour})
	fc := &frameCollector{}
	sup.Attach(func(f Frame) error {
		fc.collect(f)
		return nil
	})
	sup.Register("uuid-tee", dockerrt.ContainerSpec{Ref: "uuid-tee", Image: "t", Cmd: []string{"sh"}})

	sup.consoleOpen("uuid-tee", "s1")
	if !waitCond(2*time.Second, func() bool {
		return strings.Contains(string(sup.ringFor("uuid-tee").Bytes()), "server started")
	}) {
		t.Fatal("console output never reached ring buffer")
	}
	found := false
	for _, f := range fc.all() {
		if f.Ch == "console:uuid-tee" && f.Type == "out" && strings.Contains(string(f.Payload), "server started") {
			found = true
		}
	}
	if !found {
		t.Fatal("console output not forwarded to panel")
	}
	sup.consoleCloseAll("done")
}
