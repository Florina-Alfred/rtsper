package udpalloc

import (
	"testing"
)

// TestReservePairGetConnRelease verifies that ReservePair binds RTP/RTCP sockets,
// GetConn returns them, and release unbinds them.
func TestReservePairGetConnRelease(t *testing.T) {
	// pick a reasonably high port range to reduce collision likelihood
	alloc, err := NewAllocator(40000, 40010)
	if err != nil {
		t.Fatalf("failed to create allocator: %v", err)
	}

	p, release, err := alloc.ReservePair()
	if err != nil {
		t.Fatalf("ReservePair failed: %v", err)
	}
	// ensure we release at the end
	defer release()

	if (p % 2) != 0 {
		t.Fatalf("expected even base port, got %d", p)
	}

	// rtp conn
	pc, ok := alloc.GetConn(p)
	if !ok || pc == nil {
		t.Fatalf("expected RTP PacketConn for port %d", p)
	}
	// rtcp conn
	pc2, ok := alloc.GetConn(p + 1)
	if !ok || pc2 == nil {
		t.Fatalf("expected RTCP PacketConn for port %d", p+1)
	}

	// release and ensure they are removed
	release()
	if _, ok := alloc.GetConn(p); ok {
		t.Fatalf("expected RTP port %d to be released", p)
	}
	if _, ok := alloc.GetConn(p + 1); ok {
		t.Fatalf("expected RTCP port %d to be released", p+1)
	}
}

// TestReserveExhaustion ensures allocator returns an error when no ports remain
func TestReserveExhaustion(t *testing.T) {
	alloc, err := NewAllocator(41000, 41000) // only one pair available
	if err != nil {
		t.Fatalf("failed to create allocator: %v", err)
	}

	p, release, err := alloc.ReservePair()
	if err != nil {
		t.Fatalf("first ReservePair failed: %v", err)
	}
	defer release()

	// second reservation should fail
	if _, _, err := alloc.ReservePair(); err == nil {
		t.Fatalf("expected ReservePair to fail when ports exhausted")
	}

	// cleanup done by deferred release
	_ = p
}
