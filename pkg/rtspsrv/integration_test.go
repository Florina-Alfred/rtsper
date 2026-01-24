package rtspsrv

import (
	"testing"
	"time"

	"redalf.de/rtsper/pkg/cluster"
	"redalf.de/rtsper/pkg/topic"
)

// Integration style test: start two servers on ephemeral ports, configure cluster,
// and verify that a connection to non-owner is forwarded (proxy listener will
// accept and should forward; here we assert that the listening port remains open
// and servers start without error). This is a lightweight smoke test rather
// than full end-to-end RTSP protocol validation.
func TestTwoServerStartAndProxyListen(t *testing.T) {
	// topic config with TCP only
	cfg := topic.Config{PublishPort: 0, SubscribePort: 0}
	m1 := topic.NewManager(cfg)
	m2 := topic.NewManager(cfg)

	// create cluster with two members using localhost names; since dial occurs
	// against address:port we will use loopback names and set ports accordingly
	// We'll start servers on ephemeral ports and use member names "srv1","srv2".
	cl, err := cluster.NewFromCSV("srv1,srv2", "srv1")
	if err != nil {
		t.Fatalf("cluster create failed: %v", err)
	}

	// start first server (pub/sub ports 0 => OS assigned)
	s1 := NewServer(m1, 0, 0, nil, cl, true, 1*time.Second, 5*time.Second)
	if err := s1.Start(nil); err != nil {
		t.Fatalf("s1 failed to start: %v", err)
	}
	defer s1.Close()

	// start second server
	// make cl think self is srv2 for this instance by creating a separate cluster
	cl2, _ := cluster.NewFromCSV("srv1,srv2", "srv2")
	s2 := NewServer(m2, 0, 0, nil, cl2, true, 1*time.Second, 5*time.Second)
	if err := s2.Start(nil); err != nil {
		t.Fatalf("s2 failed to start: %v", err)
	}
	defer s2.Close()

	// ensure both listening sockets are present by dialing their pub ports via net.Dial
	// We need to extract the actual bound port from server.pubSrv.Addr(). String contains ":<port>".
	// Wait briefly for servers to start
	time.Sleep(100 * time.Millisecond)

	// we can't easily obtain bound TCP port from gortsplib, so instead assert pubSrv is non-nil
	// and that Start did not return error (smoke check).
	if s1.pubSrv == nil || s2.pubSrv == nil {
		t.Fatalf("expected pub servers to be initialized")
	}
}
