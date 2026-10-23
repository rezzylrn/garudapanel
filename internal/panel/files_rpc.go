// fileio_rpc: infra RPC panel → daemon untuk file manager (SPEC §4).
// rpc:res dicocokkan via req_id; file:read-chunk dialirkan ke sink (download
// streaming). Dipakai files_handlers.go untuk semua operasi file.
package panel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	"garudapanel/internal/daemonhub"
)

// rpcEvent satu kejadian rpc dari daemon untuk sebuah req_id.
type rpcEvent struct {
	ChunkB64 string // non-kosong = frame file:read-chunk (streaming download)
	Final    *rpcFinal
}

// rpcFinal: balasan akhir rpc:res.
type rpcFinal struct {
	Result json.RawMessage
	Err    error
}

// rpcWait: registry req_id → channel kejadian.
var rpcWait = struct {
	mu   sync.Mutex
	wait map[string]chan rpcEvent
}{wait: map[string]chan rpcEvent{}}

func errFromStr(s string) error {
	if s == "" {
		return nil
	}
	return errors.New(s)
}

// resolveRPCWait: router frame rpc:res dari daemon (dipasang di daemonhub).
func resolveRPCWait(payload []byte) {
	var p struct {
		ReqID  string          `json:"req_id"`
		Result json.RawMessage `json:"result,omitempty"`
		Error  string          `json:"error,omitempty"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}
	rpcWait.mu.Lock()
	ch, ok := rpcWait.wait[p.ReqID]
	if ok {
		delete(rpcWait.wait, p.ReqID)
	}
	rpcWait.mu.Unlock()
	if ok {
		ch <- rpcEvent{Final: &rpcFinal{Result: p.Result, Err: errFromStr(p.Error)}}
	}
}

// deliverReadChunk: router frame file:read-chunk dari daemon.
func deliverReadChunk(payload []byte) {
	var p struct {
		ReqID string `json:"req_id"`
		D     string `json:"d"`
	}
	if json.Unmarshal(payload, &p) != nil || p.ReqID == "" {
		return
	}
	rpcWait.mu.Lock()
	ch, ok := rpcWait.wait[p.ReqID]
	rpcWait.mu.Unlock()
	if ok {
		select {
		case ch <- rpcEvent{ChunkB64: p.D}:
		default: // antre penuh → drop (browser lambat)
		}
	}
}

// DaemonRPC kirim frame rpc ke node + tunggu rpc:res dengan req_id.
// onChunk (opsional): dipanggil tiap file:read-chunk yang cocok (streaming).
func DaemonRPC(s *Server, nodeID int64, frameType string, payload map[string]any, timeout time.Duration, onChunk func(string) error) (json.RawMessage, error) {
	conn := s.Hub.Get(nodeID)
	if conn == nil {
		return nil, errors.New("node offline")
	}
	reqID := newReqID()
	payload["req_id"] = reqID
	rpcWait.mu.Lock()
	ch := make(chan rpcEvent, 256)
	rpcWait.wait[reqID] = ch
	rpcWait.mu.Unlock()
	defer func() {
		rpcWait.mu.Lock()
		delete(rpcWait.wait, reqID)
		rpcWait.mu.Unlock()
	}()
	body, _ := json.Marshal(payload)
	if err := conn.Send(daemonhub.Frame{Ch: "rpc", Type: frameType, Payload: body}); err != nil {
		return nil, err
	}
	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if ev.Final == nil {
				if onChunk != nil {
					if err := onChunk(ev.ChunkB64); err != nil {
						return nil, err
					}
				}
				continue
			}
			if ev.Final.Err != nil {
				return nil, ev.Final.Err
			}
			return ev.Final.Result, nil
		case <-deadline:
			return nil, errors.New("daemon rpc timeout")
		}
	}
}

// newReqID: id request acak (hex 16 char) — crypto/rand.
func newReqID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
