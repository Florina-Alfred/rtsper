package main

import (
	"fmt"
	"net"
	"redalf.de/rtsper/pkg/topic"
)

func validateUDPConfig(cfg topic.Config) error {
	// ensure base ports are set and even
	if cfg.PublisherUDPBase == 0 {
		return fmt.Errorf("publisher UDP base port not set")
	}
	if (cfg.PublisherUDPBase % 2) != 0 {
		return fmt.Errorf("publisher UDP base port must be even")
	}
	if cfg.SubscriberUDPBase == 0 {
		return fmt.Errorf("subscriber UDP base port not set")
	}
	if (cfg.SubscriberUDPBase % 2) != 0 {
		return fmt.Errorf("subscriber UDP base port must be even")
	}
	// check availability by binding to RTP and RTCP ports
	pairs := []int{cfg.PublisherUDPBase, cfg.PublisherUDPBase + 1, cfg.SubscriberUDPBase, cfg.SubscriberUDPBase + 1}
	conns := make([]net.PacketConn, 0, len(pairs))
	for _, p := range pairs {
		addr := fmt.Sprintf(":%d", p)
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			// close previously opened
			for _, c := range conns {
				c.Close()
			}
			return fmt.Errorf("failed to bind UDP port %d: %w", p, err)
		}
		conns = append(conns, pc)
	}
	for _, c := range conns {
		c.Close()
	}
	return nil
}
