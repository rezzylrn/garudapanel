// fileio: streaming read/write file antara panel ↔ daemon (SPEC §4/§9).
//
// Desain streaming murni (tanpa buffer file penuh di RAM):
//   - UPLOAD  : panel kirim frame {type:"file:write-begin"} → daemon buka
//     io.Pipe + FileOps.Write di goroutine → panel mengalirkan isi via frame
//     biner WS {type:"file:write-chunk"} → {type:"file:write-end"}.
//   - DOWNLOAD: panel kirim {type:"file:read-begin"} → daemon pompa isi file
//     sebagai frame {type:"file:read-chunk"} (32KB per frame) sampai habis.
//
// Frame tetap MessageText JSON? TIDAK untuk chunk — pakai base64 di field
// "d" (websocket nhooyr MessageBinary juga bisa, tapi JSON base64 konsisten
// dengan Frame format). Overhead base64 ~33% diterima untuk MVP.
package daemon

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
)

const fileChunkSize = 32 * 1024 // 32KB per frame chunk

// writeSession: sesi upload aktif (pipe → FileOps.Write di goroutine).
type writeSession struct {
	pw     *io.PipeWriter
	pr     *io.PipeReader
	done   chan error // hasil Write (bytes, err)
	wroteN int64
}

// fileIORegistry: sesi upload/download aktif per (server, path).
type fileIORegistry struct {
	mu     sync.Mutex
	writes map[string]*writeSession // key = uuid + ":" + path
}

func newFileIORegistry() *fileIORegistry {
	return &fileIORegistry{writes: map[string]*writeSession{}}
}

// handleWriteBegin: frame {ch:"rpc", type:"file:write-begin", payload:
// {server_uuid, path}}. Buka sesi upload; mulai goroutine Write streaming.
func (s *Supervisor) handleWriteBegin(payload []byte) error {
	var p struct {
		ServerUUID string `json:"server_uuid"`
		Path       string `json:"path"`
	}
	if err := unmarshal(payload, &p); err != nil || p.ServerUUID == "" || p.Path == "" {
		return errors.New("bad write-begin payload")
	}
	fops, err := s.fileOps(p.ServerUUID)
	if err != nil {
		return err
	}
	key := p.ServerUUID + ":" + p.Path
	s.fileIO.mu.Lock()
	if _, dup := s.fileIO.writes[key]; dup {
		s.fileIO.mu.Unlock()
		return errors.New("write session already open for this path")
	}
	s.fileIO.mu.Unlock()

	pr, pw := io.Pipe()
	ws := &writeSession{pw: pw, pr: pr, done: make(chan error, 1)}
	s.fileIO.mu.Lock()
	s.fileIO.writes[key] = ws
	s.fileIO.mu.Unlock()

	// goroutine: streaming write ke volume (quota + chown di FileOps).
	go func() {
		defer func() {
			s.fileIO.mu.Lock()
			// hapus sesi HANYA bila masih sesi ini (re-begin menimpa)
			if cur, ok := s.fileIO.writes[key]; ok && cur == ws {
				delete(s.fileIO.writes, key)
			}
			s.fileIO.mu.Unlock()
			pr.Close()
			pw.Close()
		}()
		n, werr := fops.Write(p.Path, pr, 0) // size 0 = quota dicek on-the-fly via scan
		ws.wroteN = n
		ws.done <- werr
		if werr != nil {
			log.Printf("fileio %s write %s: %v (wrote %d)", p.ServerUUID, p.Path, werr, n)
		}
	}()
	return nil
}

// handleWriteChunk: frame {type:"file:write-chunk", payload:{server_uuid,
// path, d:"<base64>"}} → tulis ke pipe. Blocking saat pipe penuh (backpressure
// alami — yang memaksa panel streaming pelan, BUKAN menumpuk di RAM).
func (s *Supervisor) handleWriteChunk(payload []byte) error {
	var p struct {
		ServerUUID string `json:"server_uuid"`
		Path       string `json:"path"`
		D          string `json:"d"`
	}
	if err := unmarshal(payload, &p); err != nil || p.ServerUUID == "" || p.Path == "" {
		return errors.New("bad write-chunk payload")
	}
	s.fileIO.mu.Lock()
	ws, ok := s.fileIO.writes[p.ServerUUID+":"+p.Path]
	s.fileIO.mu.Unlock()
	if !ok {
		return errors.New("no active write session")
	}
	data, err := base64.StdEncoding.DecodeString(p.D)
	if err != nil {
		return fmt.Errorf("bad base64 chunk: %w", err)
	}
	// blocking write → backpressure; batal via ctx bila WS putus (pw.Close).
	if _, err := ws.pw.Write(data); err != nil {
		return err
	}
	return nil
}

// handleWriteEnd: frame {type:"file:write-end", payload:{server_uuid, path}}
// → tutup pipe, tunggu hasil Write (balasan rpc:res {bytes|error}).
func (s *Supervisor) handleWriteEnd(payload []byte) {
	var p struct {
		ServerUUID string `json:"server_uuid"`
		Path       string `json:"path"`
		ReqID      string `json:"req_id"`
	}
	if err := unmarshal(payload, &p); err != nil || p.ServerUUID == "" {
		return
	}
	s.fileIO.mu.Lock()
	key := p.ServerUUID + ":" + p.Path
	ws, ok := s.fileIO.writes[key]
	s.fileIO.mu.Unlock()
	if !ok {
		s.replyRPC(p.ReqID, nil, errors.New("no active write session"))
		return
	}
	_ = ws.pw.Close() // EOF → goroutine Write selesai
	err := <-ws.done  // hasil streaming write (bytes sudah di ws.wroteN)
	if err != nil {
		s.replyRPC(p.ReqID, nil, err)
		return
	}
	s.replyRPC(p.ReqID, map[string]any{"bytes": ws.wroteN}, nil)
}

// handleReadBegin: frame {type:"file:read-begin"} → pompa isi file sebagai
// frame file:read-chunk (32KB) lalu file:read-end. Streaming murni — tidak
// ada buffer penuh; tiap chunk dibaca lalu dikirim.
func (s *Supervisor) handleReadBegin(payload []byte) {
	var p struct {
		ServerUUID string `json:"server_uuid"`
		Path       string `json:"path"`
		ReqID      string `json:"req_id"`
	}
	if err := unmarshal(payload, &p); err != nil || p.ServerUUID == "" || p.Path == "" {
		return
	}
	fops, err := s.fileOps(p.ServerUUID)
	if err != nil {
		s.replyRPC(p.ReqID, nil, err)
		return
	}
	rc, err := fops.Read(p.Path)
	if err != nil {
		s.replyRPC(p.ReqID, nil, err)
		return
	}
	defer rc.Close()
	buf := make([]byte, fileChunkSize)
	for {
		n, rerr := rc.Read(buf)
		if n > 0 {
			chunk := base64.StdEncoding.EncodeToString(buf[:n])
			// frame read-chunk (cukup kecil; WS putus → error → stop)
			if err := s.sendFrame(Frame{Ch: "rpc", Type: "file:read-chunk", Payload: mustJSON(map[string]string{
				"req_id": p.ReqID, "d": chunk,
			})}); err != nil {
				log.Printf("fileio read %s/%s: %v", p.ServerUUID, p.Path, err)
				return
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			s.replyRPC(p.ReqID, nil, rerr)
			return
		}
	}
	// selesai: balas rpc:res {bytes: total} — total dihitung panel dari chunk
	s.replyRPC(p.ReqID, map[string]any{"done": true}, nil)
}

// replyRPC kirim rpc:res dengan req_id (dipakai write-end & read-begin).
func (s *Supervisor) replyRPC(reqID string, result any, err error) {
	resp := map[string]any{"req_id": reqID}
	if err != nil {
		resp["error"] = err.Error()
	} else {
		resp["result"] = result
	}
	_ = s.sendFrame(Frame{Ch: "rpc", Type: "rpc:res", Payload: mustJSON(resp)})
}
