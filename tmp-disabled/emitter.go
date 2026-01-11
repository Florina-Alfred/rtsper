//go:build ignore

package main

import (
	"context"
	"log"
	"time"

	"redalf.de/rtsper/pkg/metrics"
)

func main() {
	if err := metrics.InitOTLP(context.Background(), "localhost:4317"); err != nil {
		log.Fatalf("init otlp: %v", err)
	}
	log.Println("initialized OTLP")
	for i := 0; i < 10; i++ {
		metrics.IncPacketsReceived()
		metrics.IncAllocatorReservations()
		log.Printf("emitted iteration %d", i)
		time.Sleep(500 * time.Millisecond)
	}
	log.Println("done")
}
