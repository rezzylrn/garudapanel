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

	"garudapanel/internal/daemonhub"
)

// heartbeatInterval: kirim heartbeat tiap 10 detik.
const heartbeatInterval = 10 * time.Second

// runWS membuka reverse WebSocket ke panel dan memompa heartbeat.
// Return error saat koneksi putus; caller yang reconnect.
func runWS(ctx context.Context, opts JoinOptions, nodeToken, nodeName string) error {
	wsURL, err := wsURLFrom(opts.PanelURL)
	if err != nil {
		return err
	}
	hdr := http.Header{}
	hdr.Set("X-Node-Token", nodeToken)
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: hdr})
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	log.Printf("connected to panel ws")

	// hello frame
	hello, _ := json.Marshal(map[string]any{"name": nodeName, "version": "0.1.0"})
	if err := sendFrame(ctx, c, daemonhub.Frame{Ch: "rpc", Type: "hello", Payload: hello}); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	// goroutine baca frame dari panel (fase 1: log saja; power/console fase 2)
	readDone := make(chan error, 1)
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				readDone <- err
				return
			}
			var f daemonhub.Frame
			if json.Unmarshal(data, &f) == nil {
				log.Printf("panel frame: ch=%s type=%s", f.Ch, f.Type)
			}
		}
	}()

	// heartbeat loop tiap 10 detik
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-readDone:
			return fmt.Errorf("read loop: %w", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			hb, _ := json.Marshal(map[string]any{"ts": time.Now().Unix()})
			if err := sendFrame(ctx, c, daemonhub.Frame{Ch: "rpc", Type: "heartbeat", Payload: hb}); err != nil {
				return fmt.Errorf("heartbeat: %w", err)
			}
		}
	}
}

// sendFrame tulis satu frame JSON ke WS dengan timeout singkat.
func sendFrame(ctx context.Context, c *websocket.Conn, f daemonhub.Frame) error {
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
