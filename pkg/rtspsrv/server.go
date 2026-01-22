package rtspsrv

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"

	plog "redalf.de/rtsper/pkg/log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"

	"redalf.de/rtsper/pkg/metrics"
	"redalf.de/rtsper/pkg/topic"
	"redalf.de/rtsper/pkg/udpalloc"
)

// Server holds RTSP servers for publisher and subscriber
type Server struct {
	mgr       *topic.Manager
	pubPort   int
	subPort   int
	allocator *udpalloc.Allocator
	pubSrv    *gortsplib.Server
	subSrv    *gortsplib.Server
	mu        sync.Mutex
	h         *serverHandler
}

func NewServer(mgr *topic.Manager, pubPort, subPort int, alloc *udpalloc.Allocator) *Server {
	return &Server{mgr: mgr, pubPort: pubPort, subPort: subPort, allocator: alloc}
}

func (s *Server) Start(ctx context.Context) error {
	h := &serverHandler{
		mgr:           s.mgr,
		sessTopic:     make(map[*gortsplib.ServerSession]string),
		sessIsPub:     make(map[*gortsplib.ServerSession]bool),
		topicNameRe:   regexp.MustCompile(`^[A-Za-z0-9_-]+$`),
		subscriberQSz: 256,
	}
	s.h = h

	// configure UDP addresses if enabled
	mgrCfg := s.mgr.Config()
	pubSrv := &gortsplib.Server{Handler: h, RTSPAddress: fmt.Sprintf(":%d", s.pubPort)}
	if mgrCfg.EnableUDP && mgrCfg.PublisherUDPBase > 0 {
		pubSrv.UDPRTPAddress = fmt.Sprintf(":%d", mgrCfg.PublisherUDPBase)
		pubSrv.UDPRTCPAddress = fmt.Sprintf(":%d", mgrCfg.PublisherUDPBase+1)
		// if allocator provided, use its pre-bound PacketConns
		if s.allocator != nil {
			pubSrv.ListenPacket = func(network, address string) (net.PacketConn, error) {
				// extract port from address (e.g., ":5000" or "0.0.0.0:5000")
				host, portStr, err := net.SplitHostPort(address)
				if err != nil {
					// fall back to naive parse
					if len(address) > 0 && address[0] == ':' {
						portStr = address[1:]
					} else {
						return nil, err
					}
				}
				_ = host
				port, err := strconv.Atoi(portStr)
				if err == nil {
					if pc, ok := s.allocator.GetConn(port); ok {
						return pc, nil
					}
				}
				// fallback to normal listen
				return net.ListenPacket(network, address)
			}
		}
	}

	subSrv := &gortsplib.Server{Handler: h, RTSPAddress: fmt.Sprintf(":%d", s.subPort)}
	if mgrCfg.EnableUDP && mgrCfg.SubscriberUDPBase > 0 {
		subSrv.UDPRTPAddress = fmt.Sprintf(":%d", mgrCfg.SubscriberUDPBase)
		subSrv.UDPRTCPAddress = fmt.Sprintf(":%d", mgrCfg.SubscriberUDPBase+1)
		if s.allocator != nil {
			subSrv.ListenPacket = func(network, address string) (net.PacketConn, error) {
				host, portStr, err := net.SplitHostPort(address)
				if err != nil {
					if len(address) > 0 && address[0] == ':' {
						portStr = address[1:]
					} else {
						return nil, err
					}
				}
				_ = host
				port, err := strconv.Atoi(portStr)
				if err == nil {
					if pc, ok := s.allocator.GetConn(port); ok {
						return pc, nil
					}
				}
				return net.ListenPacket(network, address)
			}
		}
	}

	s.mu.Lock()
	s.pubSrv = pubSrv
	s.subSrv = subSrv
	s.mu.Unlock()

	go func() {
		plog.Info("starting RTSP server (publishers) on :%d", s.pubPort)
		if err := pubSrv.Start(); err != nil {
			plog.Info("pub server error: %v", err)
		}
	}()
	go func() {
		plog.Info("starting RTSP server (subscribers) on :%d", s.subPort)
		if err := subSrv.Start(); err != nil {
			plog.Info("sub server error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Close() {
	s.mu.Lock()
	if s.pubSrv != nil {
		s.pubSrv.Close()
	}
	if s.subSrv != nil {
		s.subSrv.Close()
	}
	s.mu.Unlock()
}

// serverHandler implements gortsplib.ServerHandler and routes publishers and subscribers to topics
type serverHandler struct {
	mgr           *topic.Manager
	mu            sync.Mutex
	sessTopic     map[*gortsplib.ServerSession]string
	sessIsPub     map[*gortsplib.ServerSession]bool
	topicNameRe   *regexp.Regexp
	subscriberQSz int
}

func (h *serverHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	plog.Debug("conn open %v", ctx.Conn.NetConn().RemoteAddr())
}

func (h *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	plog.Debug("conn close %v", ctx.Conn.NetConn().RemoteAddr())
}

func (h *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	topicName := strings.TrimPrefix(ctx.Path, "/")
	plog.Debug("describe %s", topicName)
	st := h.mgr.GetTopicStream(topicName)
	if st != nil {
		return &base.Response{StatusCode: base.StatusOK}, st, nil
	}
	return &base.Response{StatusCode: base.StatusNotFound}, nil, nil
}

func (h *serverHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	topicName := strings.TrimPrefix(ctx.Path, "/")
	plog.Debug("announce %s", topicName)
	if !h.topicNameRe.MatchString(topicName) {
		plog.Debug("invalid topic name: %s", topicName)
		return &base.Response{StatusCode: base.StatusBadRequest}, nil
	}
	// create publisher session id
	pubID := fmt.Sprintf("%p", ctx.Session)
	pub := topic.NewPublisherSession(pubID)
	if err := h.mgr.RegisterPublisher(context.Background(), topicName, pub); err != nil {
		plog.Info("register publisher failed: %v", err)
		return &base.Response{StatusCode: base.StatusBadRequest}, nil
	}
	// create ServerStream from tracks and set in topic
	st := gortsplib.NewServerStream(ctx.Tracks)
	h.mgr.SetTopicStream(topicName, st)
	// store session -> topic mapping and mark as publisher
	h.mu.Lock()
	h.sessTopic[ctx.Session] = topicName
	h.sessIsPub[ctx.Session] = true
	h.mu.Unlock()
	return &base.Response{StatusCode: base.StatusOK}, nil
}

func (h *serverHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	plog.Debug("record %s", ctx.Path)
	// increment packet metrics on RECORD to validate metrics plumbing
	metrics.IncPacketsReceived()
	metrics.IncPacketsDispatched()
	return &base.Response{StatusCode: base.StatusOK}, nil
}

func (h *serverHandler) OnPacketRTP(ctx *gortsplib.ServerHandlerOnPacketRTPCtx) {
	// find topic by session
	h.mu.Lock()
	topicName := h.sessTopic[ctx.Session]
	h.mu.Unlock()
	if topicName == "" {
		return
	}
	st := h.mgr.GetTopicStream(topicName)
	if st == nil {
		return
	}
	plog.Debug("OnPacketRTP for topic %s track %d", topicName, ctx.TrackID)
	// increment received metric
	metrics.IncPacketsReceived()
	// write directly to ServerStream (this will reach readers)
	st.WritePacketRTP(ctx.TrackID, ctx.Packet)
	// increment dispatched metric
	metrics.IncPacketsDispatched()

	// marshal packet and publish into topic manager so topic dispatcher handles fanout and metrics
	b, err := ctx.Packet.Marshal()
	if err != nil {
		plog.Info("failed to marshal RTP packet: %v", err)
		return
	}
	pkt := &topic.InboundPacket{Track: ctx.TrackID, Raw: b}
	if !h.mgr.PublishPacket(topicName, pkt) {
		// drop counted inside PublishPacket as needed
		plog.Debug("drop packet for topic %s", topicName)
	}
}

func (h *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	topicName := strings.TrimPrefix(ctx.Path, "/")
	plog.Debug("setup %s", topicName)
	st := h.mgr.GetTopicStream(topicName)
	if st != nil {
		return &base.Response{StatusCode: base.StatusOK}, st, nil
	}
	return &base.Response{StatusCode: base.StatusNotFound}, nil, nil
}

func (h *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	topicName := strings.TrimPrefix(ctx.Path, "/")
	plog.Debug("play %s", topicName)
	// create subscriber session with a reasonable queue size
	subID := fmt.Sprintf("%p", ctx.Session)
	sub := topic.NewSubscriberSession(subID, h.subscriberQSz)
	if err := h.mgr.RegisterSubscriber(context.Background(), topicName, sub); err != nil {
		plog.Info("register subscriber failed: %v", err)
		return &base.Response{StatusCode: base.StatusServiceUnavailable}, nil
	}
	// store mapping to allow cleanup on session close
	h.mu.Lock()
	h.sessTopic[ctx.Session] = topicName
	h.sessIsPub[ctx.Session] = false
	h.mu.Unlock()
	return &base.Response{StatusCode: base.StatusOK}, nil
}

func (h *serverHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	// cleanup mapping and unregister publisher or subscriber as appropriate
	h.mu.Lock()
	topicName := h.sessTopic[ctx.Session]
	isPub := h.sessIsPub[ctx.Session]
	delete(h.sessTopic, ctx.Session)
	delete(h.sessIsPub, ctx.Session)
	h.mu.Unlock()
	if topicName == "" {
		return
	}
	plog.Debug("session close for topic %s (isPublisher=%v)", topicName, isPub)
	if isPub {
		h.mgr.UnregisterPublisher(topicName)
	} else {
		h.mgr.UnregisterSubscriber(topicName, fmt.Sprintf("%p", ctx.Session))
	}
}
