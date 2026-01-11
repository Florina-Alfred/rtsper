package topic

import (
	"context"
	"testing"
	"time"

	"github.com/aler9/gortsplib"
)

func TestRegisterPublisherAndUnregister(t *testing.T) {
	cfg := Config{
		MaxPublishers:          5,
		MaxSubscribersPerTopic: 5,
		PublisherQueueSize:     4,
		SubscriberQueueSize:    4,
		PublisherGracePeriod:   500 * time.Millisecond,
	}
	m := NewManager(cfg)

	pub := NewPublisherSession("pub1")
	if err := m.RegisterPublisher(context.Background(), "topic1", pub); err != nil {
		t.Fatalf("RegisterPublisher failed: %v", err)
	}
	st := m.Status()
	if st.PublisherCount != 1 {
		t.Fatalf("expected PublisherCount 1, got %d", st.PublisherCount)
	}
	found := false
	for _, ts := range st.Topics {
		if ts.Name == "topic1" {
			found = true
			if !ts.HasPublisher {
				t.Fatalf("expected topic1 to have publisher")
			}
		}
	}
	if !found {
		t.Fatalf("topic1 not present in status")
	}

	// Unregister and verify counts
	m.UnregisterPublisher("topic1")
	st2 := m.Status()
	if st2.PublisherCount != 0 {
		t.Fatalf("expected PublisherCount 0 after unregister, got %d", st2.PublisherCount)
	}
	// topic entry may still exist until grace timer; ensure HasPublisher is false
	for _, ts := range st2.Topics {
		if ts.Name == "topic1" {
			if ts.HasPublisher {
				t.Fatalf("expected topic1 to not have publisher after unregister")
			}
		}
	}
}

func TestMaxPublishers(t *testing.T) {
	cfg := Config{MaxPublishers: 1, PublisherGracePeriod: time.Second}
	m := NewManager(cfg)

	if err := m.RegisterPublisher(context.Background(), "a", NewPublisherSession("p1")); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if err := m.RegisterPublisher(context.Background(), "b", NewPublisherSession("p2")); err == nil {
		t.Fatalf("expected second register to fail due to max publishers")
	}
}

func TestRegisterSubscriberLimits(t *testing.T) {
	cfg := Config{MaxPublishers: 5, MaxSubscribersPerTopic: 1, PublisherGracePeriod: time.Second}
	m := NewManager(cfg)
	// create topic by registering publisher
	if err := m.RegisterPublisher(context.Background(), "topicX", NewPublisherSession("p1")); err != nil {
		t.Fatalf("register publisher failed: %v", err)
	}
	if err := m.RegisterSubscriber(context.Background(), "topicX", NewSubscriberSession("s1", 4)); err != nil {
		t.Fatalf("first subscriber register failed: %v", err)
	}
	if err := m.RegisterSubscriber(context.Background(), "topicX", NewSubscriberSession("s2", 4)); err == nil {
		t.Fatalf("expected second subscriber to be rejected due to limit")
	}
}

func TestSetGetTopicStream(t *testing.T) {
	cfg := Config{MaxPublishers: 5, PublisherGracePeriod: time.Second}
	m := NewManager(cfg)
	if err := m.RegisterPublisher(context.Background(), "tstream", NewPublisherSession("p1")); err != nil {
		t.Fatalf("register publisher failed: %v", err)
	}
	st := gortsplib.NewServerStream(gortsplib.Tracks{})
	m.SetTopicStream("tstream", st)
	got := m.GetTopicStream("tstream")
	if got != st {
		t.Fatalf("expected same stream pointer after set/get")
	}
}
