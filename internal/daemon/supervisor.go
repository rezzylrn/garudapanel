// Supervisor daemon: satu-satunya komponen yang boleh mengubah state
// container (SPEC §7.1). Menerima power action via WS RPC dari panel,
// menjalankan ContainerRuntime, melaporkan perubahan status ke panel,
// dan mengelola sesi console + ring buffer + stats.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"garudapanel/internal/dockerrt"
)

// PowerAction aksi power yang dikenal daemon.
type PowerAction string

const (
	PowerStart   PowerAction = "start"
	PowerStop    PowerAction = "stop"
	PowerRestart PowerAction = "restart"
	PowerKill    PowerAction = "kill"
)

// SendFrame dipakai supervisor mengirim frame ke panel (dipasang oleh ws.go).
type SendFrame func(f Frame) error

// SupervisorConfig konfigurasi supervisor.
type SupervisorConfig struct {
	// DataDir base path volume server di host (per server: <DataDir>/<uuid>).
	DataDir string
	// StatsInterval: interval kirim stats (default 2 detik, SPEC §5.7).
	StatsInterval time.Duration
}

// serverState: state satu server yang di-supervise daemon.
type serverState struct {
	uuid   string
	spec   dockerrt.ContainerSpec
	status string // offline/starting/running/stopping
}

// Supervisor: registry container + dispatcher frame WS panel.
// Daemon satu-satunya yang boleh mengubah state container (SPEC §7.1):
// semua perubahan lewat Supervisor.Power.
type Supervisor struct {
	rt       dockerrt.ContainerRuntime
	cfg      SupervisorConfig
	send     SendFrame // kirim frame ke panel (diset setiap reconnect)
	mu       sync.Mutex
	servers  map[string]*serverState // uuid → state
	consoles map[string]*consoleHub  // uuid → hub console
	rings    map[string]*RingBuffer  // uuid → ring buffer log
	statsCtxs map[string]context.CancelFunc // uuid → cancel loop stats
}

// NewSupervisor: buat supervisor dengan runtime.
func NewSupervisor(rt dockerrt.ContainerRuntime, cfg SupervisorConfig) *Supervisor {
	if cfg.StatsInterval <= 0 {
		cfg.StatsInterval = 2 * time.Second
	}
	return &Supervisor{
		rt:       rt,
		cfg:      cfg,
		servers:  map[string]*serverState{},
		consoles: map[string]*consoleHub{},
		rings:    map[string]*RingBuffer{},
		statsCtxs: map[string]context.CancelFunc{},
	}
}

// RegisterServer: daemon tahu ada server dengan uuid + spec.
// Dipanggil saat panel kirim servers.sync (atau daemon boot dari DB, fase 3).
func (s *Supervisor) Register(uuid string, spec dockerrt.ContainerSpec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.servers[uuid]; ok {
		st.spec = spec
		return
	}
	s.servers[uuid] = &serverState{uuid: uuid, spec: spec, status: dockerrt.StateOffline}
}

// HasServer cek apakah uuid terdaftar.
func (s *Supervisor) HasServer(uuid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.servers[uuid]
	return ok
}

// Status status saat ini menurut daemon (source of truth).
func (s *Supervisor) Status(uuid string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.servers[uuid]; ok {
		return st.status
	}
	return dockerrt.StateOffline
}

// AttachWS dipanggil setiap koneksi WS panel hidup: supervisor mulai
// mengirim frame via `send`.
func (s *Supervisor) Attach(send SendFrame) {
	s.mu.Lock()
	s.send = send
	s.mu.Unlock()
}

// Detach: panggil saat WS putus — sesi console ditutup bersih, stats loop
// berhenti, frame ke panel di-drop sampai reconnect.
func (s *Supervisor) Detach() {
	s.mu.Lock()
	s.send = nil
	s.mu.Unlock()
	s.consoleCloseAll("panel disconnected")
	s.stopStatsAll()
}

// volumeFor: path volume host untuk server uuid (dipasang ke /data container).
func (s *Supervisor) volumeFor(uuid string) string {
	if s.cfg.DataDir == "" {
		return ""
	}
	return s.cfg.DataDir + "/" + uuid
}

// HandleFrame: dispatcher frame dari panel (dipanggil dari read-loop WS).
func (s *Supervisor) HandleFrame(f Frame) {
	switch {
	case f.Ch == "rpc" && f.Type == "power":
		var p struct {
			ServerUUID string `json:"server_uuid"`
			Action     string `json:"action"`
		}
		if err := unmarshal(f.Payload, &p); err != nil || p.ServerUUID == "" {
			log.Printf("power: bad payload")
			return
		}
		// goroutine: power action bisa lambat (docker stop timeout); WS read
		// loop jangan diblokir.
		go func() {
			if err := s.Power(p.ServerUUID, PowerAction(p.Action)); err != nil {
				log.Printf("power %s %s: %v", p.ServerUUID, p.Action, err)
				// laporkan error ke panel biar muncul di console
				_ = s.sendConsole(p.ServerUUID, "stat", map[string]any{"error": err.Error()})
			}
		}()

	case f.Ch == "rpc" && f.Type == "servers.sync":
		// panel kirim daftar server milik node ini saat daemon connect.
		var p struct {
			Servers []struct {
				UUID       string `json:"uuid"`
				Image      string `json:"image"`
				Cmd        []string `json:"cmd"`
				Env        []string `json:"env"`
				MemoryMB   int64  `json:"memory_mb"`
				CPUPercent int    `json:"cpu_percent"`
				StopSignal string `json:"stop_signal"`
				Port       int    `json:"port"`
			} `json:"servers"`
		}
		if err := unmarshal(f.Payload, &p); err != nil {
			log.Printf("servers.sync: bad payload: %v", err)
			return
		}
		for _, sv := range p.Servers {
			s.Register(sv.UUID, dockerrt.ContainerSpec{
				Ref:        sv.UUID,
				Image:      sv.Image,
				Cmd:        sv.Cmd,
				Env:        sv.Env,
				MemoryMB:   sv.MemoryMB,
				CPUPercent: sv.CPUPercent,
				BindPort:   sv.Port,
				HostPath:   s.volumeFor(sv.UUID),
			})
		}
		log.Printf("servers sync: %d registered", len(p.Servers))

	case f.Ch == "rpc" && f.Type == "replay":
		var p struct {
			ServerUUID string `json:"server_uuid"`
		}
		if unmarshal(f.Payload, &p) == nil && p.ServerUUID != "" {
			s.consoleReplay(p.ServerUUID)
		}

	case f.Ch == "stats" && f.Type == "subscribe":
		var p struct {
			ServerUUID string `json:"server_uuid"`
		}
		if unmarshal(f.Payload, &p) == nil && p.ServerUUID != "" {
			s.startStats(p.ServerUUID)
		}

	case strings.HasPrefix(f.Ch, "console:"):
		uuid := strings.TrimPrefix(f.Ch, "console:")
		s.handleConsoleFrame(uuid, f)
	default:
		// ping/heartbeat dkk: abaikan
	}
}

// handleConsoleFrame: routing frame console:<uuid> ke handler sesi.
func (s *Supervisor) handleConsoleFrame(uuid string, f Frame) {
	switch f.Type {
	case "open":
		var p struct {
			ID string `json:"id"`
		}
		if unmarshal(f.Payload, &p) == nil && p.ID != "" {
			s.consoleOpen(uuid, p.ID)
		}
	case "in":
		var p struct {
			ID string `json:"id"`
			D  string `json:"d"`
		}
		if unmarshal(f.Payload, &p) == nil {
			s.consoleWriteStdin(uuid, p.ID, p.D)
		}
	case "resize":
		var p struct {
			ID   string `json:"id"`
			Cols uint   `json:"cols"`
			Rows uint   `json:"rows"`
		}
		if unmarshal(f.Payload, &p) == nil {
			s.consoleResize(uuid, p.ID, p.Cols, p.Rows)
		}
	case "close":
		var p struct {
			ID string `json:"id"`
		}
		if unmarshal(f.Payload, &p) == nil {
			s.consoleClose(uuid, p.ID, "panel close")
		}
	case "replay":
		s.consoleReplay(uuid)
	}
}

// Power menjalankan power action end-to-end:
// REST panel → WS RPC → ContainerRuntime (SPEC §4). Setiap transisi state
// dilaporkan ke panel (state milik daemon, SPEC §7.1). Tidak ada auto-restart.
func (s *Supervisor) Power(uuid string, action PowerAction) error {
	s.mu.Lock()
	st, ok := s.servers[uuid]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("server %s not registered on this node", uuid)
	}

	switch action {
	case PowerStart:
		s.reportState(uuid, dockerrt.StateStarting)
		// container belum ada → create dulu (idempotent; Docker SDK create
		// menghapus container lama bernama sama).
		if err := s.rt.Create(context.Background(), st.spec); err != nil {
			s.reportState(uuid, dockerrt.StateOffline)
			return err
		}
		if err := s.rt.Start(context.Background(), uuid); err != nil {
			s.reportState(uuid, dockerrt.StateOffline)
			return err
		}
		s.reportState(uuid, dockerrt.StateRunning)
		return nil

	case PowerStop:
		s.reportState(uuid, dockerrt.StateStopping)
		if err := s.rt.Stop(context.Background(), uuid, 10); err != nil {
			s.reportState(uuid, dockerrt.StateOffline)
			return err
		}
		s.reportState(uuid, dockerrt.StateOffline)
		return nil

	case PowerRestart:
		s.reportState(uuid, dockerrt.StateStopping)
		if err := s.rt.Restart(context.Background(), uuid, 10); err != nil {
			s.reportState(uuid, dockerrt.StateOffline)
			return err
		}
		s.reportState(uuid, dockerrt.StateRunning)
		return nil

	case PowerKill:
		if err := s.rt.Kill(context.Background(), uuid); err != nil {
			s.reportState(uuid, dockerrt.StateOffline)
			return err
		}
		s.reportState(uuid, dockerrt.StateOffline)
		return nil
	default:
		return fmt.Errorf("unknown power action %q", action)
	}
}

// reportState: simpan state lokal + kirim frame `state` ke panel.
func (s *Supervisor) reportState(uuid, status string) {
	s.mu.Lock()
	if st, ok := s.servers[uuid]; ok {
		st.status = status
	}
	s.mu.Unlock()
	if err := s.sendFrame(Frame{Ch: "rpc", Type: "state", Payload: mustJSON(map[string]string{
		"server_uuid": uuid, "status": status,
	})}); err != nil {
		log.Printf("state send %s: %v", uuid, err)
	}
}

// sendFrame: kirim frame ke panel (nil-safe: kalau WS putus, drop).
func (s *Supervisor) sendFrame(f Frame) error {
	s.mu.Lock()
	send := s.send
	s.mu.Unlock()
	if send == nil {
		return nil
	}
	return send(f)
}

// collectStats: satu snapshot stats. ContainerRuntime punya StatsOnce
// (opsional): Docker asli baca dari API, Fake untuk test.
func (s *Supervisor) collectStats(ctx context.Context, uuid string) (StatsSnapshot, error) {
	if st, ok := s.rt.(dockerrt.StatsProvider); ok {
		dSnap, err := st.StatsOnce(ctx, uuid)
		if err != nil {
			return StatsSnapshot{}, err
		}
		out := StatsSnapshot(dSnap)
		out.ServerUUID = uuid
		return out, nil
	}
	return StatsSnapshot{}, fmt.Errorf("runtime %T does not support stats", s.rt)
}

// WatchEvents: langganan event runtime (crash/stop dari luar) → report state.
// Dipanggil sekali saat daemon mulai.
func (s *Supervisor) WatchEvents(ctx context.Context) {
	ch := make(chan dockerrt.StateEvent, 64)
	go s.rt.Events(ctx, ch)
	go func() {
		for ev := range ch {
			// crash (die tanpa stop) dianggap offline — JANGAN restart otomatis
			// di MVP (SPEC §7.1). Panel menampilkan status terbaru.
			s.reportState(ev.Ref, ev.Status)
		}
	}()
}

// mustJSON: encode ke JSON (fallback "{}").
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
