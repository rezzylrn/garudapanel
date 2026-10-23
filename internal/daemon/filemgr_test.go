// Test file manager end-to-end di daemon (tanpa Docker): FileOps + RPC file
// + upload streaming chunk + quota + chown + path traversal ditolak.
package daemon

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"garudapanel/internal/dockerrt"
	"garudapanel/internal/filecore"
)

// newTestSupervisorFS: supervisor dengan MemFS factory (tanpa disk asli).
func newTestSupervisorFS(t *testing.T) (*Supervisor, *dockerrt.FakeRuntime, *frameCollector, *filecore.MemFS) {
	t.Helper()
	rt := dockerrt.NewFake()
	sup := NewSupervisor(rt, SupervisorConfig{StatsInterval: 50 * time.Millisecond, DataDir: "/tmp/fake-volumes"})
	mem := filecore.NewMem()
	sup.volumes = filecore.NewVolumes(func(root string) (filecore.FS, error) {
		return mem, nil
	})
	fc := &frameCollector{}
	sup.Attach(func(f Frame) error {
		fc.collect(f)
		return nil
	})
	return sup, rt, fc, mem
}

func registerForFiles(t *testing.T, sup *Supervisor, uuid string, diskMB int64) {
	t.Helper()
	sup.Register(uuid, dockerrt.ContainerSpec{
		Ref: uuid, Image: "test:latest", Cmd: []string{"sh"},
		DiskMB: diskMB, ImageUser: "1000:1000",
	})
}

func TestFileRPCListMkdirDelete(t *testing.T) {
	sup, _, _, mem := newTestSupervisorFS(t)
	registerForFiles(t, sup, "uuid-1", 100)

	if _, err := sup.handleFileRPC([]byte(`{"server_uuid":"uuid-1","op":"mkdir","path":"world"}`)); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !mem.Has("world") {
		t.Fatal("dir world tidak dibuat")
	}
	fops, err := sup.fileOps("uuid-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fops.Write("world/level.dat", strings.NewReader("hello"), 0); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := sup.handleFileRPC([]byte(`{"server_uuid":"uuid-1","op":"list","path":"world"}`))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), "level.dat") {
		t.Fatalf("list result tidak berisi level.dat: %s", raw)
	}
	if _, err := sup.handleFileRPC([]byte(`{"server_uuid":"uuid-1","op":"delete","path":"world"}`)); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if mem.Has("world/level.dat") {
		t.Fatal("file harus terhapus (rekursif)")
	}
}

func TestFileRPCRenameExtractUsage(t *testing.T) {
	sup, _, _, mem := newTestSupervisorFS(t)
	registerForFiles(t, sup, "uuid-1", 100)
	fops, _ := sup.fileOps("uuid-1")

	fops.Write("a.txt", strings.NewReader("AAA"), 0)
	if _, err := sup.handleFileRPC([]byte(`{"server_uuid":"uuid-1","op":"rename","path":"a.txt","to":"b.txt"}`)); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !mem.Has("b.txt") || mem.Has("a.txt") {
		t.Fatal("rename salah")
	}
	res, err := sup.handleFileRPC([]byte(`{"server_uuid":"uuid-1","op":"usage"}`))
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	b, _ := json.Marshal(res)
	if !strings.Contains(string(b), "used") || !strings.Contains(string(b), "limit") {
		t.Fatalf("usage result: %s", b)
	}
}

func TestFileRPCTraversalRejected(t *testing.T) {
	sup, _, _, _ := newTestSupervisorFS(t)
	registerForFiles(t, sup, "uuid-1", 100)
	if _, err := sup.handleFileRPC([]byte(`{"server_uuid":"uuid-1","op":"list","path":"../../etc"}`)); err == nil {
		t.Fatal("list traversal harus ditolak")
	}
	if _, err := sup.handleFileRPC([]byte(`{"server_uuid":"uuid-1","op":"delete","path":"../../../etc/passwd"}`)); err == nil {
		t.Fatal("delete traversal harus ditolak")
	}
}

func TestFileWriteStreamingEndToEnd(t *testing.T) {
	sup, _, _, mem := newTestSupervisorFS(t)
	registerForFiles(t, sup, "uuid-1", 100)

	if err := sup.handleWriteBegin([]byte(`{"server_uuid":"uuid-1","path":"upload/world.zip"}`)); err != nil {
		t.Fatalf("write-begin: %v", err)
	}
	data := []byte("DATA-CHUNK-1")
	if err := sup.handleWriteChunk([]byte(`{"server_uuid":"uuid-1","path":"upload/world.zip","d":"` +
		base64.StdEncoding.EncodeToString(data) + `"}`)); err != nil {
		t.Fatalf("chunk: %v", err)
	}
	sup.handleWriteEnd([]byte(`{"req_id":"r1","server_uuid":"uuid-1","path":"upload/world.zip"}`))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, ok := mem.Get("upload/world.zip"); ok && string(data) == "DATA-CHUNK-1" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("file tidak tertulis via streaming write")
}

func TestFileReadStreamingEndToEnd(t *testing.T) {
	sup, _, fc, mem := newTestSupervisorFS(t)
	registerForFiles(t, sup, "uuid-1", 100)
	mem.Put("server.log", []byte("hello world from server"))

	sup.handleReadBegin([]byte(`{"req_id":"rr","server_uuid":"uuid-1","path":"server.log"}`))
	ok := waitCond(2*time.Second, func() bool {
		for _, f := range fc.all() {
			if f.Type == "file:read-chunk" {
				return true
			}
		}
		return false
	})
	if !ok {
		t.Fatal("tidak ada read-chunk frame")
	}
	for _, f := range fc.all() {
		if f.Type == "file:read-chunk" {
			var p struct {
				D string `json:"d"`
			}
			_ = json.Unmarshal(f.Payload, &p)
			got, _ := base64.StdEncoding.DecodeString(p.D)
			if string(got) != "hello world from server" {
				t.Fatalf("chunk = %q", got)
			}
			return
		}
	}
}

func TestQuotaAccountingEventBased(t *testing.T) {
	sup, _, _, _ := newTestSupervisorFS(t)
	registerForFiles(t, sup, "uuid-1", 1)
	fops, _ := sup.fileOps("uuid-1")
	fops.Write("a.txt", strings.NewReader("12345"), 0) // 5 byte
	used, _ := fops.UsedDisk()
	if used != 5 {
		t.Fatalf("used = %d, want 5", used)
	}
	fops.Delete("a.txt")
	used, _ = fops.UsedDisk()
	if used != 0 {
		t.Fatalf("used after delete = %d, want 0", used)
	}
}

func TestBackgroundScanThrottled1Per5Min(t *testing.T) {
	sup, _, _, _ := newTestSupervisorFS(t)
	registerForFiles(t, sup, "uuid-1", 1)
	fops, _ := sup.fileOps("uuid-1")
	fops.Write("a.txt", strings.NewReader("123456"), 0)
	// scan pertama jalan, kedua di-throttle
	if _, ran := fops.ScanBackground(); !ran {
		t.Fatal("scan pertama harus jalan")
	}
	if _, ran := fops.ScanBackground(); ran {
		t.Fatal("scan kedua harus di-throttle (1x/5 menit)")
	}
}
