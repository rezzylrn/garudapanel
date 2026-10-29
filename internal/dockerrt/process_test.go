package dockerrt

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProcessRuntimeRunStop(t *testing.T) {
	dir := t.TempDir()
	rt := NewProcess()
	ctx := context.Background()
	spec := ContainerSpec{
		Ref:        "test-js",
		Cmd:        []string{"/bin/bash", "-c", "echo hello-running; sleep 30"},
		HostPath:   dir,
		StopSignal: "SIGINT",
	}
	if err := rt.Create(ctx, spec); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := rt.Start(ctx, "test-js"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st, err := rt.State(ctx, "test-js")
	if err != nil || st != StateRunning {
		t.Fatalf("State = %q, err %v", st, err)
	}
	// Output mengalir via ExecAttach (replay + live).
	var out bytes.Buffer
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()
	h, err := rt.ExecAttach(ctx, "test-js", pr, &out)
	if err != nil {
		t.Fatalf("ExecAttach: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(out.String(), "hello-running") {
		time.Sleep(50 * time.Millisecond)
	}
	_ = h.Close()
	if !strings.Contains(out.String(), "hello-running") {
		t.Fatalf("output tidak mengandung hello-running: %q", out.String())
	}
	if err := rt.Stop(ctx, "test-js", 3); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	st, _ = rt.State(ctx, "test-js")
	if st != StateOffline {
		t.Fatalf("State setelah Stop = %q", st)
	}
}

func TestProcessRuntimeExitEvent(t *testing.T) {
	dir := t.TempDir()
	rt := NewProcess()
	ctx := context.Background()
	_ = rt.Create(ctx, ContainerSpec{
		Ref:      "test-exit",
		Cmd:      []string{"/bin/bash", "-c", "echo bye; exit 0"},
		HostPath: dir,
	})
	ch := make(chan StateEvent, 8)
	evCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go rt.Events(evCtx, ch)
	if err := rt.Start(ctx, "test-exit"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	sawOffline := false
	for time.Now().Before(deadline) && !sawOffline {
		select {
		case ev := <-ch:
			if ev.Ref == "test-exit" && ev.Status == StateOffline {
				sawOffline = true
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !sawOffline {
		t.Fatal("tidak ada event offline setelah proses exit")
	}
}

func TestProcessRuntimeNodeScript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node tidak tersedia")
	}
	dir := t.TempDir()
	script := "console.log('node-running'); setInterval(()=>console.log('tick'), 200);"
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	rt := NewProcess()
	ctx := context.Background()
	_ = rt.Create(ctx, ContainerSpec{
		Ref:      "test-node",
		Cmd:      []string{"/bin/bash", "-c", "node index.js"},
		HostPath: dir,
		Env:      []string{"CMD_RUN=npm start"},
	})
	if err := rt.Start(ctx, "test-node"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer rt.Kill(ctx, "test-node")
	var out bytes.Buffer
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()
	h, err := rt.ExecAttach(ctx, "test-node", pr, &out)
	if err != nil {
		t.Fatalf("ExecAttach: %v", err)
	}
	defer h.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(out.String(), "node-running") {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(out.String(), "node-running") {
		t.Fatalf("output node tidak ditemukan: %q", out.String())
	}
}
