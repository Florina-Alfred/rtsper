package rtspsrv

import (
	"context"
	"fmt"
	plog "redalf.de/rtsper/pkg/log"
	"regexp"
	"strings"
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"

	"redalf.de/rtsper/pkg/topic"
)

// Server holds RTSP servers for publisher and subscriber
type Server struct {
	mgr     *topic.Manager
	pubPort int
	subPort int
	pubSrv  *gortsplib.Server
	subSrv  *gortsplib.Server
	mu      sync.Mutex
	h       *serverHandler
}

func NewServer(mgr *topic.Manager, pubPort, subPort int) *Server {
	return &Server{mgr: mgr, pubPort: pubPort, subPort: subPort}
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
	}

	subSrv := &gortsplib.Server{Handler: h, RTSPAddress: fmt.Sprintf(":%d", s.subPort)}
	if mgrCfg.EnableUDP && mgrCfg.SubscriberUDPBase > 0 {
		subSrv.UDPRTPAddress = fmt.Sprintf(":%d", mgrCfg.SubscriberUDPBase)
		subSrv.UDPRTCPAddress = fmt.Sprintf(":%d", mgrCfg.SubscriberUDPBase+1)
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
	plog.Info("conn open %v", ctx.Conn.NetConn().RemoteAddr())
}

func (h *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	plog.Info("conn close %v", ctx.Conn.NetConn().RemoteAddr())
}

func (h *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	topicName := strings.TrimPrefix(ctx.Path, "/")
	plog.Info("describe %s", topicName)
	st := h.mgr.GetTopicStream(topicName)
	if st != nil {
		return &base.Response{StatusCode: base.StatusOK}, st, nil
	}
	return &base.Response{StatusCode: base.StatusNotFound}, nil, nil
}

func (h *serverHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	topicName := strings.TrimPrefix(ctx.Path, "/")
	plog.Info("announce %s", topicName)
	if !h.topicNameRe.MatchString(topicName) {
		plog.Info("invalid topic name: %s", topicName)
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
	plog.Info("record %s", ctx.Path)
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
	st.WritePacketRTP(ctx.TrackID, ctx.Packet)
}

func (h *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	topicName := strings.TrimPrefix(ctx.Path, "/")
	plog.Info("setup %s", topicName)
	st := h.mgr.GetTopicStream(topicName)
	if st != nil {
		return &base.Response{StatusCode: base.StatusOK}, st, nil
	}
	return &base.Response{StatusCode: base.StatusNotFound}, nil, nil
}

func (h *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	topicName := strings.TrimPrefix(ctx.Path, "/")
	plog.Info("play %s", topicName)
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
	plog.Info("session close for topic %s (isPublisher=%v)", topicName, isPub)
	if isPub {
		h.mgr.UnregisterPublisher(topicName)
	} else {
		h.mgr.UnregisterSubscriber(topicName, fmt.Sprintf("%p", ctx.Session))
	}
}
