// Stats daemon: rangkum docker stats per container, kirim tiap 2 detik
// via WS ke panel (SPEC §7.6). Satu goroutine per server yang di-subscribe.
package daemon

import (
	"context"
	"log"
	"time"
)

// StatsSnapshot rangkuman resource satu server (dikirim tiap 2 detik).
type StatsSnapshot struct {
	ServerUUID   string  `json:"server_uuid"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemUsageMB   float64 `json:"mem_used_mb"`
	MemLimitMB   float64 `json:"mem_limit_mb"`
	NetRxKB      float64 `json:"net_rx_kb"`
	NetTxKB      float64 `json:"net_tx_kb"`
	BlockReadMB  float64 `json:"block_read_mb"`
	BlockWriteMB float64 `json:"block_write_mb"`
	UptimeSec    int64   `json:"uptime_seconds"`
	State        string  `json:"state"`
}

// startStats: mulai loop stats untuk uuid. Goroutine berhenti saat WS
// panel putus (stopStatsAll) atau daemon shutdown.
func (s *Supervisor) startStats(uuid string) {
	s.mu.Lock()
	if _, dup := s.statsCtxs[uuid]; dup {
		s.mu.Unlock()
		return // loop stats uuid ini sudah jalan
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.statsCtxs[uuid] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.statsCtxs, uuid)
			s.mu.Unlock()
		}()
		ticker := time.NewTicker(s.cfg.StatsInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap, err := s.collectStats(ctx, uuid)
				if err != nil {
					continue // container mati / runtime error: coba lagi nanti
				}
				snap.State = s.Status(uuid)
				if err := s.sendFrame(Frame{Ch: "stats", Type: "stat", Payload: mustJSON(snap)}); err != nil {
					log.Printf("stats send %s: %v", uuid, err)
				}
			}
		}
	}()
}

// stopStatsAll: hentikan semua loop stats (saat WS ke panel putus).
func (s *Supervisor) stopStatsAll() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.statsCtxs))
	for uuid, cancel := range s.statsCtxs {
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
		delete(s.statsCtxs, uuid)
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
