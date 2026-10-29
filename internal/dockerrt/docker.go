package dockerrt

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// dockerRT: implementasi asli ContainerRuntime di atas Docker SDK.
// Dibangun pakai client resmi (github.com/docker/docker/client) —
// semuanya tunduk ke interface ContainerRuntime di runtime.go.
type dockerRT struct {
	cli *client.Client
}

// NewDocker: buat runtime Docker asli. env kosong = default
// (unix:///var/run/docker.sock atau DOCKER_HOST).
func NewDocker() (*dockerRT, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &dockerRT{cli: cli}, nil
}

// DockerAvailable: cek daemon Docker benar-benar bisa dihubungi (Ping).
// NewDocker saja tidak cukup — client terbentuk walau socket tidak ada.
func DockerAvailable(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer cli.Close()
	if _, err := cli.Ping(ctx); err != nil {
		return fmt.Errorf("docker ping: %w", err)
	}
	return nil
}

// refToName: nama container = garuda-<ref> (nama host-side deterministic).
func refToName(ref string) string {
	return "garuda-" + strings.ToLower(strings.TrimSpace(ref))
}

func (d *dockerRT) Create(ctx context.Context, spec ContainerSpec) error {
	name := refToName(spec.Ref)
	// hapus container lama dengan nama sama (kalau ada) supaya create bisa ulang
	_ = d.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})

	hostCfg := &container.HostConfig{}
	if spec.MemoryMB > 0 {
		hostCfg.Memory = spec.MemoryMB * 1024 * 1024
	}
	if spec.CPUPercent > 0 {
		// cpu_percent di spec = persen dari SATU core.
		// Docker pakai nano-CPU: 100% = 1e9 (core penuh).
		hostCfg.NanoCPUs = int64(spec.CPUPercent) * 1e6
	}
	if spec.HostPath != "" {
		hostCfg.Binds = append(hostCfg.Binds, spec.HostPath+":/data")
	}
	if spec.BindPort > 0 {
		hostCfg.PortBindings = nat.PortMap{
			nat.Port(strconv.Itoa(spec.BindPort) + "/tcp"): []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: strconv.Itoa(spec.BindPort)},
			},
		}
	}

	cmd := spec.Cmd
	if len(cmd) == 0 {
		return fmt.Errorf("container %s: empty command", spec.Ref)
	}
	cfg := &container.Config{
		Image: spec.Image,
		Cmd:   cmd,
		Env:   spec.Env,
	}
	if spec.StopSignal != "" {
		cfg.StopSignal = spec.StopSignal
	}
	if _, err := d.cli.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name); err != nil {
		return fmt.Errorf("create container %s: %w", name, err)
	}
	return nil
}

func (d *dockerRT) Start(ctx context.Context, ref string) error {
	if err := d.cli.ContainerStart(ctx, refToName(ref), container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", ref, err)
	}
	return nil
}

func (d *dockerRT) Stop(ctx context.Context, ref string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	t := container.StopOptions{Timeout: &timeoutSec}
	if err := d.cli.ContainerStop(ctx, refToName(ref), t); err != nil {
		return fmt.Errorf("stop container %s: %w", ref, err)
	}
	return nil
}

func (d *dockerRT) Restart(ctx context.Context, ref string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	if err := d.cli.ContainerRestart(ctx, refToName(ref), container.StopOptions{Timeout: &timeoutSec}); err != nil {
		return fmt.Errorf("restart container %s: %w", ref, err)
	}
	return nil
}

func (d *dockerRT) Kill(ctx context.Context, ref string) error {
	if err := d.cli.ContainerKill(ctx, refToName(ref), "SIGKILL"); err != nil {
		return fmt.Errorf("kill container %s: %w", ref, err)
	}
	return nil
}

func (d *dockerRT) Remove(ctx context.Context, ref string, force bool) error {
	if err := d.cli.ContainerRemove(ctx, refToName(ref), container.RemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("remove container %s: %w", ref, err)
	}
	return nil
}

// State baca status container dari inspect (konversi ke 4 state daemon).
func (d *dockerRT) State(ctx context.Context, ref string) (string, error) {
	info, err := d.cli.ContainerInspect(ctx, refToName(ref))
	if err != nil {
		if client.IsErrNotFound(err) {
			return StateOffline, nil // container belum dibuat = offline
		}
		return "", fmt.Errorf("inspect container %s: %w", ref, err)
	}
	switch {
	case info.State == nil:
		return StateOffline, nil
	case info.State.Running && info.State.Restarting:
		return StateStarting, nil
	case info.State.Running:
		return StateRunning, nil
	default:
		return StateOffline, nil
	}
}

// ExecAttach buat exec TTY di dalam container. in = stdin (dari console WS),
// out = stdout+stderr digabung (TTY true di Docker SDK menggabungkan).
// Streaming murni: tidak ada buffer penuh, hanya salin langsung.
func (d *dockerRT) ExecAttach(ctx context.Context, ref string, in io.Reader, out io.Writer) (ExecHandle, error) {
	name := refToName(ref)
	execCfg := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}
	resp, err := d.cli.ContainerExecCreate(ctx, name, execCfg)
	if err != nil {
		return nil, fmt.Errorf("exec create %s: %w", name, err)
	}
	attach, err := d.cli.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return nil, fmt.Errorf("exec attach %s: %w", name, err)
	}
	// goroutine 1: salin in (stdin) → stream docker (jarang; console input).
	go func() {
		if _, err := io.Copy(attach.Conn, in); err != nil {
			// tutup sisi tulis supaya goroutine pemaca out juga tahu
			_ = attach.CloseWrite()
		}
	}()
	// goroutine 2: salin stream docker → out (stdout+stderr console).
	go func() {
		_, _ = io.Copy(out, attach.Reader)
		// biarkan attach tetap terbuka sampai Close dipanggil; caller yang
		// memutus koneksi WS console men-trigger ctx.Done → Close.
	}()
	return &dockerExecHandle{ctx: ctx, id: resp.ID, conn: &attach, cli: d.cli}, nil
}

type dockerExecHandle struct {
	ctx  context.Context
	id   string
	conn *types.HijackedResponse
	cli  *client.Client
}

// Resize mengubah ukuran TTY exec aktif (via REST exec resize endpoint).
func (h *dockerExecHandle) Resize(ctx context.Context, cols, rows uint) error {
	if ctx == nil {
		ctx = h.ctx
	}
	return h.cli.ContainerExecResize(ctx, h.id, container.ResizeOptions{
		Height: rows,
		Width:  cols,
	})
}

// Close tutup stream exec (stdin ditutup, reader berhenti). Sisi Docker
// akan terminate exec process-nya.
func (h *dockerExecHandle) Close() error {
	h.conn.CloseWrite()
	h.conn.Close()
	return nil
}

// Events: langganan Docker event API (container state changes) dan
// relay ke channel `ch` sebagai StateEvent. Goroutine tutup saat ctx done
// atau koneksi ke Docker putus (error).
func (d *dockerRT) Events(ctx context.Context, ch chan<- StateEvent) {
	go func() {
		defer close(ch)
		args := filters.NewArgs(
			filters.Arg("type", string(events.ContainerEventType)),
			filters.Arg("event", "start"),
			filters.Arg("event", "stop"),
			filters.Arg("event", "die"),
			filters.Arg("event", "kill"),
			filters.Arg("event", "oom"),
		)
		stream, errCh := d.cli.Events(ctx, events.ListOptions{Filters: args})
		if stream == nil {
			return // daemon reconnect saat runWS main loop retry
		}
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errCh:
				if err != nil {
					return
				}
			case msg := <-stream:
				// Docker API event Actor.ID = container ID; nama di Actor.Attributes["name"]
				ref := ""
				// Docker API event Actor.ID = container ID; nama ada di Actor.Attributes["name"]
				if nm, ok := msg.Actor.Attributes["name"]; ok {
					ref = strings.TrimPrefix(nm, "garuda-")
				}
				var status string
				switch msg.Action {
				case "start":
					status = StateRunning
				case "die", "stop", "kill", "oom":
					status = StateOffline
				default:
					continue
				}
				// jangan blocking: kalau consumer lambat, buang event
				select {
				case ch <- StateEvent{Ref: ref, Status: status}:
				default:
				}
			}
		}
	}()
}
