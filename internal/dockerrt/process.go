// ProcessRuntime: ContainerRuntime yang menjalankan startup command server
// LANGSUNG sebagai proses host (tanpa container). Dipakai daemon sebagai
// fallback ketika Docker tidak tersedia/tidak bisa menjalankan container
// (mis. sandbox tanpa privilege namespace). TIDAK ADA ISOLASI: proses jalan
// dengan UID daemon, akses filesystem host penuh ke volume server. Daemon
// WAJIB mencatat pemakaian runtime ini dengan jelas di log saat startup.
//
// Perilaku disamakan dengan Docker sebisa mungkin:
//   - Create menyimpan spec (idempotent; proses lama yang masih jalan dihentikan).
//   - Start menjalankan Cmd di HostPath dengan Env dari spec; stdout+stderr
//     di-broadcast ke semua sesi ExecAttach + disimpan sebagai replay (100KB).
//   - ExecAttach: replay dulu, lalu output live; stdin ditulis ke stdin proses.
//   - Stop mengirim StopSignal ke SELURUH process group, lalu SIGKILL bila
//     masih hidup setelah timeout.
//   - Process exit (crash/normal) mengirim StateEvent offline — tanpa restart
//     otomatis, sama seperti Docker.
package dockerrt

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const processReplayCap = 100 * 1024 // samakan dengan ring buffer console (SPEC §4)

// processRT: runtime proses host.
type processRT struct {
	mu     sync.Mutex
	procs  map[string]*procState
	events []chan<- StateEvent
}

// procState: satu proses server.
type procState struct {
	spec   ContainerSpec
	cmd    *exec.Cmd
	cancel context.CancelFunc
	stdin  io.WriteCloser
	done   chan struct{} // ditutup saat proses exit
	exited bool
	// runningReported: teks StartupDone sudah terlihat di output (atau
	// StartupDone kosong sehingga running langsung saat Start).
	runningReported bool
	// startedAt: kapan proses terakhir di-Start (untuk uptime).
	// lastJiffies/lastWall: baseline CPU% antar panggilan StatsOnce.
	startedAt   time.Time
	lastJiffies uint64
	lastWall    time.Time
	mu          sync.Mutex // lindungi subs + replay
	subs        map[*procSub]struct{}
	replay      []byte
}

// procSub: satu subscriber output (sesi ExecAttach).
type procSub struct {
	ch   chan []byte
	done chan struct{}
}

// NewProcess: buat ProcessRuntime kosong.
func NewProcess() *processRT {
	return &processRT{procs: map[string]*procState{}}
}

// Create: simpan spec; hentikan proses lama bila masih jalan.
func (r *processRT) Create(ctx context.Context, spec ContainerSpec) error {
	if spec.Ref == "" {
		return errors.New("process runtime: ref kosong")
	}
	if len(spec.Cmd) == 0 {
		return errors.New("process runtime: cmd kosong")
	}
	if spec.HostPath != "" {
		if st, err := os.Stat(spec.HostPath); err != nil || !st.IsDir() {
			return fmt.Errorf("process runtime: volume dir tidak ada: %s", spec.HostPath)
		}
	}
	r.mu.Lock()
	if old, ok := r.procs[spec.Ref]; ok {
		r.mu.Unlock()
		_ = r.killProc(old)
		r.mu.Lock()
	}
	r.procs[spec.Ref] = &procState{spec: spec, subs: map[*procSub]struct{}{}}
	r.mu.Unlock()
	return nil
}

// Start: jalankan proses. Idempotent terhadap Create yang belum di-Start;
// bila proses sudah jalan, kembalikan error agar supervisor tidak dobel.
func (r *processRT) Start(ctx context.Context, ref string) error {
	r.mu.Lock()
	st, ok := r.procs[ref]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("process runtime: server %s belum di-Create", ref)
	}
	st.mu.Lock()
	if st.cmd != nil && !st.exited {
		st.mu.Unlock()
		return fmt.Errorf("process runtime: server %s sudah running", ref)
	}
	spec := st.spec
	st.mu.Unlock()

	argv := spec.Cmd
	// Kompatibilitas yolks: script egg memakai path absolut image
	// (/usr/local/bin/npm, /usr/local/bin/node, ...) yang tidak ada di host.
	// Saat jalan native, strip prefix itu supaya command resolve via PATH host.
	// Hanya untuk argv bentuk [shell, -c, script] dari panel.
	if len(argv) >= 3 && argv[1] == "-c" {
		argv = append([]string{}, argv...)
		argv[2] = strings.ReplaceAll(argv[2], "/usr/local/bin/", "")
	}
	// Cmd dari panel: ["/bin/bash", "-c", "<script>"] atau argv langsung.
	procCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(procCtx, argv[0], argv[1:]...)
	cmd.Dir = spec.HostPath
	cmd.Env = append(append([]string{}, os.Environ()...), spec.Env...)
	// Process group sendiri: Stop/Kill bisa sinyal seluruh grup (anak cucu).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	st.mu.Lock()
	st.cmd = cmd
	st.cancel = cancel
	st.stdin = stdin
	st.exited = false
	st.done = make(chan struct{})
	st.runningReported = false
	st.startedAt = time.Now()
	st.lastJiffies = 0
	st.lastWall = time.Time{}
	st.mu.Unlock()

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("process runtime: start: %w", err)
	}
	// Pompa output: stdout+stderr → broadcast (replay + subscriber).
	go r.pumpOutput(st, stdout)
	go r.pumpOutput(st, stderr)
	// Tunggu exit → tandai exited, kirim event offline.
	go func() {
		_ = cmd.Wait()
		cancel()
		st.mu.Lock()
		st.exited = true
		close(st.done)
		st.mu.Unlock()
		r.emit(StateEvent{Ref: ref, Status: StateOffline})
	}()
	// Status awal: bila egg punya teks startup-done, tetap "starting"
	// sampai teks itu muncul di output (dicek broadcast); bila tidak ada,
	// running begitu proses hidup — sama seperti sebelumnya.
	if spec.StartupDone == "" {
		st.mu.Lock()
		st.runningReported = true
		st.mu.Unlock()
		r.emit(StateEvent{Ref: ref, Status: StateRunning})
	} else {
		r.emit(StateEvent{Ref: ref, Status: StateStarting})
	}
	return nil
}

// pumpOutput: baca stream proses, broadcast per baris.
func (r *processRT) pumpOutput(st *procState, rd io.Reader) {
	br := bufio.NewReader(rd)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			r.broadcast(st, line)
		}
		if err != nil {
			return
		}
	}
}

// broadcast: simpan ke replay (cap, potong dari depan) + kirim ke subscriber
// (non-blocking; subscriber lambat kehilangan potongan, proses tidak stall).
// Di sini juga deteksi teks startup-done: kemunculan pertamanya mengubah
// status starting → running (pola Pterodactyl wings).
func (r *processRT) broadcast(st *procState, p []byte) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.replay = append(st.replay, p...)
	if len(st.replay) > processReplayCap {
		st.replay = st.replay[len(st.replay)-processReplayCap:]
	}
	if !st.runningReported && st.spec.StartupDone != "" && bytes.Contains(p, []byte(st.spec.StartupDone)) {
		st.runningReported = true
		// emit hanya mengunci r.mu (bukan st.mu) jadi aman dari sini.
		defer r.emit(StateEvent{Ref: st.spec.Ref, Status: StateRunning})
	}
	cp := append([]byte(nil), p...)
	for sub := range st.subs {
		select {
		case sub.ch <- cp:
		default:
		}
	}
}

// Stop: kirim StopSignal ke process group, tunggu timeout, lalu SIGKILL.
func (r *processRT) Stop(ctx context.Context, ref string, timeoutSec int) error {
	r.mu.Lock()
	st, ok := r.procs[ref]
	r.mu.Unlock()
	if !ok {
		return nil // tidak ada proses = sudah offline
	}
	st.mu.Lock()
	cmd := st.cmd
	exited := st.exited
	done := st.done
	st.mu.Unlock()
	if cmd == nil || cmd.Process == nil || exited {
		return nil
	}
	sig := signalFor(st.spec.StopSignal)
	// Sinyal ke -PGID: seluruh grup proses (shell + anaknya).
	_ = syscall.Kill(-cmd.Process.Pid, sig)
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	select {
	case <-done:
		return nil
	case <-time.After(time.Duration(timeoutSec) * time.Second):
	}
	_ = r.killProc(st)
	<-done
	return nil
}

// Restart: Stop lalu Start.
func (r *processRT) Restart(ctx context.Context, ref string, timeoutSec int) error {
	if err := r.Stop(ctx, ref, timeoutSec); err != nil {
		return err
	}
	return r.Start(ctx, ref)
}

// Kill: SIGKILL process group seketika.
func (r *processRT) Kill(ctx context.Context, ref string) error {
	r.mu.Lock()
	st, ok := r.procs[ref]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	return r.killProc(st)
}

func (r *processRT) killProc(st *procState) error {
	st.mu.Lock()
	cmd := st.cmd
	exited := st.exited
	done := st.done
	st.mu.Unlock()
	if cmd == nil || cmd.Process == nil || exited {
		return nil
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if done != nil {
		<-done
	}
	return nil
}

// Remove: hentikan proses, hapus dari registry.
func (r *processRT) Remove(ctx context.Context, ref string, force bool) error {
	r.mu.Lock()
	st, ok := r.procs[ref]
	if ok {
		delete(r.procs, ref)
	}
	r.mu.Unlock()
	if !ok {
		return nil
	}
	return r.killProc(st)
}

// State: starting bila proses hidup tapi teks startup-done belum muncul,
// running bila hidup dan done (atau tanpa done), else offline.
func (r *processRT) State(ctx context.Context, ref string) (string, error) {
	r.mu.Lock()
	st, ok := r.procs[ref]
	r.mu.Unlock()
	if !ok {
		return StateOffline, nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.cmd != nil && !st.exited {
		if st.spec.StartupDone != "" && !st.runningReported {
			return StateStarting, nil
		}
		return StateRunning, nil
	}
	return StateOffline, nil
}

// ExecAttach: replay + live output; stdin ke proses. Handle Resize/Close no-op.
func (r *processRT) ExecAttach(ctx context.Context, ref string, in io.Reader, out io.Writer) (ExecHandle, error) {
	r.mu.Lock()
	st, ok := r.procs[ref]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("process runtime: server %s tidak terdaftar", ref)
	}
	sub := &procSub{ch: make(chan []byte, 128), done: make(chan struct{})}
	st.mu.Lock()
	st.subs[sub] = struct{}{}
	replay := append([]byte(nil), st.replay...)
	stdin := st.stdin
	running := st.cmd != nil && !st.exited
	st.mu.Unlock()
	if !running {
		st.mu.Lock()
		delete(st.subs, sub)
		st.mu.Unlock()
		return nil, fmt.Errorf("process runtime: server %s sedang offline", ref)
	}
	// Replay dulu (sinkron, sebelum live) supaya urutan output benar.
	if len(replay) > 0 {
		if _, err := out.Write(replay); err != nil {
			st.mu.Lock()
			delete(st.subs, sub)
			st.mu.Unlock()
			return nil, err
		}
	}
	h := &procExecHandle{rt: r, st: st, sub: sub}
	// goroutine 1: subscriber → out.
	go func() {
		defer func() {
			st.mu.Lock()
			delete(st.subs, sub)
			st.mu.Unlock()
		}()
		for {
			select {
			case <-sub.done:
				return
			case <-ctx.Done():
				return
			case p, ok := <-sub.ch:
				if !ok {
					return
				}
				if _, err := out.Write(p); err != nil {
					return
				}
			}
		}
	}()
	// goroutine 2: in (stdin browser) → stdin proses.
	go func() {
		if stdin == nil {
			return
		}
		_, _ = io.Copy(stdin, in)
	}()
	return h, nil
}

// Events: terima event state; channel ditutup saat ctx.Done.
func (r *processRT) Events(ctx context.Context, ch chan<- StateEvent) {
	r.mu.Lock()
	r.events = append(r.events, ch)
	r.mu.Unlock()
	<-ctx.Done()
	r.mu.Lock()
	kept := r.events[:0]
	for _, c := range r.events {
		if c != ch {
			kept = append(kept, c)
		}
	}
	r.events = kept
	r.mu.Unlock()
}

func (r *processRT) emit(ev StateEvent) {
	r.mu.Lock()
	subs := append([]chan<- StateEvent(nil), r.events...)
	r.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// StatsOnce: baca /proc/<pid> (RSS + CPU time). Gagal → snapshot nol.
func (r *processRT) StatsOnce(ctx context.Context, ref string) (StatsSnapshot, error) {
	snap := StatsSnapshot{ServerUUID: ref}
	r.mu.Lock()
	st, ok := r.procs[ref]
	r.mu.Unlock()
	if !ok {
		return snap, fmt.Errorf("process runtime: server %s tidak terdaftar", ref)
	}
	st.mu.Lock()
	cmd := st.cmd
	running := cmd != nil && !st.exited
	memLimit := st.spec.MemoryMB
	startedAt := st.startedAt
	st.mu.Unlock()
	if !running || cmd.Process == nil {
		return snap, nil
	}
	snap.MemLimitMB = float64(memLimit)
	if !startedAt.IsZero() {
		snap.UptimeSec = int64(time.Since(startedAt).Seconds())
	}
	pid := cmd.Process.Pid
	if rssKB, err := procRSSKB(pid); err == nil {
		snap.MemUsageMB = float64(rssKB) / 1024
	}
	if utime, stime, err := procCPUJiffies(pid); err == nil {
		// CPU%: delta jiffies (utime+stime, HZ=100) terhadap baseline
		// panggilan sebelumnya. Panggilan pertama hanya menyimpan
		// baseline dan melaporkan 0.
		total := utime + stime
		now := time.Now()
		st.mu.Lock()
		if !st.lastWall.IsZero() && now.After(st.lastWall) {
			elapsed := now.Sub(st.lastWall).Seconds()
			if elapsed > 0 && total >= st.lastJiffies {
				pct := float64(total-st.lastJiffies) / 100 / elapsed * 100
				if pct < 0 {
					pct = 0
				}
				snap.CPUPercent = pct
			}
		}
		st.lastJiffies = total
		st.lastWall = now
		st.mu.Unlock()
	}
	return snap, nil
}

func procRSSKB(pid int) (int64, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				return strconv.ParseInt(f[1], 10, 64)
			}
		}
	}
	return 0, errors.New("VmRSS tidak ditemukan")
}

func procCPUJiffies(pid int) (uint64, uint64, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, 0, err
	}
	// field 14 (utime) & 15 (stime); nama comm bisa mengandung spasi/parens.
	s := string(b)
	end := strings.LastIndex(s, ")")
	if end < 0 {
		return 0, 0, errors.New("format /proc stat tidak dikenal")
	}
	f := strings.Fields(s[end+1:])
	if len(f) < 13 {
		return 0, 0, errors.New("field /proc stat kurang")
	}
	utime, err1 := strconv.ParseUint(f[11], 10, 64)
	stime, err2 := strconv.ParseUint(f[12], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, errors.New("parse jiffies gagal")
	}
	return utime, stime, nil
}

func signalFor(name string) syscall.Signal {
	switch name {
	case "SIGINT":
		return syscall.SIGINT
	case "SIGKILL":
		return syscall.SIGKILL
	case "SIGHUP":
		return syscall.SIGHUP
	case "SIGQUIT":
		return syscall.SIGQUIT
	default:
		return syscall.SIGTERM
	}
}

// procExecHandle: sesi console proses.
type procExecHandle struct {
	rt  *processRT
	st  *procState
	sub *procSub
}

func (h *procExecHandle) Resize(ctx context.Context, cols, rows uint) error { return nil }

func (h *procExecHandle) Close() error {
	h.st.mu.Lock()
	delete(h.st.subs, h.sub)
	h.st.mu.Unlock()
	select {
	case <-h.sub.done:
	default:
		close(h.sub.done)
	}
	return nil
}
