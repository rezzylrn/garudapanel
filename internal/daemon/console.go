// Console daemon: sesi console live per server (SPEC §7.2).
// Sesi = exec TTY di container + 2 goroutine (stdin pump + stdout pump).
// Tutup bersih saat WS ke panel putus — tanpa goroutine leak (test wajib).
package daemon

import (
	"context"
	"io"
	"log"
	"sync"

	"garudapanel/internal/dockerrt"
)

const ringBufferLimit = 100 * 1024 // 100KB log terakhir per container (SPEC §4)
const maxConsoleSessions = 30      // SPEC §4: maks 30 sesi console per server

// consoleHub: kumpulan sesi console satu server + ring buffer-nya.
type consoleHub struct {
	uuid  string
	ring  *RingBuffer
	mu    sync.Mutex
	conns map[string]*consoleSession // id sesi (dari panel) → sesi
}

// consoleSession: satu sesi console aktif (satu browser via panel).
type consoleSession struct {
	id     string
	cancel context.CancelFunc  // batal → stdout pump berhenti + handle.Close
	stdin  io.WriteCloser      // input dari panel/console browser
	handle dockerrt.ExecHandle // handle exec (resize/close) — nil sampai attach selesai
}

// consoleHubFor: ambil (atau buat) hub console untuk uuid.
func (s *Supervisor) consoleHubFor(uuid string) *consoleHub {
	s.mu.Lock()
	h, ok := s.consoles[uuid]
	if !ok {
		h = &consoleHub{uuid: uuid, conns: map[string]*consoleSession{}}
		s.consoles[uuid] = h
	}
	s.mu.Unlock()
	h.ring = s.ringFor(uuid) // ringFor lock sendiri — di luar s.mu (no deadlock)
	return h
}

// ringFor: ring buffer per uuid (dibuat lazy).
func (s *Supervisor) ringFor(uuid string) *RingBuffer {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rings[uuid]
	if !ok {
		r = NewRingBuffer(ringBufferLimit)
		s.rings[uuid] = r
	}
	return r
}

// open: buka sesi console baru (panel kirim frame console:<uuid> type=open).
// Maks 30 sesi per server (SPEC §4).
func (s *Supervisor) consoleOpen(uuid, id string) {
	h := s.consoleHubFor(uuid)
	h.mu.Lock()
	if len(h.conns) >= maxConsoleSessions {
		h.mu.Unlock()
		_ = s.sendConsole(uuid, "stat", map[string]any{"id": id, "error": "too many sessions"})
		return
	}
	if _, dup := h.conns[id]; dup {
		h.mu.Unlock()
		return // sesi id sudah ada
	}
	h.mu.Unlock()

	// stdout+stderr dari container → tee: ring buffer + panel WS.
	outW := &consoleWriter{sup: s, uuid: uuid, ring: s.ringFor(uuid)}
	pr, pw := io.Pipe()
	sess := &consoleSession{id: id, stdin: pw}

	ctx, cancel := context.WithCancel(context.Background())
	sess.cancel = cancel

	h.mu.Lock()
	h.conns[id] = sess
	h.mu.Unlock()

	// goroutine sesi: jalankan exec attach, pompa output sampai ctx done.
	go func() {
		defer func() {
			// tutup bersih: hapus dari registry + tutup stdin pipe.
			// Hapus HANYA bila entri masih sesi ini (bukan sesi baru dengan
			// id sama yang sudah di-register ulang oleh consoleOpen).
			_ = pw.Close()
			h.mu.Lock()
			if cur, ok := h.conns[id]; ok && cur == sess {
				delete(h.conns, id)
			}
			h.mu.Unlock()
		}()
		handle, err := s.rt.ExecAttach(ctx, uuid, pr, outW)
		if err != nil {
			log.Printf("console exec %s/%s: %v", uuid, id, err)
			_ = s.sendConsole(uuid, "stat", map[string]any{"id": id, "error": err.Error()})
			// drain stdin supaya goroutine penulis input tidak menggantung
			go func() {
				buf := make([]byte, 4096)
				for {
					if _, err := pr.Read(buf); err != nil {
						return
					}
				}
			}()
			return
		}
		sess.handle = handle
		_ = s.sendConsole(uuid, "stat", map[string]any{"id": id, "ok": true})
		<-ctx.Done() // WS putus / close → exec ditutup bersih
		_ = handle.Close()
	}()
}

// consoleWriter: io.Writer yang menyalin byte console ke ring + WS.
// Streaming murni: tidak ada buffer penuh; setiap Write langsung diteruskan.
type consoleWriter struct {
	sup  *Supervisor
	uuid string
	ring *RingBuffer
}

func (w *consoleWriter) Write(p []byte) (int, error) {
	w.ring.Write(p)
	// kirim ke panel (frame console:<uuid> type=out); kalau WS putus, tetap
	// masuk ring buffer — replay nanti.
	_ = w.sup.sendConsole(w.uuid, "out", map[string]string{"d": string(p)})
	return len(p), nil
}

// consoleWriteStdin: input console dari panel → stdin exec.
func (s *Supervisor) consoleWriteStdin(uuid, id, data string) {
	s.mu.Lock()
	h, ok := s.consoles[uuid]
	s.mu.Unlock()
	if !ok {
		return
	}
	h.mu.Lock()
	sess, ok := h.conns[id]
	h.mu.Unlock()
	if !ok || sess.stdin == nil {
		return
	}
	_, _ = sess.stdin.Write([]byte(data))
}

// consoleResize: resize TTY sesi.
func (s *Supervisor) consoleResize(uuid, id string, cols, rows uint) {
	s.mu.Lock()
	h, ok := s.consoles[uuid]
	s.mu.Unlock()
	if !ok {
		return
	}
	h.mu.Lock()
	sess := h.conns[id]
	h.mu.Unlock()
	if sess == nil || sess.handle == nil {
		return
	}
	if err := sess.handle.Resize(context.Background(), cols, rows); err != nil {
		log.Printf("console resize %s/%s: %v", uuid, id, err)
	}
}

// consoleClose: tutup satu sesi console (dari frame panel "close").
func (s *Supervisor) consoleClose(uuid, id, reason string) {
	s.mu.Lock()
	h, ok := s.consoles[uuid]
	s.mu.Unlock()
	if !ok {
		return
	}
	h.mu.Lock()
	sess, ok := h.conns[id]
	if ok {
		delete(h.conns, id)
	}
	h.mu.Unlock()
	if ok && sess.cancel != nil {
		log.Printf("console %s/%s closed: %s", uuid, id, reason)
		sess.cancel()
	}
}

// consoleCloseAll: tutup SEMUA sesi console semua server (WS panel putus).
func (s *Supervisor) consoleCloseAll(reason string) {
	s.mu.Lock()
	consoles := make([]*consoleHub, 0, len(s.consoles))
	for _, h := range s.consoles {
		consoles = append(consoles, h)
	}
	s.mu.Unlock()
	for _, h := range consoles {
		h.mu.Lock()
		sessions := make([]*consoleSession, 0, len(h.conns))
		for _, sess := range h.conns {
			sessions = append(sessions, sess)
		}
		h.conns = map[string]*consoleSession{}
		h.mu.Unlock()
		for _, sess := range sessions {
			if sess.stdin != nil {
				_ = sess.stdin.Close()
			}
			if sess.cancel != nil {
				sess.cancel()
			}
		}
	}
}

// sendConsole: kirim frame console:<uuid> ke panel (kalau WS nyambung).
// Kalau panel tidak nyambung, output console TETAP ditulis ke ring buffer.
func (s *Supervisor) sendConsole(uuid, typ string, payload any) error {
	s.mu.Lock()
	send := s.send
	s.mu.Unlock()
	if send == nil {
		return nil // diam-diam: log tetap di ring buffer
	}
	return send(Frame{Ch: "console:" + uuid, Type: typ, Payload: mustJSON(payload)})
}

// consolePrint: tulis satu baris asal daemon (bukan output proses) ke console
// server — masuk ring buffer (ikut replay) + terkirim live ke semua sesi.
// Dipakai baris lifecycle start/restart/stop, supaya console hanya berisi
// output script + event lifecycle. Tidak melewati broadcast runtime sehingga
// tidak memengaruhi deteksi teks startup-done.
func (s *Supervisor) consolePrint(uuid, line string) {
	w := &consoleWriter{sup: s, uuid: uuid, ring: s.ringFor(uuid)}
	_, _ = w.Write([]byte(line + "\n"))
}

// consoleReplay: kirim ulang isi ring buffer uuid ke panel (replay).
func (s *Supervisor) consoleReplay(uuid string) {
	r := s.ringFor(uuid)
	data := r.Bytes()
	if len(data) == 0 {
		return
	}
	if err := s.sendConsole(uuid, "out", map[string]string{"d": string(data)}); err != nil {
		log.Printf("console replay %s: %v", uuid, err)
	}
}
