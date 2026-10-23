// Test helper: supervisor dengan fake runtime + frame collector thread-safe.
package daemon

import (
	"sync"
	"testing"
	"time"

	"garudapanel/internal/dockerrt"
)

// frameCollector: kumpulan frame yang aman dipanggil dari goroutine mana pun.
type frameCollector struct {
	mu     sync.Mutex
	frames []Frame
}

func (fc *frameCollector) collect(f Frame) {
	fc.mu.Lock()
	fc.frames = append(fc.frames, f)
	fc.mu.Unlock()
}

func (fc *frameCollector) all() []Frame {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	out := make([]Frame, len(fc.frames))
	copy(out, fc.frames)
	return out
}

// newTestSupervisor: supervisor fake runtime + collector.
func newTestSupervisor(t *testing.T) (*Supervisor, *dockerrt.FakeRuntime, *frameCollector) {
	t.Helper()
	rt := dockerrt.NewFake()
	sup := NewSupervisor(rt, SupervisorConfig{StatsInterval: 50 * time.Millisecond})
	fc := &frameCollector{}
	sup.Attach(func(f Frame) error {
		fc.collect(f)
		return nil
	})
	return sup, rt, fc
}

// waitCond: poll kondisi sampai true / timeout.
func waitCond(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
