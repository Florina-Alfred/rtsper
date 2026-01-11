package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"redalf.de/rtsper/pkg/admin"
	plog "redalf.de/rtsper/pkg/log"
	"redalf.de/rtsper/pkg/rtspsrv"
	"redalf.de/rtsper/pkg/topic"
	"syscall"
	"time"
)

func loadConfig(path string) (topic.Config, error) {
	var cfg topic.Config
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

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
		// UDP options
		enableUDP         = flag.Bool("enable-udp", false, "Enable UDP RTP/RTCP listeners")
		publisherUDPBase  = flag.Int("publisher-udp-base", 0, "Publisher UDP base port (RTP). RTCP = base+1")
		subscriberUDPBase = flag.Int("subscriber-udp-base", 0, "Subscriber UDP base port (RTP). RTCP = base+1")
		configPath        = flag.String("config", "", "Path to JSON config file (optional)")
	)
	flag.Parse()

	fileCfg, err := loadConfig(*configPath)
	if err != nil {
		plog.Error("failed to load config: %v", err)
		os.Exit(1)
	}

	// Merge flags over file config. Flags that are non-default override file.
	cfg := fileCfg
	// basic defaults if file empty
	if cfg.PublisherQueueSize == 0 {
		cfg.PublisherQueueSize = *publisherQueueSize
	}
	if cfg.SubscriberQueueSize == 0 {
		cfg.SubscriberQueueSize = *subscriberQueueSize
	}
	if cfg.MaxPublishers == 0 {
		cfg.MaxPublishers = *maxPublishers
	}
	if cfg.MaxSubscribersPerTopic == 0 {
		cfg.MaxSubscribersPerTopic = *maxSubscribersPerTopic
	}
	if cfg.PublishPort == 0 {
		cfg.PublishPort = *publishPort
	}
	if cfg.SubscribePort == 0 {
		cfg.SubscribePort = *subscribePort
	}
	if cfg.PublisherGracePeriod.Duration == 0 {
		cfg.PublisherGracePeriod.Duration = *publisherGrace
	}
	// flags override file values if explicitly provided
	if *enableUDP {
		cfg.EnableUDP = true
	}
	if *publisherUDPBase != 0 {
		cfg.PublisherUDPBase = *publisherUDPBase
	}
	if *subscriberUDPBase != 0 {
		cfg.SubscriberUDPBase = *subscriberUDPBase
	}

	m := topic.NewManager(cfg)

	// start admin server
	mux := http.NewServeMux()
	mux.HandleFunc("/status", admin.StatusHandler(m))
	adminSrv := &http.Server{Addr: fmt.Sprintf(":%d", *adminPort), Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		plog.Info("admin: listening on %s", adminSrv.Addr)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			plog.Error("admin server error: %v", err)
		}
	}()

	// validate UDP configuration if enabled
	if cfg.EnableUDP {
		if err := validateUDPConfig(cfg); err != nil {
			plog.Error("invalid UDP configuration: %v", err)
			os.Exit(1)
		}
	}

	// start RTSP servers
	rtspSrv := rtspsrv.NewServer(m, cfg.PublishPort, cfg.SubscribePort)
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
	m.Shutdown()
}
