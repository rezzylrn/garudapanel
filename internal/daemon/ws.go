package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

// heartbeatInterval: kirim heartbeat tiap 10 detik.
const heartbeatInterval = 10 * time.Second

// runWS membuka reverse WebSocket ke panel, memasang supervisor (Attach),
// memompa heartbeat, dan mem-broadcast frame panel ke supervisor.
// Return error saat koneksi putus; caller yang reconnect.
func runWS(ctx context.Context, opts JoinOptions, nodeToken, nodeName string, sup *Supervisor) error {
	wsURL, err := wsURLFrom(opts.PanelURL)
	if err != nil {
		return err
	}
	hdr := http.Header{}
	hdr.Set("X-Node-Token", nodeToken)
	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dialCancel()
	c, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer c.Close(websocket.StatusInternalError, "")
	log.Printf("connected to panel ws")

	// satu goroutine tulis: semua frame (supervisor + heartbeat) lewat sini —
	// websocket.Conn tidak boleh ditulis concurrent.
	writeCh := make(chan Frame, 128)
	writeDone := make(chan error, 1)
	go func() {
		defer close(writeDone)
		for {
			select {
			case f := <-writeCh:
				if err := sendFrame(ctx, c, f); err != nil {
					writeDone <- err
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// pasang supervisor: semua kirim frame supervisor lewat channel ini.
	// Kalau channel penuh (panel lambat), frame console/stats di-drop —
	// ring buffer daemon tetap menyimpan log untuk replay.
	sup.Attach(func(f Frame) error {
		select {
		case writeCh <- f:
			return nil
		default:
			return nil // drop (panel lambat); ring buffer tetap rekam
		}
	})
	defer sup.Detach()

	// hello frame (nama node asli — bukan "resume", review milestone-1)
	hello, _ := json.Marshal(map[string]any{"name": nodeName, "version": "0.2.0"})
	if err := sendFrame(ctx, c, Frame{Ch: "rpc", Type: "hello", Payload: hello}); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	// goroutine baca: frame panel → supervisor (Read boleh paralel dengan
	// Write di websocket.Conn; yang dilarang tulis-kirim bersamaan).
	readDone := make(chan error, 1)
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				readDone <- err
				return
			}
			var f Frame
			if json.Unmarshal(data, &f) == nil {
				sup.HandleFrame(f)
			}
		}
	}()

	// loop utama WS: tunggu error baca/tulis, kirim heartbeat tiap 10 detik.
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-readDone:
			return fmt.Errorf("read loop: %w", err)
		case err := <-writeDone:
			return fmt.Errorf("write loop: %w", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			hb, _ := json.Marshal(map[string]any{"ts": time.Now().Unix()})
			select {
			case writeCh <- Frame{Ch: "rpc", Type: "heartbeat", Payload: hb}:
			default: // drop heartbeat kalau penuh (frame penting utama dulu)
			}
		}
	}
}

// sendFrame tulis satu frame JSON ke WS dengan timeout singkat.
func sendFrame(ctx context.Context, c *websocket.Conn, f Frame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.Write(wctx, websocket.MessageText, data)
}

// wsURLFrom: http(s)://host → ws(s)://host/api/daemon/ws
func wsURLFrom(panelURL string) (string, error) {
	u, err := url.Parse(panelURL)
	if err != nil {
		return "", fmt.Errorf("invalid panel url: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api/daemon/ws"
	return u.String(), nil
}
