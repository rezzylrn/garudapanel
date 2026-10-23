// Package daemon: sisi daemon — tukar join token jadi node token long-lived,
// buka reverse WebSocket ke panel, kirim heartbeat tiap 10 detik.
package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"garudapanel/internal/dockerrt"
)

// JoinOptions konfigurasi daemon join.
type JoinOptions struct {
	PanelURL    string // mis. http://127.0.0.1:8080
	JoinToken   string // sekali pakai (kosong kalau sudah punya ResumeToken)
	ResumeToken string // node token long-lived tersimpan (mode restart)
	StorePath   string // file simpan node token
	DataDir     string // dir data daemon (volume server)
}

// Run menjalankan daemon: join → buat supervisor → WS connect → heartbeat
// sampai sinyal exit. Daemon = source of truth state container (SPEC §7.1).
func Join(opts JoinOptions) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Dapatkan node token: resume (sudah join sebelumnya) atau tukar join token.
	var nodeToken, nodeName string
	if opts.ResumeToken != "" {
		nodeToken = opts.ResumeToken
		// nama node asli diminta dari panel via hello-respone (WS handler
		// panel balas frame `welcome` berisi nama node). "resume" hanya
		// placeholder sampai panel konfirmasi.
		nodeName = ""
	} else {
		var err error
		nodeToken, nodeName, err = exchangeJoinToken(ctx, opts)
		if err != nil {
			return fmt.Errorf("join: %w", err)
		}
		log.Printf("joined as node %q; node token saved to %s", nodeName, opts.StorePath)
	}

	// 2. Supervisor: container runtime asli (Docker) — semua state container
	//    lewat sini (SPEC §7.1: daemon satu-satunya yang boleh ubah state).
	rt, err := dockerrt.NewDocker()
	if err != nil {
		return fmt.Errorf("docker runtime: %w", err)
	}
	sup := NewSupervisor(rt, SupervisorConfig{
		DataDir: opts.DataDir,
	})
	sup.WatchEvents(ctx)

	// 3. Loop utama: sambung WS, kirim hello + heartbeat, reconnect otomatis.
	for {
		if err := runWS(ctx, opts, nodeToken, nodeName, sup); err != nil {
			log.Printf("ws: %v", err)
		}
		select {
		case <-ctx.Done():
			log.Printf("daemon: shutdown")
			return nil
		case <-time.After(3 * time.Second):
			log.Printf("reconnecting to panel...")
		}
	}
}

// exchangeJoinToken POST /api/daemon/join dengan join token sekali pakai.
func exchangeJoinToken(ctx context.Context, opts JoinOptions) (nodeToken, nodeName string, err error) {
	log.Printf("joining panel %s ...", opts.PanelURL)
	body, _ := json.Marshal(map[string]string{"join_token": opts.JoinToken})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(opts.PanelURL, "/")+"/api/daemon/join", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request panel: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var er struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&er)
		return "", "", fmt.Errorf("panel returned %d: %s", resp.StatusCode, er.Error)
	}
	var out struct {
		NodeToken string `json:"node_token"`
		NodeName  string `json:"node_name"`
		NodeID    int64  `json:"node_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode response: %w", err)
	}
	if out.NodeToken == "" {
		return "", "", fmt.Errorf("panel returned empty node token")
	}
	// simpan node token ke disk agar restart tidak perlu join ulang
	if err := saveNodeToken(opts.StorePath, out.NodeToken); err != nil {
		log.Printf("warning: save node token: %v (token hanya hidup di memori sesi ini)", err)
	}
	return out.NodeToken, out.NodeName, nil
}

func saveNodeToken(path, token string) error {
	if path == "" {
		return fmt.Errorf("empty store path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0o600)
}

// LoadNodeToken membaca node token tersimpan (dipakai `garuda daemon` tanpa --join).
func LoadNodeToken(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("empty token file")
	}
	return tok, nil
}
