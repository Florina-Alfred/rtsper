package udpalloc

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"redalf.de/rtsper/pkg/metrics"
)

// Allocator reserves even RTP/RTCP port pairs from a configurable range.
// It binds UDP sockets at allocation time to avoid races.
type Allocator struct {
	start int
	end   int
	mu    sync.Mutex
	// map port -> reservation (net.PacketConn)
	reserved map[int]net.PacketConn
}

// NewAllocator creates an allocator for the inclusive port range [start,end].
func NewAllocator(start, end int) (*Allocator, error) {
	if start <= 0 || end <= 0 || start > end {
		return nil, fmt.Errorf("invalid port range")
	}
	if (start % 2) != 0 {
		start++ // make it even
	}
	return &Allocator{start: start, end: end, reserved: make(map[int]net.PacketConn)}, nil
}

// ReservePair finds an available even base port p in the range, binds RTP (p) and RTCP (p+1)
// and returns the base port and a release function. The caller must call release when done.
func (a *Allocator) ReservePair() (int, func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for p := a.start; p <= a.end; p += 2 {
		if _, ok := a.reserved[p]; ok {
			continue
		}
		// try bind RTP and RTCP
		rtpAddr := fmt.Sprintf(":%d", p)
		rtcpAddr := fmt.Sprintf(":%d", p+1)
		rtp, err := net.ListenPacket("udp", rtpAddr)
		if err != nil {
			continue
		}
		rtcp, err := net.ListenPacket("udp", rtcpAddr)
		if err != nil {
			rtp.Close()
			continue
		}
		// success: store both conns keyed by port
		a.reserved[p] = rtp
		a.reserved[p+1] = rtcp
		num := 2
		metrics.IncAllocatorReservations()
		metrics.IncAllocatorReservedPairs()
		_ = num
		release := func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			if c, ok := a.reserved[p]; ok {
				c.Close()
				delete(a.reserved, p)
			}
			if c, ok := a.reserved[p+1]; ok {
				c.Close()
				delete(a.reserved, p+1)
			}
		}
		return p, release, nil
	}
	return 0, nil, errors.New("no available ports")
}

// GetConn returns the previously reserved PacketConn for a given port if present.
// The returned net.PacketConn must NOT be closed by the caller (the allocator manages lifecycle).
func (a *Allocator) GetConn(port int) (net.PacketConn, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c, ok := a.reserved[port]
	return c, ok
}
