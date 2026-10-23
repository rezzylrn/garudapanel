// Stats asli via Docker API (container stats one-shot).
package dockerrt

import (
	"context"
	"fmt"

	v1 "github.com/docker/docker/api/types/container"
)

// StatsOnce: ambil satu snapshot stats container (stream=false) lalu ringkas.
// Satu HTTP request per panggilan — tidak ada stream terbuka antar poll.
func (d *dockerRT) StatsOnce(ctx context.Context, ref string) (StatsSnapshot, error) {
	resp, err := d.cli.ContainerStatsOneShot(ctx, refToName(ref))
	if err != nil {
		return StatsSnapshot{}, fmt.Errorf("stats %s: %w", ref, err)
	}
	defer resp.Body.Close()
	var st v1.StatsResponse
	if err := jsonDecode(resp.Body, &st); err != nil {
		return StatsSnapshot{}, fmt.Errorf("stats decode %s: %w", ref, err)
	}
	return summarize(st), nil
}

// summarize: konversi stats mentah Docker ke rangkuman daemon.
func summarize(st v1.StatsResponse) StatsSnapshot {
	s := StatsSnapshot{}
	// CPU: perubahan cpu usage vs system delta (rumus docker CLI).
	cpuDelta := float64(st.CPUStats.CPUUsage.TotalUsage) - float64(st.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(st.CPUStats.SystemUsage) - float64(st.PreCPUStats.SystemUsage)
	if sysDelta > 0 {
		// docker CLI: % = (cpuDelta/sysDelta) * onlineCPUs * 100
		online := float64(st.CPUStats.OnlineCPUs)
		if online == 0 {
			online = float64(len(st.CPUStats.CPUUsage.PercpuUsage))
		}
		s.CPUPercent = (cpuDelta / sysDelta) * online * 100.0
	}
	s.MemUsageMB = float64(st.MemoryStats.Usage-st.MemoryStats.Stats["cache"]) / (1024 * 1024)
	if st.MemoryStats.Limit > 0 {
		s.MemLimitMB = float64(st.MemoryStats.Limit) / (1024 * 1024)
	}
	for _, v := range st.Networks {
		s.NetRxKB += float64(v.RxBytes) / 1024
		s.NetTxKB += float64(v.TxBytes) / 1024
	}
	for _, e := range st.BlkioStats.IoServiceBytesRecursive {
		switch e.Op {
		case "read", "Read":
			s.BlockReadMB += float64(e.Value) / (1024 * 1024)
		case "write", "Write":
			s.BlockWriteMB += float64(e.Value) / (1024 * 1024)
		}
	}
	return s
}
