//go:build ignore

package main

import (
	"fmt"
	"time"

	"redalf.de/rtsper/pkg/topic"
)

func main() {
	cfg := topic.Config{PublisherQueueSize: 1024, SubscriberQueueSize: 256, MaxSubscribersPerTopic: 5, MaxPublishers: 1}
	m := topic.NewManager(cfg)
	// create a topic and register a subscriber
	sub := topic.NewSubscriberSession("sub1", 10)
	// RegisterPublisher to create topic
	pub := topic.NewPublisherSession("pub1")
	m.RegisterPublisher(nil, "testtopic", pub)
	fmt.Println("after register publisher:", m.Status())
	m.RegisterSubscriber(nil, "testtopic", sub)
	fmt.Println("after register subscriber:", m.Status())
	m.UnregisterSubscriber("testtopic", "sub1")
	fmt.Println("after unregister subscriber:", m.Status())
	// cleanup
	m.UnregisterPublisher("testtopic")
	fmt.Println("after unregister publisher:", m.Status())
	// wait a bit for any goroutines
	time.Sleep(200 * time.Millisecond)
}
