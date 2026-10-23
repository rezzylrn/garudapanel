package dockerrt

import (
	"context"
	"io"
	"sync"
)

// FakeRuntime: implementasi ContainerRuntime in-memory untuk unit test daemon
// tanpa Docker. Perilaku dibuat deterministic dan sesuai kontrak:
// - Create menyimpan spec; Start/Stop/Kill mengubah state.
// - ExecAttach mengalirkan data dari FakeOutput (seolah stdout container)
//   ke out, dan menyalin apa pun yang ditulis ke in ke FakeInput.
// - Semua method reentrant-safe (mutex) karena daemon memanggil dari
//   beberapa goroutine.
type FakeRuntime struct {
	mu sync.Mutex
	// state per ref: offline/starting/running/stopping
	states map[string]string
	specs  map[string]ContainerSpec
	// fails: bila diset, method bernama mengembalikan error (untuk test error path)
	Fail map[string]error

	// FakeOutput: tiap ExecAttach menyalin konten ini ke `out` lalu menutupnya.
	FakeOutput []byte
	// OnExec: callback opsional (dipakai test untuk memverifikasi input stdin).
	OnExec func(ref string, in []byte)
	// execs: jumlah sesi exec yang dibuka (untuk test goroutine leak).
	execs int
}

func NewFake() *FakeRuntime {
	return &FakeRuntime{
		states: map[string]string{},
		specs:  map[string]ContainerSpec{},
		Fail:   map[string]error{},
	}
}

func (f *FakeRuntime) fail(method string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Fail[method]
}

func (f *FakeRuntime) Create(_ context.Context, spec ContainerSpec) error {
	if err := f.fail("create"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.specs[spec.Ref] = spec
	f.states[spec.Ref] = StateOffline
	return nil
}

func (f *FakeRuntime) Start(_ context.Context, ref string) error {
	if err := f.fail("start"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[ref] = StateRunning
	return nil
}

func (f *FakeRuntime) Stop(_ context.Context, ref string, _ int) error {
	if err := f.fail("stop"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[ref] = StateOffline
	return nil
}

func (f *FakeRuntime) Restart(_ context.Context, ref string, _ int) error {
	if err := f.fail("restart"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[ref] = StateRunning
	return nil
}

func (f *FakeRuntime) Kill(_ context.Context, ref string) error {
	if err := f.fail("kill"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[ref] = StateOffline
	return nil
}

func (f *FakeRuntime) Remove(_ context.Context, ref string, _ bool) error {
	if err := f.fail("remove"); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.states, ref)
	delete(f.specs, ref)
	return nil
}

func (f *FakeRuntime) State(_ context.Context, ref string) (string, error) {
	if err := f.fail("state"); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.states[ref]
	if !ok {
		return StateOffline, nil // container tidak ada = offline
	}
	return s, nil
}

func (f *FakeRuntime) ExecAttach(ctx context.Context, ref string, in io.Reader, out io.Writer) (ExecHandle, error) {
	if err := f.fail("exec"); err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.execs++
	f.mu.Unlock()

	// salin input dari `in` (stdin console) sampai EOF / ctx done — streaming.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := in.Read(buf)
			if n > 0 {
				if f.OnExec != nil {
					f.OnExec(ref, buf[:n])
				}
			}
			if err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	// tulis output fake ke out sampai selesai/ctx done — streaming.
	go func() {
		if len(f.FakeOutput) > 0 {
			_, _ = out.Write(f.FakeOutput)
		}
		<-ctx.Done() // tetap hidup sampai sesi ditutup (seperti exec asli)
	}()

	return fakeHandle{ctx: ctx}, nil
}

func (f *FakeRuntime) ExecCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.execs
}

// SetState memaksa state satu ref (dipakai test untuk simulasi crash).
func (f *FakeRuntime) SetState(ref, state string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states[ref] = state
}

// Specs mengembalikan copy spec terdaftar (untuk assert di test).
func (f *FakeRuntime) Specs() map[string]ContainerSpec {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]ContainerSpec{}
	for k, v := range f.specs {
		out[k] = v
	}
	return out
}

func (f *FakeRuntime) Events(_ context.Context, _ chan<- StateEvent) {
	// fake: tidak ada event async (test panggil SetState manual)
}

type fakeHandle struct{ ctx context.Context }

func (fakeHandle) Resize(context.Context, uint, uint) error { return nil }
func (fakeHandle) Close() error                             { return nil }
