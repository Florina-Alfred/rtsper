//go:build ignore

package main

import (
	"log"
	"time"

	"github.com/aler9/gortsplib"
)

func main() {
	c := &gortsplib.Client{}
	if err := c.Start("tcp", "localhost:8555"); err != nil {
		log.Fatalf("start: %v", err)
	}
	defer c.Close()
	// play topic3
	if _, err := c.Play("rtsp://localhost:8555/topic3"); err != nil {
		log.Fatalf("play failed: %v", err)
	}
	log.Println("playing for 2s")
	time.Sleep(2 * time.Second)
	log.Println("done")
}
