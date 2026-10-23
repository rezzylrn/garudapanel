// Package daemonhub: registry koneksi WebSocket daemon aktif + online/offline
// status + jembatan frame panel↔daemon (power, console, stats).
package daemonhub

import (
	"encoding/json"
	"sync"
)

// Frame: format pesan WS panel↔daemon (SPEC §4).
type Frame struct {
	Ch      string          `json:"ch"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ErrNotOnline: node tidak sedang tersambung.
var ErrNotOnline = &sendError{"node not online"}

type sendError struct{ msg string }

func (e *sendError) Error() string { return e.msg }

// Conn merepresentasikan satu daemon yang tersambung.
// send thread-safe via mutex (websocket write tidak boleh concurrent).
type Conn struct {
	NodeID int64
	Name   string

	mu   sync.Mutex
	send func(Frame) error
}

// BindSend pasang fungsi kirim frame ke daemon ini (dipanggil handler WS
// saat koneksi daemon hidup; dinil-kan saat putus).
func (c *Conn) BindSend(fn func(Frame) error) {
	c.mu.Lock()
	c.send = fn
	c.mu.Unlock()
}

// Send kirim satu frame ke daemon. Error bila koneksi putus/belum siap.
func (c *Conn) Send(f Frame) error {
	c.mu.Lock()
	fn := c.send
	c.mu.Unlock()
	if fn == nil {
		return ErrNotOnline
	}
	return fn(f)
}

// Hub menyimpan node yang sedang online (node_id → koneksi).
type Hub struct {
	mu     sync.RWMutex
	online map[int64]*Conn
	// onFrame: callback frame dari daemon (di-set panel console router)
	onFrame func(nodeID int64, f Frame)
}

func NewHub() *Hub {
	return &Hub{online: map[int64]*Conn{}}
}

// SetOnFrame: pasang router frame daemon (dipanggil sekali saat panel start).
func (h *Hub) SetOnFrame(fn func(nodeID int64, f Frame)) {
	h.mu.Lock()
	h.onFrame = fn
	h.mu.Unlock()
}

// DispatchFrame: dipanggil handler WS daemon untuk tiap frame masuk.
func (h *Hub) Dispatch(nodeID int64, f Frame) {
	h.mu.RLock()
	fn := h.onFrame
	h.mu.RUnlock()
	if fn != nil {
		fn(nodeID, f)
	}
}

// Add mendaftarkan koneksi daemon. Return true bila node sebelumnya offline.
func (h *Hub) Add(c *Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, wasOnline := h.online[c.NodeID]
	h.online[c.NodeID] = c
	return !wasOnline
}

// Remove melepas koneksi; return true bila memang yang terdaftar.
func (h *Hub) Remove(nodeID int64, c *Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.online[nodeID] == c {
		delete(h.online, nodeID)
		return true
	}
	return false
}

// Get ambil koneksi daemon aktif (nil kalau offline).
func (h *Hub) Get(nodeID int64) *Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.online[nodeID]
}

// IsOnline cek status koneksi node.
func (h *Hub) IsOnline(nodeID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.online[nodeID]
	return ok
}

// OnlineNodes daftar node_id yang online.
func (h *Hub) OnlineNodes() []int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]int64, 0, len(h.online))
	for id := range h.online {
		out = append(out, id)
	}
	return out
}
