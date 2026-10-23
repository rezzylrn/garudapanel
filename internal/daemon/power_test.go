// Test power actions end-to-end (REST panel → WS RPC → runtime fake)
// dan status reporting (SPEC §7.1: daemon satu-satunya yang ubah state).
package daemon

import (
	"bytes"
	"context"
	"testing"

	"garudapanel/internal/dockerrt"
)

func TestPowerStartStopReportsState(t *testing.T) {
	sup, rt, frames := newTestSupervisor(t)
	sup.Register("uuid-1", dockerrt.ContainerSpec{Ref: "uuid-1", Image: "test", Cmd: []string{"sh"}})

	if err := sup.Power("uuid-1", PowerStart); err != nil {
		t.Fatalf("start: %v", err)
	}
	if got := sup.Status("uuid-1"); got != dockerrt.StateRunning {
		t.Fatalf("status after start = %q, want running", got)
	}
	if err := sup.Power("uuid-1", PowerStop); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := sup.Status("uuid-1"); got != dockerrt.StateOffline {
		t.Fatalf("status after stop = %q, want offline", got)
	}
	if err := sup.Power("uuid-1", PowerRestart); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if got := sup.Status("uuid-1"); got != dockerrt.StateRunning {
		t.Fatalf("status after restart = %q, want running", got)
	}
	if err := sup.Power("uuid-1", PowerKill); err != nil {
		t.Fatalf("kill: %v", err)
	}
	if got := sup.Status("uuid-1"); got != dockerrt.StateOffline {
		t.Fatalf("status after kill = %q, want offline", got)
	}

	// state frame harus dikirim ke panel tiap transisi:
	// starting, running, stopping, offline, stopping, running, offline
	var states []string
	for _, f := range frames.all() {
		if f.Ch == "rpc" && f.Type == "state" {
			var p struct {
				ServerUUID string `json:"server_uuid"`
				Status     string `json:"status"`
			}
			if err := unmarshal(f.Payload, &p); err == nil {
				states = append(states, p.Status)
			}
		}
	}
	want := []string{"starting", "running", "stopping", "offline", "stopping", "running", "offline"}
	if len(states) != len(want) {
		t.Fatalf("state frames = %v, want %v", states, want)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Fatalf("state[%d] = %q, want %q (all: %v)", i, states[i], want[i], states)
		}
	}
	// runtime fake juga harus mengubah state internalnya
	st, _ := rt.State(context.Background(), "uuid-1")
	if st != dockerrt.StateOffline {
		t.Fatalf("fake runtime state after kill = %q", st)
	}
}

func TestPowerUnregisteredServerFails(t *testing.T) {
	sup, _, _ := newTestSupervisor(t)
	if err := sup.Power("nope", PowerStart); err == nil {
		t.Fatal("power on unregistered server should fail")
	}
}

func TestPowerStartFailureReportsOffline(t *testing.T) {
	sup, rt, frames := newTestSupervisor(t)
	sup.Register("uuid-err", dockerrt.ContainerSpec{Ref: "uuid-err", Image: "test", Cmd: []string{"sh"}})
	rt.Fail["start"] = context.DeadlineExceeded
	if err := sup.Power("uuid-err", PowerStart); err == nil {
		t.Fatal("start should fail")
	}
	if got := sup.Status("uuid-err"); got != dockerrt.StateOffline {
		t.Fatalf("status after failed start = %q, want offline", got)
	}
	// harus ada frame state offline ke panel
	found := false
	for _, f := range frames.all() {
		if f.Type == "state" && bytes.Contains(f.Payload, []byte(`"status":"offline"`)) {
			found = true
		}
	}
	if !found {
		t.Fatal("no offline state frame sent to panel after failed start")
	}
}
