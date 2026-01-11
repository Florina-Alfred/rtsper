package topic

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aler9/gortsplib"
)

// Config holds configuration for TopicManager
type Config struct {
	PublishPort            int
	SubscribePort          int
	MaxPublishers          int
	MaxSubscribersPerTopic int
	PublisherQueueSize     int
	SubscriberQueueSize    int
	PublisherGracePeriod   time.Duration
}

// Manager manages topics and global counters
type Manager struct {
	mu             sync.RWMutex
	topics         map[string]*Topic
	cfg            Config
	publisherCount int
}

// NewManager creates a new Topic Manager
func NewManager(cfg Config) *Manager {
	return &Manager{topics: make(map[string]*Topic), cfg: cfg}
}

// Shutdown cleans up resources
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.topics {
		t.Close()
	}
}

// StatusJSON is a condensed status structure for admin
type StatusJSON struct {
	Topics         []TopicStatus `json:"topics"`
	PublisherCount int           `json:"publisher_count"`
}

// TopicStatus describes topic in status
type TopicStatus struct {
	Name            string `json:"name"`
	HasPublisher    bool   `json:"has_publisher"`
	PublisherID     string `json:"publisher_id"`
	SubscriberCount int    `json:"subscriber_count"`
}

// RegisterPublisher registers a publisher for a topic. Returns error if not allowed.
func (m *Manager) RegisterPublisher(ctx context.Context, name string, pub *PublisherSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.publisherCount >= m.cfg.MaxPublishers {
		return ErrMaxPublishers
	}
	if t, ok := m.topics[name]; ok {
		if t.HasPublisher() {
			return ErrTopicHasPublisher
		}
		t.SetPublisher(pub)
	} else {
		t := NewTopic(name, m.cfg)
		t.SetPublisher(pub)
		m.topics[name] = t
	}
	m.publisherCount++
	return nil
}

// SetTopicStream associates a gortsplib ServerStream with a topic.
func (m *Manager) SetTopicStream(name string, st *gortsplib.ServerStream) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.topics[name]; ok {
		t.SetStream(st)
	}
}

// GetTopicStream returns the ServerStream for a topic or nil if none.
func (m *Manager) GetTopicStream(name string) *gortsplib.ServerStream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if t, ok := m.topics[name]; ok {
		return t.Stream()
	}
	return nil
}

// UnregisterPublisher removes publisher from topic
func (m *Manager) UnregisterPublisher(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.topics[name]; ok {
		t.RemovePublisher()
		if m.publisherCount > 0 {
			m.publisherCount--
		}
	}
}

// RegisterSubscriber registers a subscriber; returns error if topic missing or limit reached
func (m *Manager) RegisterSubscriber(ctx context.Context, name string, sub *SubscriberSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.topics[name]; ok {
		if len(t.subscribers) >= m.cfg.MaxSubscribersPerTopic {
			return ErrTopicMaxSubscribers
		}
		t.AddSubscriber(sub)
		return nil
	}
	return ErrNoActivePublisher
}

// UnregisterSubscriber removes subscriber from a topic
func (m *Manager) UnregisterSubscriber(name string, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.topics[name]; ok {
		t.RemoveSubscriber(id)
	}
}

// Status returns a StatusJSON for admin
func (m *Manager) Status() StatusJSON {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st := StatusJSON{PublisherCount: m.publisherCount}
	for _, t := range m.topics {
		st.Topics = append(st.Topics, TopicStatus{
			Name:            t.name,
			HasPublisher:    t.HasPublisher(),
			PublisherID:     t.PublisherID(),
			SubscriberCount: len(t.subscribers),
		})
	}
	return st
}

// PublishPacket pushes a packet into the topic inbound channel. Returns false if topic missing or closed.
func (m *Manager) PublishPacket(topicName string, pkt *InboundPacket) bool {
	m.mu.RLock()
	t, ok := m.topics[topicName]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case t.in <- pkt:
		return true
	default:
		// drop-oldest to prioritize live
		select {
		case <-t.in:
		default:
		}
		select {
		case t.in <- pkt:
			return true
		default:
			return false
		}
	}
}

// Topic represents a streaming topic
type Topic struct {
	name      string
	mu        sync.RWMutex
	publisher *PublisherSession
	stream    *gortsplib.ServerStream
	// legacy queues (kept for potential future use)
	subscribers map[string]*SubscriberSession
	in          chan *InboundPacket
	cfg         Config
	closed      bool
	// grace timer
	graceTimer *time.Timer
}

// NewTopic creates a topic
func NewTopic(name string, cfg Config) *Topic {
	t := &Topic{
		name:        name,
		subscribers: make(map[string]*SubscriberSession),
		in:          make(chan *InboundPacket, cfg.PublisherQueueSize),
		cfg:         cfg,
	}
	go t.dispatcher()
	return t
}

// dispatcher reads inbound packets and fans out to subscribers
func (t *Topic) dispatcher() {
	for pkt := range t.in {
		// naive fanout
		t.mu.RLock()
		for _, s := range t.subscribers {
			// non-blocking enqueue
			select {
			case s.queue <- pkt:
			default:
				// drop oldest: try to read one then push
				select {
				case <-s.queue:
				default:
				}
				select {
				case s.queue <- pkt:
				default:
				}
			}
		}
		t.mu.RUnlock()
	}
}

// HasPublisher returns whether topic has publisher
func (t *Topic) HasPublisher() bool {
	return t.publisher != nil
}

// SetPublisher sets the publisher
func (t *Topic) SetPublisher(p *PublisherSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.publisher = p
	// stop grace timer if running
	if t.graceTimer != nil {
		t.graceTimer.Stop()
		t.graceTimer = nil
	}
}

// SetStream sets the gortsplib ServerStream for this topic
func (t *Topic) SetStream(st *gortsplib.ServerStream) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stream = st
}

// Stream returns the ServerStream for this topic
func (t *Topic) Stream() *gortsplib.ServerStream {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stream
}

// RemovePublisher clears publisher and starts grace timer
func (t *Topic) RemovePublisher() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.publisher == nil {
		return
	}
	// cancel publisher context
	if t.publisher.cancel != nil {
		t.publisher.cancel()
	}
	t.publisher = nil
	// close stream if present
	if t.stream != nil {
		t.stream.Close()
		t.stream = nil
	}
	// start grace timer to cleanup
	t.graceTimer = time.AfterFunc(t.cfg.PublisherGracePeriod, func() { t.Close() })
}

// PublisherID returns publisher id if any
func (t *Topic) PublisherID() string {
	if t.publisher == nil {
		return ""
	}
	return t.publisher.id
}

// AddSubscriber registers subscriber
func (t *Topic) AddSubscriber(s *SubscriberSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.subscribers[s.id] = s
}

// RemoveSubscriber removes subscriber
func (t *Topic) RemoveSubscriber(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.subscribers[id]; ok {
		if s.cancel != nil {
			s.cancel()
		}
		delete(t.subscribers, id)
	}
}

// Close cleans up topic
func (t *Topic) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	// cancel publisher
	if t.publisher != nil && t.publisher.cancel != nil {
		t.publisher.cancel()
	}
	// cancel subscribers
	for _, s := range t.subscribers {
		if s.cancel != nil {
			s.cancel()
		}
	}
	// close inbound
	close(t.in)
	// clear maps
	t.subscribers = make(map[string]*SubscriberSession)
	t.mu.Unlock()
}

// Minimal session structs and errors

var (
	ErrMaxPublishers       = errors.New("max publishers reached")
	ErrTopicHasPublisher   = errors.New("topic already has active publisher")
	ErrTopicMaxSubscribers = errors.New("topic max subscribers reached")
	ErrNoActivePublisher   = errors.New("no active publisher for topic")
)

// PublisherSession is a placeholder for publisher connection
type PublisherSession struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
}

// SubscriberSession is a placeholder for subscriber connection
type SubscriberSession struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	queue  chan *InboundPacket
}

// InboundPacket is a wrapper for RTP packets
type InboundPacket struct {
	Track int
	Raw   []byte
}

// NewPublisherSession creates a session
func NewPublisherSession(id string) *PublisherSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &PublisherSession{id: id, ctx: ctx, cancel: cancel}
}

// NewSubscriberSession creates a session with a queue
func NewSubscriberSession(id string, queueSize int) *SubscriberSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &SubscriberSession{id: id, ctx: ctx, cancel: cancel, queue: make(chan *InboundPacket, queueSize)}
}

// Enqueue on subscriber returns false if dropped
func (s *SubscriberSession) Enqueue(pkt *InboundPacket) bool {
	select {
	case s.queue <- pkt:
		return true
	default:
		// drop oldest
		select {
		case <-s.queue:
		default:
		}
		select {
		case s.queue <- pkt:
			return true
		default:
			return false
		}
	}
}

// Dequeue helper used by writer goroutine
func (s *SubscriberSession) Dequeue() (*InboundPacket, bool) {
	select {
	case pkt := <-s.queue:
		return pkt, true
	default:
		return nil, false
	}
}
