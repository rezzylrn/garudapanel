// Package daemonhub: registry koneksi WebSocket daemon aktif + online/offline
// status. Fase 1: hanya heartbeat; power/console dirilis di fase berikutnya.
package daemonhub

import (
	"log"
	"sync"
)

// Hub menyimpan node yang sedang online (node_id → koneksi).
type Hub struct {
	mu     sync.RWMutex
	online map[int64]*Conn
}

// Conn merepresentasikan satu daemon yang tersambung.
// Fase 1: hanya menandai "alive"; frame handling ada di panel.ws.go.
type Conn struct {
	NodeID int64
	Name   string
}

// OnOffline dipanggil saat node disconnect (untuk status server → offline).
type OnOffline func(nodeID int64)

func NewHub() *Hub {
	return &Hub{online: map[int64]*Conn{}}
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

func logf(format string, args ...any) { log.Printf("hub: "+format, args...) }
