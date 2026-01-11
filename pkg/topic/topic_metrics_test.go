package topic

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestTopicCloseDecrementsActiveSubscribers(t *testing.T) {
	cfg := Config{MaxPublishers: 5, MaxSubscribersPerTopic: 5, PublisherQueueSize: 16, SubscriberQueueSize: 8, PublisherGracePeriod: Duration{Duration: 0}}
	m := NewManager(cfg)
	pub := NewPublisherSession("p1")
	if err := m.RegisterPublisher(nil, "t1", pub); err != nil {
		t.Fatalf("register publisher: %v", err)
	}
	// register a subscriber
	sub := NewSubscriberSession("s1", 8)
	if err := m.RegisterSubscriber(nil, "t1", sub); err != nil {
		t.Fatalf("register subscriber: %v", err)
	}
	// check gauge is 1
	vec, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}
	found := false
	for _, f := range vec {
		if f.GetName() == "rtsper_active_subscribers" {
			found = true
			if len(f.Metric) == 0 || f.Metric[0].Gauge.GetValue() != 1 {
				t.Fatalf("expected active_subscribers 1, got %v", f.Metric)
			}
		}
	}
	if !found {
		t.Fatal("rtsper_active_subscribers metric not found")
	}
	// unregister publisher to trigger topic close
	m.UnregisterPublisher("t1")
	// wait for any async cleanup
	time.Sleep(50 * time.Millisecond)
	// gather again
	vec2, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}
	for _, f := range vec2 {
		if f.GetName() == "rtsper_active_subscribers" {
			if len(f.Metric) == 0 {
				t.Fatalf("no metric samples after close")
			}
			if f.Metric[0].Gauge.GetValue() != 0 {
				t.Fatalf("expected active_subscribers 0 after close, got %v", f.Metric[0].Gauge.GetValue())
			}
		}
	}
}
