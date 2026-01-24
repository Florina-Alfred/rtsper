package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"redalf.de/rtsper/pkg/admin"
	plog "redalf.de/rtsper/pkg/log"
	"redalf.de/rtsper/pkg/metrics"
	"redalf.de/rtsper/pkg/rtspsrv"
	"redalf.de/rtsper/pkg/topic"
	"redalf.de/rtsper/pkg/udpalloc"
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
		// logging options
		logFile  = flag.String("log-file", "", "Path to log file (optional). If set, log rotation is enabled")
		logLevel = flag.String("log-level", "info", "Log level: debug,info,warn,error")
		// OTLP/OTEL endpoint
		otelEndpoint = flag.String("otel-endpoint", "localhost:4317", "OTLP/gRPC collector endpoint (host:port). Empty to disable OTLP init.")
		// allow explicit disabling of OTEL even when endpoint default is set
		disableOtel = flag.Bool("disable-otel", false, "Disable pushing metrics to OTLP collector (overrides --otel-endpoint)")
		// UDP options
		enableUDP         = flag.Bool("enable-udp", false, "Enable UDP RTP/RTCP listeners")
		publisherUDPBase  = flag.Int("publisher-udp-base", 0, "Publisher UDP base port (RTP). RTCP = base+1")
		subscriberUDPBase = flag.Int("subscriber-udp-base", 0, "Subscriber UDP base port (RTP). RTCP = base+1")
		udpPortStart      = flag.Int("udp-port-start", 0, "Start of UDP port range for allocator (inclusive)")
		udpPortEnd        = flag.Int("udp-port-end", 0, "End of UDP port range for allocator (inclusive)")
		configPath        = flag.String("config", "", "Path to JSON config file (optional)")
	)
	flag.Parse()

	// configure logging early
	if *logFile != "" {
		lj := &lumberjack.Logger{
			Filename:   *logFile,
			MaxSize:    100, // megabytes
			MaxBackups: 7,
			MaxAge:     30, // days
		}
		if err := plog.Configure(lj, *logLevel); err != nil {
			plog.Error("invalid log level: %v", err)
			os.Exit(1)
		}
	} else {
		if err := plog.Configure(os.Stdout, *logLevel); err != nil {
			plog.Error("invalid log level: %v", err)
			os.Exit(1)
		}
	}

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

	// UDP allocator and validation
	var allocatorRelease func()
	var alloc *udpalloc.Allocator
	if cfg.EnableUDP {
		// if range provided via flags, use them; otherwise use example defaults
		start := *udpPortStart
		end := *udpPortEnd
		if start == 0 || end == 0 {
			start = 5000
			end = 6000
		}
		var err error
		alloc, err = udpalloc.NewAllocator(start, end)
		if err != nil {
			plog.Error("failed to create UDP allocator: %v", err)
			os.Exit(1)
		}
		p1, rel1, err := alloc.ReservePair()
		if err != nil {
			plog.Error("failed to reserve publisher UDP pair: %v", err)
			os.Exit(1)
		}
		cfg.PublisherUDPBase = p1
		allocatorRelease = rel1

		p2, rel2, err := alloc.ReservePair()
		if err != nil {
			plog.Error("failed to reserve subscriber UDP pair: %v", err)
			// release previous
			rel1()
			os.Exit(1)
		}
		cfg.SubscriberUDPBase = p2
		// wrap releases
		allocatorRelease = func() {
			rel1()
			rel2()
		}
	}

	// validate UDP configuration if enabled. If we used allocator (pre-bound), skip validation.
	if cfg.EnableUDP && alloc == nil {
		if err := validateUDPConfig(cfg); err != nil {
			plog.Error("invalid UDP configuration: %v", err)
			if allocatorRelease != nil {
				allocatorRelease()
			}
			os.Exit(1)
		}
	}

	// initialize OTLP metrics if requested and not explicitly disabled
	if *disableOtel {
		plog.Info("otel metrics disabled via --disable-otel")
	} else if *otelEndpoint != "" {
		if err := metrics.InitOTLP(context.Background(), *otelEndpoint); err != nil {
			plog.Error("failed to initialize OTLP metrics: %v", err)
			if allocatorRelease != nil {
				allocatorRelease()
			}
			os.Exit(1)
		}
	}

	// create manager with final config
	m := topic.NewManager(cfg)

	// start admin server
	mux := http.NewServeMux()
	mux.HandleFunc("/status", admin.StatusHandler(m))
	mux.Handle("/metrics", promhttp.Handler())
	adminSrv := &http.Server{Addr: fmt.Sprintf(":%d", *adminPort), Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		plog.Info("admin: listening on %s", adminSrv.Addr)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			plog.Error("admin server error: %v", err)
		}
	}()

	// start RTSP servers
	rtspSrv := rtspsrv.NewServer(m, cfg.PublishPort, cfg.SubscribePort, alloc)
	if err := rtspSrv.Start(ctx); err != nil {
		plog.Error("failed to start rtsp servers: %v", err)
		if allocatorRelease != nil {
			allocatorRelease()
		}
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
	if allocatorRelease != nil {
		allocatorRelease()
	}
}
