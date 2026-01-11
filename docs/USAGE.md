USAGE — rtsper

Overview

- rtsper is a minimal RTSP relay/distributor. It accepts RTSP publishers and exposes the same topics to RTSP subscribers.
- Default mode uses TCP-interleaved transport. UDP-based RTP/RTCP is supported and can be configured.
- Topic names must match the regex: `^[A-Za-z0-9_-]+$` (e.g. `camera1`, `topic_3`).
- A single active publisher may publish a topic at a time; multiple subscribers may attach to the same topic.

Quick start (build + run)

1. Build the binary:

   `gofmt -w . && go build -o rtsper ./cmd/rtsper`

2. Run with defaults (TCP interleaved):

   `./rtsper`

3. Publish from a webcam (TCP):

   `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1`

4. Play with ffplay (TCP):

   `ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1`

Command-line flags (selected)

- `-publish-port` (int) — RTSP port to accept publishers. Default: `9191`.
- `-subscribe-port` (int) — RTSP port for subscribers. Default: `9192`.
- `-admin-port` (int) — HTTP admin server port (status + `/metrics`). Default: `8080`.
- `-enable-udp` (bool) — Enable UDP RTP/RTCP listeners. Default: `false`.
- `-udp-port-start` / `-udp-port-end` (int) — Port range for the allocator (used when `-enable-udp=true`). Example: `-udp-port-start=5000 -udp-port-end=6000`.
- `-publisher-udp-base` / `-subscriber-udp-base` (int) — Fixed base ports (even numbers) for RTP; RTCP = base+1.
- `-max-publishers` (int) — Max concurrent publishers. Default: `5`.
- `-max-subscribers-per-topic` (int) — Default: `5`.
- `-publisher-queue-size` / `-subscriber-queue-size` (int) — Internal queues for backpressure.
- `-publisher-grace` (duration) — Grace period for reconnects (e.g. `5s`).
- `-log-file` (string) — Path to a rotated log file; enables log rotation.
- `-log-level` (string) — `debug`, `info`, `warn`, `error`. Per-packet logging is `debug` to avoid spam.
- `-otel-endpoint` (string) — OTLP/gRPC collector endpoint (e.g. `localhost:4317`). Empty disables OTLP.
- `-config` (string) — Path to JSON config file (optional). Flags override config values when explicitly supplied.

Configuration file (JSON)

- `rtsper` supports passing a JSON file with the same options as flags. Flags override values present in the file.

Example `config.json` (equivalent to many CLI flags):

```
{
  "PublishPort": 9191,
  "SubscribePort": 9192,
  "AdminPort": 8080,
  "EnableUDP": true,
  "PublisherUDPBase": 5004,
  "SubscriberUDPBase": 6004,
  "MaxPublishers": 8,
  "MaxSubscribersPerTopic": 10,
  "PublisherQueueSize": 1024,
  "SubscriberQueueSize": 256,
  "PublisherGracePeriod": "5s"
}
```

Starting the server with a config file (CLI override example):

- Use the config file directly:

  `./rtsper -config /path/to/config.json`

- Use the config file but override the log level and admin port via flags:

  `./rtsper -config /path/to/config.json -log-level=info -admin-port=8083`

UDP allocator vs explicit base ports

- You can either provide explicit base ports (`-publisher-udp-base`, `-subscriber-udp-base`) or enable the allocator with a port range (`-udp-port-start`, `-udp-port-end`).
- The allocator reserves even RTP/RTCP port pairs from the configured range and pre-binds them to reduce bind races.

Example: use allocator with a 5000–6000 range

`./rtsper -enable-udp -udp-port-start=5000 -udp-port-end=6000`

Publishing examples (multiple ways)

1) Publish from a Linux webcam (v4l2) — TCP (recommended for NATs):

`ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport tcp rtsp://localhost:9191/camera1`

2) Publish from a Linux webcam — UDP (when `-enable-udp=true`):

`ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport udp rtsp://localhost:9191/camera1`

3) Publish a local file (simulate live input) — TCP:

`ffmpeg -re -i sample.mp4 -c:v copy -c:a copy -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1`

4) Publisher helper (included in the repo)

- There are simple publisher helpers in `tmp-disabled/` which can send H264 RTP to the server for manual smoke tests. They use the standard `log` package and write to stdout. Run them with `go run tmp-disabled/pub_h264.go` and redirect output to a file.

Subscribing / playback examples (multiple ways)

1) `ffplay` (TCP):

`ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1`

2) `ffplay` (UDP):

`ffplay -rtsp_transport udp rtsp://localhost:9192/topic1`

3) VLC (GUI):

- Open -> Network -> `rtsp://localhost:9192/topic1` and press Play.

4) VLC (CLI):

`vlc rtsp://localhost:9192/topic1`

5) GStreamer (simple subscriber pipeline):

`gst-launch-1.0 rtspsrc location=rtsp://localhost:9192/topic1 ! rtph264depay ! avdec_h264 ! autovideosink`

6) Save the incoming stream to a file (ffmpeg):

`ffmpeg -rtsp_transport tcp -i rtsp://localhost:9192/topic1 -c copy output.mp4`

Examples showing the same action multiple ways

- Publish a webcam stream (three ways):
  - CLI flags, run server with defaults and publish with `ffmpeg` (TCP):

    `./rtsper`  then  `ffmpeg ... -rtsp_transport tcp rtsp://localhost:9191/camera1`

  - Config file + override admin port:

    `./rtsper -config config.json -admin-port=8083`  then `ffmpeg ... -rtsp_transport tcp rtsp://localhost:9191/camera1`

  - Use UDP via allocator:

    `./rtsper -enable-udp -udp-port-start=5000 -udp-port-end=6000`  then `ffmpeg ... -rtsp_transport udp rtsp://localhost:9191/camera1`

- Subscribe & view the same topic (three ways):
  - `ffplay -rtsp_transport tcp rtsp://localhost:9192/camera1`
  - `vlc rtsp://localhost:9192/camera1`
  - `gst-launch-1.0 rtspsrc location=rtsp://localhost:9192/camera1 ! ...`

Admin & metrics

- The server exposes an admin HTTP server with a status endpoint and Prometheus metrics. Default admin port is `8080`.

  - Status: `curl -sS http://localhost:8080/status`
  - Prometheus metrics: `curl -sS http://localhost:8080/metrics | rg '^rtsper_'` (or open `http://localhost:8080/metrics` in a browser)

- OTLP: set `-otel-endpoint` to enable OTLP export (e.g. `localhost:4317`). Leave empty to disable.

Logging

- Use `-log-level` to control verbosity. Per-packet and other high-frequency messages are logged at `debug` level. Typical usage:

  - Quiet runtime logs but keep warnings/errors: `./rtsper -log-level=warn`
  - Keep startup/info messages: `./rtsper -log-level=info`
  - Verbose per-packet debugging: `./rtsper -log-level=debug`

- To write rotated logs to disk: `./rtsper -log-file /var/log/rtsper.log -log-level=info` (the program uses lumberjack rotation settings).

Notes, tips and gotchas

- Topic name restrictions: use only letters, digits, underscore and hyphen.
- Even/odd ports: RTP uses the configured base port and RTCP uses base+1 (so base must be even when specifying explicit bases).
- When using UDP, ensure firewall/NAT forwards the chosen port ranges.
- Single active publisher per topic: a second publisher that announces the same topic will be rejected.
- If you want an in-process test harness, the repo contains tests under `pkg/` that exercise topic lifecycle and metrics scraping.

Examples for CI or automation

- Start server in the background (warn level), publish via helper, scrape metrics:

```
# Build and start
gofmt -w . && go build -o rtsper ./cmd/rtsper
nohup ./rtsper -log-level=warn > /tmp/rtsper.log 2>&1 & echo $! > /tmp/rtsper.pid

# Run included publisher helper
nohup go run tmp-disabled/pub_h264.go > /tmp/pub.log 2>&1 & echo $!

# Scrape metrics
curl -sS http://localhost:8080/metrics | rg '^rtsper_' || true
```

When to use TCP vs UDP

- TCP-interleaved (default) is simplest and works through NATs and most firewalls — good for local testing and internet-facing deployments where NAT traversal is unknown.
- UDP may provide lower latency and is the traditional transport for RTP, but requires careful firewall and NAT configuration. Use the allocator (`-udp-port-start`/`-udp-port-end`) in multi-tenant or production-like setups to reduce bind races.

Further reading & next steps

- Prometheus scrape examples: `docs/PROMETHEUS.md`
- Example systemd unit: `contrib/systemd/rtsper.service`
- Grafana dashboard (import): `contrib/grafana/dashboard.json`

Grafana notes

- To view RTSP streams inside Grafana you'll need a video plugin. We recommend installing a plugin such as the community `grafana-video-panel` (or any plugin that supports HLS/MJPEG/RTSP via a proxy).

  - Install example (Grafana 8+ with CLI plugin install):

    `grafana-cli plugins install marcusolsson-picture-panel`  # example plugin; replace with actual video plugin

  - After installing, restart Grafana and import `contrib/grafana/dashboard.json` via the Grafana UI (Dashboards -> Import).

- If Grafana cannot directly access RTSP endpoints (NAT/firewall), run a small proxy (RTSP -> HLS or MJPEG) and point the Grafana video panel to the proxy URL.

- Dashboard panels included:
  - Active publishers (gauge)
  - Active subscribers (gauge)
  - Packets received/dispatched (1m rate)
  - Packet drop rate (1m)
  - Text panel with RTSP viewing instructions and plugin notes

- I can add a docker-compose with Prometheus + Grafana + rtsper for a local demo if you want; I added an example under `contrib/docker-compose/`.

Proxy example (RTSP -> HLS)

- A small proxy using `ffmpeg` can convert RTSP to HLS which Grafana video panels can render. See `contrib/proxy/rtsp-to-hls.sh` for a simple demo script.

  Usage example:

  1. Start the script to proxy a topic to HLS:

     `./contrib/proxy/rtsp-to-hls.sh rtsp://localhost:9192/topic1 /tmp/hls-topic1`

  2. Open the produced playlist in a browser or Grafana video panel:

     `http://localhost:8088/index.m3u8`

- The compose uses an nginx-based `hls` service that serves files from `./hls` on port `8088`. Use the provided script `contrib/proxy/rtsp-to-hls.sh` to produce HLS segments into that directory and view them at `http://localhost:8088/index.m3u8`. For production use, serve the HLS directory from a hardened nginx or CDN and tune `ffmpeg` HLS options.


- See `README.md` for a short quickstart. This `docs/USAGE.md` expands on examples and shows multiple ways to accomplish the same tasks.
- If you'd like, I can add: a) a PROMETHEUS scrape example for Prometheus config; b) a systemd service unit file example; or c) an integration test harness that starts server+publisher+subscriber in-process and asserts metrics change. Tell me which you want next.
