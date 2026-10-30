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
	"garudapanel/internal/filecore"
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
	rt        dockerrt.ContainerRuntime
	cfg       SupervisorConfig
	send      SendFrame // kirim frame ke panel (diset setiap reconnect)
	volumes   *filecore.Volumes
	fileIO    *fileIORegistry
	mu        sync.Mutex
	servers   map[string]*serverState       // uuid → state
	consoles  map[string]*consoleHub        // uuid → hub console
	rings     map[string]*RingBuffer        // uuid → ring buffer log
	statsCtxs map[string]context.CancelFunc // uuid → cancel loop stats
}

// stopCommandToSignal maps a Pterodactyl-style stop command to a Docker
// stop signal. "^C" (the Pterodactyl convention) means SIGINT; a bare signal
// name is passed through; anything else falls back to SIGTERM (Wings would
// write it to stdin — not yet supported here).
func stopCommandToSignal(cmd string) string {
	switch cmd {
	case "^C", "SIGINT":
		return "SIGINT"
	case "SIGTERM", "SIGKILL", "SIGHUP", "SIGQUIT":
		return cmd
	default:
		return "SIGTERM"
	}
}

// NewSupervisor: buat supervisor dengan runtime.
func NewSupervisor(rt dockerrt.ContainerRuntime, cfg SupervisorConfig) *Supervisor {
	if cfg.StatsInterval <= 0 {
		cfg.StatsInterval = 2 * time.Second
	}
	return &Supervisor{
		rt:        rt,
		cfg:       cfg,
		volumes:   filecore.NewVolumes(nil), // osFS asli; test injeksi factory fake
		fileIO:    newFileIORegistry(),
		servers:   map[string]*serverState{},
		consoles:  map[string]*consoleHub{},
		rings:     map[string]*RingBuffer{},
		statsCtxs: map[string]context.CancelFunc{},
	}
}

// Register: daemon tahu ada server dengan uuid + spec. ImageUser kosong
// di-resolve dari image config via runtime (SPEC §7.4) — fallback 1000:1000.
func (s *Supervisor) Register(uuid string, spec dockerrt.ContainerSpec) {
	if spec.ImageUser == "" {
		if p, ok := s.rt.(dockerrt.ImageUserProvider); ok {
			if user, err := p.ImageUser(context.Background(), spec.Image); err == nil {
				spec.ImageUser = user
			} else {
				log.Printf("register %s: image user resolve: %v (fallback 1000:1000)", uuid, err)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.servers[uuid]; ok {
		st.spec = spec
		return
	}
	s.servers[uuid] = &serverState{uuid: uuid, spec: spec, status: dockerrt.StateOffline}
}

// SyncServers: ganti seluruh registry dengan daftar dari panel.
// Server yang tidak ada di daftar akan di-unregister (mis. setelah dihapus
// dari panel). Dipanggil saat menerima servers.sync.
func (s *Supervisor) SyncServers(specs map[string]dockerrt.ContainerSpec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for uuid := range s.servers {
		if _, ok := specs[uuid]; !ok {
			delete(s.servers, uuid)
			log.Printf("sync: unregistered removed server %s", uuid)
		}
	}
	for uuid, spec := range specs {
		if st, ok := s.servers[uuid]; ok {
			st.spec = spec
		} else {
			s.servers[uuid] = &serverState{uuid: uuid, spec: spec, status: dockerrt.StateOffline}
		}
	}
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
				UUID        string   `json:"uuid"`
				Image       string   `json:"image"`
				Cmd         []string `json:"cmd"`
				Env         []string `json:"env"`
				MemoryMB    int64    `json:"memory_mb"`
				CPUPercent  int      `json:"cpu_percent"`
				StopCommand string   `json:"stop_command"`
				Port        int      `json:"port"`
				DiskMB      int64    `json:"disk_mb"`
				ImageUser   string   `json:"image_user"`
				StartupDone string   `json:"startup_done"`
			} `json:"servers"`
		}
		if err := unmarshal(f.Payload, &p); err != nil {
			log.Printf("servers.sync: bad payload: %v", err)
			return
		}
		specs := make(map[string]dockerrt.ContainerSpec, len(p.Servers))
		for _, sv := range p.Servers {
			spec := dockerrt.ContainerSpec{
				Ref:         sv.UUID,
				Image:       sv.Image,
				Cmd:         sv.Cmd,
				Env:         sv.Env,
				MemoryMB:    sv.MemoryMB,
				CPUPercent:  sv.CPUPercent,
				StopSignal:  stopCommandToSignal(sv.StopCommand),
				BindPort:    sv.Port,
				HostPath:    s.volumeFor(sv.UUID),
				DiskMB:      sv.DiskMB,
				ImageUser:   sv.ImageUser,
				StartupDone: sv.StartupDone,
			}
			// Resolve image user jika kosong.
			if spec.ImageUser == "" {
				if ip, ok := s.rt.(dockerrt.ImageUserProvider); ok {
					if user, err := ip.ImageUser(context.Background(), spec.Image); err == nil {
						spec.ImageUser = user
					}
				}
			}
			specs[sv.UUID] = spec
		}
		s.SyncServers(specs)
		log.Printf("servers sync: %d registered", len(specs))

	case f.Ch == "rpc" && f.Type == "file":
		// file manager ops (list/mkdir/delete/rename/extract/usage) —
		// goroutine terpisah: operasi disk bisa lambat, jangan blok read loop.
		// Reply: rpc:res dengan req_id dari request (pencocokan di panel).
		go func(payload []byte) {
			var reqID string
			var p struct {
				ReqID string `json:"req_id"`
			}
			if unmarshal(payload, &p) == nil {
				reqID = p.ReqID
			}
			res, err := s.handleFileRPC(payload)
			resp := map[string]any{"req_id": reqID}
			if err != nil {
				log.Printf("file rpc: %v", err)
				resp["error"] = err.Error()
			} else {
				resp["result"] = res
			}
			_ = s.sendFrame(Frame{Ch: "rpc", Type: "rpc:res", Payload: mustJSON(resp)})
		}(f.Payload)

	case f.Ch == "rpc" && f.Type == "file:write-begin":
		// write-begin HARUS sinkron di read loop, sama seperti chunk:
		// begin yang di-goroutine bisa kalah balapan dengan chunk yang
		// datang tepat di belakangnya (panel kirim begin+chunk+end
		// berurutan dalam satu flush) → chunk gagal "no active write
		// session" dan file tertulis 0 byte. handleWriteBegin sendiri
		// cepat (registrasi + spawn goroutine writer), tidak memblokir.
		if err := s.handleWriteBegin(f.Payload); err != nil {
			log.Printf("file write-begin: %v", err)
		}
	case f.Ch == "rpc" && f.Type == "file:write-chunk":
		// chunk di pump langsung (backpressure blocking): TIDAK di goroutine
		// terpisah, supaya urutan chunk terjaga. Bila pipe penuh, read loop
		// menunggu — panel mengirim dengan flow control.
		if err := s.handleWriteChunk(f.Payload); err != nil {
			log.Printf("file write-chunk: %v", err)
		}
	case f.Ch == "rpc" && f.Type == "file:write-end":
		go s.handleWriteEnd(f.Payload)
	case f.Ch == "rpc" && f.Type == "file:read-begin":
		go s.handleReadBegin(f.Payload)

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
		// Status "running" TIDAK dilaporkan di sini: runtime yang
		// melaporkannya lewat event — ProcessRuntime saat teks
		// startup-done muncul di output (atau langsung bila egg tidak
		// punya teks done), Docker saat container start, FakeRuntime
		// tidak melapor event sehingga supervisor melaporkannya di bawah
		// bila spec tidak punya teks done.
		if st.spec.StartupDone == "" {
			s.reportState(uuid, dockerrt.StateRunning)
		}
		s.consolePrint(uuid, "[GarudaPanel] Server started.")
		return nil

	case PowerStop:
		s.reportState(uuid, dockerrt.StateStopping)
		if err := s.rt.Stop(context.Background(), uuid, 10); err != nil {
			s.reportState(uuid, dockerrt.StateOffline)
			return err
		}
		s.reportState(uuid, dockerrt.StateOffline)
		s.consolePrint(uuid, "[GarudaPanel] Server stopped.")
		return nil

	case PowerRestart:
		s.reportState(uuid, dockerrt.StateStopping)
		if err := s.rt.Restart(context.Background(), uuid, 10); err != nil {
			s.reportState(uuid, dockerrt.StateOffline)
			return err
		}
		// Sama seperti PowerStart: status running datang dari event
		// runtime (teks startup-done), bukan dari sini.
		if st.spec.StartupDone == "" {
			s.reportState(uuid, dockerrt.StateRunning)
		}
		s.consolePrint(uuid, "[GarudaPanel] Server restarted.")
		return nil

	case PowerKill:
		if err := s.rt.Kill(context.Background(), uuid); err != nil {
			s.reportState(uuid, dockerrt.StateOffline)
			return err
		}
		s.reportState(uuid, dockerrt.StateOffline)
		s.consolePrint(uuid, "[GarudaPanel] Server stopped (forced).")
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
		out := StatsSnapshot{
			ServerUUID:   dSnap.ServerUUID,
			CPUPercent:   dSnap.CPUPercent,
			MemUsageMB:   dSnap.MemUsageMB,
			MemLimitMB:   dSnap.MemLimitMB,
			NetRxKB:      dSnap.NetRxKB,
			NetTxKB:      dSnap.NetTxKB,
			BlockReadMB:  dSnap.BlockReadMB,
			BlockWriteMB: dSnap.BlockWriteMB,
			UptimeSec:    dSnap.UptimeSec,
		}
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
