// Test ring buffer 100KB (SPEC §4/§9): memutar dengan benar.
package daemon

import (
	"bytes"
	"testing"
)

func TestRingBufferBasic(t *testing.T) {
	r := NewRingBuffer(100)
	r.Write([]byte("hello world"))
	if got := string(r.Bytes()); got != "hello world" {
		t.Fatalf("got %q want %q", got, "hello world")
	}
}

func TestRingBufferOrderPreserved(t *testing.T) {
	r := NewRingBuffer(16)
	r.Write([]byte("abcdefgh"))
	r.Write([]byte("ijklmnop"))
	if got := string(r.Bytes()); got != "abcdefghijklmnop" {
		t.Fatalf("got %q", got)
	}
	r.Write([]byte("qrst")) // rotasi 4
	if got := string(r.Bytes()); got != "efghijklmnopqrst" {
		t.Fatalf("got %q want efghijklmnopqrst", got)
	}
}

func TestRingBufferRotation(t *testing.T) {
	size := 100
	r := NewRingBuffer(size)
	r.Write(bytes.Repeat([]byte("a"), 60))
	r.Write(bytes.Repeat([]byte("b"), 60)) // total 120 > 100
	b := r.Bytes()
	if len(b) != size {
		t.Fatalf("len = %d, want %d", len(b), size)
	}
	// 20 'a' pertama terguling; sisanya 40 'a' + 60 'b'
	want := string(bytes.Repeat([]byte("a"), 40)) + string(bytes.Repeat([]byte("b"), 60))
	if string(b) != want {
		t.Fatalf("rotation content wrong: got %.20q... want %.20q...", b, want)
	}
}

func TestRingBufferOversizeWrite(t *testing.T) {
	r := NewRingBuffer(10)
	r.Write(bytes.Repeat([]byte("y"), 50)) // lebih besar dari kapasitas
	got := r.Bytes()
	if len(got) != 10 {
		t.Fatalf("len = %d want 10", len(got))
	}
	if string(got) != "yyyyyyyyyy" {
		t.Fatalf("got %q", string(got))
	}
}

func TestRingBuffer100KB(t *testing.T) {
	// ring 100KB diputar dengan payload ~150KB
	r := NewRingBuffer(ringBufferLimit)
	payload := bytes.Repeat([]byte("z"), 150*1024)
	r.Write(payload)
	if r.Len() != ringBufferLimit {
		t.Fatalf("len = %d want %d", r.Len(), ringBufferLimit)
	}
	got := r.Bytes()
	if !bytes.Equal(got, payload[len(payload)-ringBufferLimit:]) {
		t.Fatal("100KB ring did not keep the tail of 150KB write")
	}
}
