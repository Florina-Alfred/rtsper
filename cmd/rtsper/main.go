package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	plog "redalf.de/rtsper/pkg/log"
	"syscall"
	"time"

	"redalf.de/rtsper/pkg/admin"
	"redalf.de/rtsper/pkg/rtspsrv"
	"redalf.de/rtsper/pkg/topic"
)

func main() {
	var (
		publishPort            = flag.Int("publish-port", 9191, "RTSP publisher port")
		subscribePort          = flag.Int("subscribe-port", 9192, "RTSP subscriber port")
		adminPort              = flag.Int("admin-port", 8080, "Admin HTTP port")
		maxPublishers          = flag.Int("max-publishers", 5, "Max concurrent publishers")
		maxSubscribersPerTopic = flag.Int("max-subscribers-per-topic", 5, "Max subscribers per topic")
		publisherQueueSize     = flag.Int("publisher-queue-size", 1024, "Per-topic inbound queue size")
		subscriberQueueSize    = flag.Int("subscriber-queue-size", 256, "Per-subscriber queue size")
		publisherGrace         = flag.Duration("publisher-grace", 5*time.Second, "Publisher grace period for reconnect")
	)
	flag.Parse()

	cfg := topic.Config{
		PublishPort:            *publishPort,
		SubscribePort:          *subscribePort,
		MaxPublishers:          *maxPublishers,
		MaxSubscribersPerTopic: *maxSubscribersPerTopic,
		PublisherQueueSize:     *publisherQueueSize,
		SubscriberQueueSize:    *subscriberQueueSize,
		PublisherGracePeriod:   *publisherGrace,
	}

	mgr := topic.NewManager(cfg)

	// start admin server
	mux := http.NewServeMux()
	mux.HandleFunc("/status", admin.StatusHandler(mgr))
	adminSrv := &http.Server{Addr: fmt.Sprintf(":%d", *adminPort), Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		plog.Info("admin: listening on %s", adminSrv.Addr)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			plog.Info("admin server error: %v", err)
		}
	}()

	// start RTSP servers (rtspsrv stub for now)
	rtspSrv := rtspsrv.NewServer(mgr, cfg.PublishPort, cfg.SubscribePort)
	if err := rtspSrv.Start(ctx); err != nil {
		plog.Error("failed to start rtsp servers: %v", err)
		os.Exit(1)
	}

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	plog.Info("shutdown requested")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
	defer shutdownCancel()
	adminSrv.Shutdown(shutdownCtx)
	rtspSrv.Close()
	mgr.Shutdown()
}
