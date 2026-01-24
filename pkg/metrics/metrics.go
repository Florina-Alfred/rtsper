package metrics

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"go.opentelemetry.io/otel"
	otlpmetricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	plog "redalf.de/rtsper/pkg/log"
)

var (
	meter metric.Meter

	// OTLP instruments
	activePublishers       metric.Int64UpDownCounter
	totalPublishers        metric.Int64Counter
	activeSubscribers      metric.Int64UpDownCounter
	totalSubscribers       metric.Int64Counter
	packetsReceived        metric.Int64Counter
	packetsDispatched      metric.Int64Counter
	packetsDropped         metric.Int64Counter
	allocatorReservations  metric.Int64Counter
	allocatorReservedPairs metric.Int64UpDownCounter

	// Prometheus equivalents
	promActivePublishers       prometheus.Gauge
	promTotalPublishers        prometheus.Counter
	promActiveSubscribers      prometheus.Gauge
	promTotalSubscribers       prometheus.Counter
	promPacketsReceived        prometheus.Counter
	promPacketsDispatched      prometheus.Counter
	promPacketsDropped         prometheus.Counter
	promAllocatorReservations  prometheus.Counter
	promAllocatorReservedPairs prometheus.Gauge
	// forwarding metrics
	promForwardedConnections prometheus.Counter
	promForwardedBytes       prometheus.Counter
	promForwardFailed        prometheus.Counter
)

func init() {
	// create and register Prometheus metrics mirroring the OTLP instruments
	promActivePublishers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "rtsper_active_publishers",
		Help: "Number of active publishers",
	})
	promTotalPublishers = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_publishers_registered_total",
		Help: "Total publishers registered",
	})
	promActiveSubscribers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "rtsper_active_subscribers",
		Help: "Number of active subscribers",
	})
	promTotalSubscribers = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_subscribers_registered_total",
		Help: "Total subscribers registered",
	})
	promPacketsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_packets_received_total",
		Help: "Total packets received",
	})
	promPacketsDispatched = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_packets_dispatched_total",
		Help: "Total packets dispatched",
	})
	promPacketsDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_packets_dropped_total",
		Help: "Total packets dropped",
	})
	promAllocatorReservations = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_allocator_reservations_total",
		Help: "Total allocator reservations",
	})
	promAllocatorReservedPairs = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "rtsper_allocator_reserved_pairs",
		Help: "Current number of reserved allocator pairs",
	})

	// forwarding metrics
	promForwardedConnections = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_forwarded_connections_total",
		Help: "Total number of connections forwarded to other cluster nodes",
	})

	promForwardedBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_forwarded_bytes_total",
		Help: "Total bytes forwarded to other cluster nodes",
	})

	promForwardFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "rtsper_forward_failed_total",
		Help: "Total failed attempts to forward connections to other nodes",
	})

	// Register metrics
	prometheus.MustRegister(
		promActivePublishers,
		promTotalPublishers,
		promActiveSubscribers,
		promTotalSubscribers,
		promPacketsReceived,
		promPacketsDispatched,
		promPacketsDropped,
		promAllocatorReservations,
		promAllocatorReservedPairs,
		promForwardedConnections,
		promForwardedBytes,
		promForwardFailed,
	)
}

// Forwarding metrics helpers
func IncForwardedConnections() {
	if promForwardedConnections != nil {
		promForwardedConnections.Inc()
	}
}

func AddForwardedBytes(n int64) {
	if promForwardedBytes != nil {
		promForwardedBytes.Add(float64(n))
	}
}

func IncForwardFailed() {
	if promForwardFailed != nil {
		promForwardFailed.Inc()
	}
}

// InitOTLP initializes an OTLP exporter to the provided endpoint (host:port)
// and configures a MeterProvider that exports periodically. If endpoint is
// empty, InitOTLP is a no-op and returns nil. This avoids attempting to
// connect to localhost from containerized environments when the caller
// neglected to set an endpoint.
func InitOTLP(ctx context.Context, endpoint string) error {
	if endpoint == "" {
		return nil
	}

	// tryInit attempts to set up the exporter and meter provider.
	tryInit := func() error {
		// Resolve the OTLP hostname first to avoid creating the exporter when
		// DNS is failing (this avoids repeated exporter/dial errors that show
		// as "dns: A record lookup error" in logs). Use a short timeout for
		// DNS resolution.
		host, _, err := net.SplitHostPort(endpoint)
		if err != nil {
			// If endpoint doesn't include a port, treat the whole string as host
			host = endpoint
		}
		rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupHost(rctx, host)
		if err != nil {
			return fmt.Errorf("dns lookup failed for %s: %w", host, err)
		}
		plog.Debug("otel: resolved %s -> %v", host, addrs)

		exporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(endpoint), otlpmetricgrpc.WithInsecure())
		if err != nil {
			return err
		}

		// Use a slightly longer export interval to reduce noisy retry logs
		// when the collector is temporarily unavailable.
		reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second))
		provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		otel.SetMeterProvider(provider)
		meter = provider.Meter("rtsper")

		// create instruments
		var e error
		activePublishers, e = meter.Int64UpDownCounter("rtsper_active_publishers")
		if e != nil {
			return e
		}
		totalPublishers, e = meter.Int64Counter("rtsper_publishers_registered_total")
		if e != nil {
			return e
		}
		activeSubscribers, e = meter.Int64UpDownCounter("rtsper_active_subscribers")
		if e != nil {
			return e
		}
		totalSubscribers, e = meter.Int64Counter("rtsper_subscribers_registered_total")
		if e != nil {
			return e
		}
		packetsReceived, e = meter.Int64Counter("rtsper_packets_received_total")
		if e != nil {
			return e
		}
		packetsDispatched, e = meter.Int64Counter("rtsper_packets_dispatched_total")
		if e != nil {
			return e
		}
		packetsDropped, e = meter.Int64Counter("rtsper_packets_dropped_total")
		if e != nil {
			return e
		}
		allocatorReservations, e = meter.Int64Counter("rtsper_allocator_reservations_total")
		if e != nil {
			return e
		}
		allocatorReservedPairs, e = meter.Int64UpDownCounter("rtsper_allocator_reserved_pairs")
		if e != nil {
			return e
		}

		return nil
	}

	// Try once synchronously. If it fails, log a warning and retry
	// in the background so the service can start up even if the collector
	// is not yet reachable.
	if err := tryInit(); err == nil {
		plog.Info("otel: metrics exporter initialized")
		return nil
	} else {
		plog.Warn("otel: initial metrics exporter init failed: %v; starting background retry", err)
	}

	go func() {
		backoff := 5 * time.Second
		for {
			select {
			case <-ctx.Done():
				plog.Info("otel: stopping init retries due to context cancellation")
				return
			case <-time.After(backoff):
			}
			if err := tryInit(); err == nil {
				plog.Info("otel: metrics exporter initialized (background)")
				return
			} else {
				plog.Warn("otel: metrics exporter init retry failed: %v; retrying in %s", err, backoff)
			}
			if backoff < 60*time.Second {
				backoff *= 2
			}
		}
	}()
	return nil
}

func IncActivePublishers() {
	if activePublishers != nil {
		activePublishers.Add(context.Background(), 1)
	}
	if promActivePublishers != nil {
		promActivePublishers.Add(1)
	}
}
func DecActivePublishers() {
	if activePublishers != nil {
		activePublishers.Add(context.Background(), -1)
	}
	if promActivePublishers != nil {
		promActivePublishers.Add(-1)
	}
}

// AddActivePublishers adjusts active publishers gauge by delta (positive or negative).
func AddActivePublishers(delta int64) {
	if activePublishers != nil {
		activePublishers.Add(context.Background(), delta)
	}
	if promActivePublishers != nil {
		promActivePublishers.Add(float64(delta))
	}
}
func IncTotalPublishers() {
	if totalPublishers != nil {
		totalPublishers.Add(context.Background(), 1)
	}
	if promTotalPublishers != nil {
		promTotalPublishers.Add(1)
	}
}

func IncActiveSubscribers() {
	if activeSubscribers != nil {
		activeSubscribers.Add(context.Background(), 1)
	}
	if promActiveSubscribers != nil {
		promActiveSubscribers.Add(1)
	}
}
func DecActiveSubscribers() {
	if activeSubscribers != nil {
		activeSubscribers.Add(context.Background(), -1)
	}
	if promActiveSubscribers != nil {
		promActiveSubscribers.Add(-1)
	}
}

// AddActiveSubscribers adjusts active publishers gauge by delta (positive or negative).
func AddActiveSubscribers(delta int64) {
	if activeSubscribers != nil {
		activeSubscribers.Add(context.Background(), delta)
	}
	if promActiveSubscribers != nil {
		promActiveSubscribers.Add(float64(delta))
	}
}
func IncTotalSubscribers() {
	if totalSubscribers != nil {
		totalSubscribers.Add(context.Background(), 1)
	}
	if promTotalSubscribers != nil {
		promTotalSubscribers.Add(1)
	}
}

func IncPacketsReceived() {
	if packetsReceived != nil {
		packetsReceived.Add(context.Background(), 1)
	}
	if promPacketsReceived != nil {
		promPacketsReceived.Add(1)
	}
}
func IncPacketsDispatched() {
	if packetsDispatched != nil {
		packetsDispatched.Add(context.Background(), 1)
	}
	if promPacketsDispatched != nil {
		promPacketsDispatched.Add(1)
	}
}
func IncPacketsDropped() {
	if packetsDropped != nil {
		packetsDropped.Add(context.Background(), 1)
	}
	if promPacketsDropped != nil {
		promPacketsDropped.Add(1)
	}
}

func IncAllocatorReservations() {
	if allocatorReservations != nil {
		allocatorReservations.Add(context.Background(), 1)
	}
	if promAllocatorReservations != nil {
		promAllocatorReservations.Add(1)
	}
}
func IncAllocatorReservedPairs() {
	if allocatorReservedPairs != nil {
		allocatorReservedPairs.Add(context.Background(), 1)
	}
	if promAllocatorReservedPairs != nil {
		promAllocatorReservedPairs.Add(1)
	}
}
func DecAllocatorReservedPairs() {
	if allocatorReservedPairs != nil {
		allocatorReservedPairs.Add(context.Background(), -1)
	}
	if promAllocatorReservedPairs != nil {
		promAllocatorReservedPairs.Add(-1)
	}
}
func SetAllocatorReservedPairs(n int64) { /* no direct set in OTEL, use up/down adjustments */
	_ = n
}
