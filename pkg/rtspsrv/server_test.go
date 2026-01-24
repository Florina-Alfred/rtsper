package rtspsrv

import (
	"context"
	"testing"
	"time"

	"redalf.de/rtsper/pkg/topic"
	"redalf.de/rtsper/pkg/udpalloc"
)

// TestServerStartWithAllocator verifies the server starts when an allocator has pre-bound UDP pairs.
func TestServerStartWithAllocator(t *testing.T) {
	alloc, err := udpalloc.NewAllocator(42000, 42010)
	if err != nil {
		t.Fatalf("failed to create allocator: %v", err)
	}

	p1, rel1, err := alloc.ReservePair()
	if err != nil {
		t.Fatalf("failed to reserve first pair: %v", err)
	}
	p2, rel2, err := alloc.ReservePair()
	if err != nil {
		rel1()
		t.Fatalf("failed to reserve second pair: %v", err)
	}
	// ensure cleanup
	defer func() {
		rel1()
		rel2()
	}()

	cfg := topic.Config{
		EnableUDP:         true,
		PublisherUDPBase:  p1,
		SubscriberUDPBase: p2,
		PublishPort:       0, // let OS pick TCP ports to avoid collisions in tests
		SubscribePort:     0,
	}
	m := topic.NewManager(cfg)

	// use defaults for cluster/proxy in tests - no cluster
	s := NewServer(m, cfg.PublishPort, cfg.SubscribePort, alloc, nil, false, 0, 0)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("server failed to start: %v", err)
	}
	// give goroutines a moment to start
	time.Sleep(100 * time.Millisecond)

	// basic assertion: allocator still has the conns
	if pc, ok := alloc.GetConn(p1); !ok || pc == nil {
		t.Fatalf("expected allocator to have RTP conn for %d", p1)
	}
	if pc, ok := alloc.GetConn(p2); !ok || pc == nil {
		t.Fatalf("expected allocator to have RTP conn for %d", p2)
	}

	// Close server (should not panic)
	s.Close()
	// give a moment for shutdown
	time.Sleep(50 * time.Millisecond)
}
