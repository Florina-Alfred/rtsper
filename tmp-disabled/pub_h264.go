//go:build ignore

package main

import (
	"log"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/pion/rtp"
)

func main() {
	// create an H264 track
	track := &gortsplib.TrackH264{
		PayloadType:       96,
		PacketizationMode: 1,
	}
	tracks := gortsplib.Tracks{track}

	c := gortsplib.Client{Transport: func() *gortsplib.Transport { v := gortsplib.TransportTCP; return &v }()}
	// StartPublishing will connect, ANNOUNCE, SETUP and RECORD (force TCP interleaved transport).
	if err := c.StartPublishing("rtsp://localhost:8554/topic3", tracks); err != nil {
		log.Fatalf("StartPublishing failed: %v", err)
	}
	defer c.Close()

	log.Println("publishing 50 RTP packets to rtsp://localhost:8554/topic3")
	for i := 0; i < 50; i++ {
		pkt := &rtp.Packet{
			Header:  rtp.Header{Version: 2, PayloadType: 96, SequenceNumber: uint16(i), Timestamp: uint32(i * 3000), SSRC: 12345},
			Payload: []byte{0x00, 0x00, 0x01, 0x09},
		}
		if err := c.WritePacketRTP(0, pkt); err != nil {
			log.Printf("write rtp error: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	log.Println("done")
}
