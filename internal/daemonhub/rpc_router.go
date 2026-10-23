package daemonhub

import "sync"

// Router frame rpc panel (rpc:res / file:read-chunk dari daemon) — callback
// dipasang panel via SetRPCFrameHandler; daemonhub TIDAK import panel
// (arah dependensi tetap panel → daemonhub).
var (
	rpcRouterMu sync.Mutex
	rpcRouter   func(Frame)
)

// SetRPCFrameHandler pasang router frame rpc file manager (sekali saat start).
func SetRPCFrameHandler(fn func(Frame)) {
	rpcRouterMu.Lock()
	rpcRouter = fn
	rpcRouterMu.Unlock()
}

// DispatchRPCFrame panggil router (nil-safe) — dipanggil handler WS daemon.
func DispatchRPCFrame(f Frame) {
	rpcRouterMu.Lock()
	fn := rpcRouter
	rpcRouterMu.Unlock()
	if fn != nil {
		fn(f)
	}
}
