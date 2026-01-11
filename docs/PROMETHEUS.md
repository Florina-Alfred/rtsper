Prometheus — scraping rtsper metrics

Overview

- rtsper exposes Prometheus-compatible metrics on the admin HTTP server under `/metrics` (default admin port `8080`).
- Metric names are prefixed with `rtsper_` (e.g. `rtsper_packets_received_total`).

Basic Prometheus scrape config

- Add a `scrape_configs` job to your `prometheus.yml` to scrape the server's metrics.

Example `prometheus.yml` snippet (scrape local instance on default admin port 8080):

```
scrape_configs:
  - job_name: 'rtsper'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
    scheme: http
```

If you run rtsper on a non-default port or host, change the target accordingly.

Example: scrape multiple rtsper instances

```
scrape_configs:
  - job_name: 'rtsper-cluster'
    static_configs:
      - targets: ['rtsper1.example:8080', 'rtsper2.example:8080']
    metrics_path: /metrics
```

Relabeling / recording rules

- You can use relabeling to attach instance labels or job-specific labels. For example, add `instance` label and a `cluster` label:

```
relabel_configs:
  - source_labels: [__address__]
    target_label: instance
  - replacement: 'prod-cluster-a'
    target_label: cluster
```

Useful metrics

- `rtsper_active_publishers` — number of active publishers (gauge)
- `rtsper_active_subscribers` — number of active subscribers (gauge)
- `rtsper_packets_received_total` — total RTP packets accepted from publishers
- `rtsper_packets_dispatched_total` — total RTP packets dispatched to subscribers
- `rtsper_packets_dropped_total` — packets dropped due to queue pressure or other reasons
- `rtsper_publishers_registered_total` — total publisher registration events
- `rtsper_subscribers_registered_total` — total subscriber registration events

Alerting examples (very basic)

- Alert when no publishers are active for a period of time:

```
- alert: NoActivePublishers
  expr: rtsper_active_publishers == 0
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "No active rtsper publishers"
    description: "No active publishers for 5 minutes"
```

- Alert when packet drops exceed a threshold (example rate over 1m):

```
- alert: HighPacketDropRate
  expr: rate(rtsper_packets_dropped_total[1m]) > 10
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "High packet drop rate on rtsper"
    description: "More than 10 packets dropped per second (1m rate)"
```

Prometheus + Grafana

- Use the metrics above to create dashboards for publisher/subscriber counts and packet loss.
- Example dashboard panels:
  - Active publishers (gauge): `rtsper_active_publishers`
  - Packets in/out over time: `rate(rtsper_packets_received_total[1m])`, `rate(rtsper_packets_dispatched_total[1m])`
  - Packet drop rate: `rate(rtsper_packets_dropped_total[1m])`

Notes

- If you enable OTLP (set `-otel-endpoint`), metrics may also be exported to your OTLP collector in addition to Prometheus scraping.
- The admin endpoint is plain HTTP by default; consider running it behind a reverse proxy or applying TLS with a proxy when exposing it publicly.
