package daemon

import (
	"encoding/json"
)

// Frame: format pesan WS panel↔daemon (SPEC §4). Duplikasi tipe di
// daemonhub supaya daemon tidak perlu import package panel.
type Frame struct {
	Ch      string          `json:"ch"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// unmarshal helper: decode payload frame.
func unmarshal(raw []byte, v any) error {
	if len(raw) == 0 {
		return nil // payload kosong dianggap zero value
	}
	return json.Unmarshal(raw, v)
}
