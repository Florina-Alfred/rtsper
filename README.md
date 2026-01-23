[title] rtsper — Simple RTSP relay / distributor (MVP)

Minimal RTSP relay server. The repository focuses on the server runtime and usage examples. See `docs/USAGE.md` for quickstart, examples and configuration details. Optional CI/maintenance notes live in `docs/CI.md`.

Overview
- rtsper accepts RTSP publishers and relays streams to RTSP subscribers.
- TCP-interleaved is the default transport (MVP). UDP is supported optionally.
- Topic names are tokenized: only characters matching `^[A-Za-z0-9_-]+$`.
- Single active publisher per topic. Multiple subscribers allowed (configurable).

App architecture (detailed ASCII)

Below is the previous high-level diagram restored and expanded. It shows the user roles (Publisher, Subscriber, Admin), network boundaries (user LAN, Internet, server/container), transports (TCP interleaved vs UDP/RTP pairs), and optional components (allocator, HLS, Prometheus/Grafana). Read the arrows left-to-right for publish -> relay -> subscribe, and the dotted lines for observability/ops.

```
  [Publisher User]
  - runs ffmpeg/encoder locally on laptop / edge device
  - sits on LAN or behind NAT
  +----------------------+                       INTERNET / LAN NAT
  | Laptop / Device      |                                  |
  | ffmpeg (push)        | - RTSP (tcp/udp) to :9191        v
  |                      |                                  |
  |  Quick command:      |  Example (TCP interleaved):      |
  |  ffmpeg -f v4l2 ...  |  ffmpeg -f v4l2 -framerate 30   |
  |  rtsp://<host>:9191/ |    -video_size 640x480 -i /dev/video0 |
  |  topic1              |    -c:v libx264 -preset veryfast  \
  +----------+-----------+    -tune zerolatency -pix_fmt yuv420p \
             |                -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1
             | (push)
             |                                      +---------------+
             |                                      |  rtsper       |
             |                                      |  (container)  |
             |                                      |               |
             |                                      |  Listener     |  <--- binds PublishPort (:9191)
             |                                      |  Topic Manager|  (accepts single active publisher per topic)
             |                                      |  Queues       |
             |                                      |  Dispatcher   |  <--- dispatches to subscribers
             |                                      |  UDP Allocator|  (optional: pre-bind RTP/RTCP pairs)
             |                                      |  HLS Output   |  (optional)
             |                                      |  Metrics HTTP |  (admin: :8080/metrics)
             |                                      +-------+-------+
             |                                              |
             |                                              | RTSP (tcp interleaved)
             |                                              | or RTP/UDP pairs (via allocator)
             |                                              v
  +----------v-----------+                          +-------+-------+
  |  Subscriber(s)       |                          |  Admin / Ops  |
  |  (ffplay, RTSP pull) |                          |  (prometheus, |
  |                      |                          |   grafana)    |
  |  Quick command:      |                          |               |
  |  ffplay (TCP):       |                          |  Quick checks:|
  |  ffplay -rtsp_transport tcp \
  |    rtsp://localhost:9192/topic1 |           |  curl http://localhost:8080/metrics |
  +----------------------+                          |  docker compose up -d (demo) |
     ^     ^   ^                                     +---------------+
     |     |   | pull from rtsper (:9192)
     |     |   +-- Subscriber may be local or remote (NAT, firewall)
     |     |
     |     +----- multiple subscribers attached to same topic (fan-out)
     |
     +----------- single active publisher for the topic (others rejected)

Observability & demo stack (optional):
 - Prometheus scrapes `http://rtsper:8080/metrics` for counters/gauges (topics, subscribers, packets).
 - Grafana reads Prometheus to display dashboards; demo compose includes a prebuilt dashboard.
 - HLS nginx can be used to serve HLS fragments generated from streams for HTTP playback.

Where the user is:
 - Publisher user: runs ffmpeg on a laptop/edge device and opens an outbound RTSP connection to rtsper:9191 (often behind NAT).
 - Subscriber user: runs ffplay or an RTSP client and connects to rtsper:9192 to receive the stream.
 - Admin/Ops: accesses metrics/UI (Prometheus/Grafana) and may operate rtsper from the host/container environment.

Transport details (quick):
 - TCP interleaved (default): RTSP control and RTP/RTCP travel over the same TCP connection — good for NAT traversal and simple setups.
 - UDP (optional): rtsper or the allocator binds even/odd RTP/RTCP port pairs and forwards packets; lower latency but requires firewall and NAT configuration.

Notes:
 - Topic names are tokenized and restricted (alphanumeric, underscore, dash).
 - A topic has at most one active publisher; additional publishers for the same topic are rejected or queued per configuration.
 - When using UDP allocator, ensure the configured port range is open and even/odd pairs are available.



Run options

- Local (build & run):
  - Build the binary:
    - `go build -o rtsper ./cmd/rtsper`
  - Run the server (defaults to TCP interleaved):
    - `./rtsper`
  - Example publish (TCP interleaved):
    - `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1`
  - Example play (TCP):
    - `ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1`

 - Containerized (local image):
   - Build and use a local image (no push to remote registry):
     1. Build the image locally (from repo root):
        - `docker build -f contrib/docker-compose/Dockerfile -t rtsper:local .`
     2. Start the demo stack using the local image compose file (the original `docker-compose.yml` already builds the image):
        - `cd contrib/docker-compose && docker compose up --build`

   - Notes:
     - The demo compose brings up `rtsper`, Prometheus, Grafana and a simple nginx HLS server.
     - `rtsper` admin/metrics endpoint is available at `http://localhost:8080/metrics`.
     - Prometheus UI: `http://localhost:9090`; Grafana UI: `http://localhost:3000` (admin/admin in the demo).

   - If you need CI/publishing details (optional registries, scanning), see `docs/CI.md`.

Quick start (legacy)
- Quick CLI example (same as before):
  - Build: `go build -o rtsper ./cmd/rtsper`
  - Run: `./rtsper`
  - Publish: `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1`
  - Play: `ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1`

Enable UDP (basic example)
- Start server with UDP enabled and explicit base ports:
  - `./rtsper -enable-udp -publisher-udp-base 5004 -subscriber-udp-base 6004`
- Publish over UDP:
  - `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport udp rtsp://localhost:9191/topic1`
- Play over UDP:
  - `ffplay -rtsp_transport udp rtsp://localhost:9192/topic1`

Enable UDP with allocator (recommended for production-like setups)
- The allocator reserves even RTP/RTCP port pairs from a configurable range and pre-binds sockets to avoid race conditions when the RTSP server tries to bind the same ports.
- Start server with allocator using a port range:
  - `./rtsper -enable-udp -udp-port-start 5000 -udp-port-end 5999`
  - The server will reserve two port pairs from the range and expose them as `PublisherUDPBase` and `SubscriberUDPBase`.
- Example publish and play commands (UDP):
  - Publisher: `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport udp rtsp://localhost:9191/topic1`
  - Subscriber: `ffplay -rtsp_transport udp rtsp://localhost:9192/topic1`

Configuration file
- You can pass a JSON config file using `-config /path/to/config.json`.
- Example `config.json` (human-friendly durations supported):

{
  "PublishPort": 9191,
  "SubscribePort": 9192,
  "EnableUDP": true,
  "PublisherUDPBase": 5004,
  "SubscriberUDPBase": 6004,
  "MaxPublishers": 5,
  "MaxSubscribersPerTopic": 5,
  "PublisherQueueSize": 1024,
  "SubscriberQueueSize": 256,
  "PublisherGracePeriod": "5s"
}

Notes
- RTP base ports must be even; RTCP uses base+1.
- When using UDP, ensure firewall allows the configured ports.
- For production use consider per-session UDP port allocation and NAT/firewall traversal.

Development
- Run tests: `go test ./... -v`
- Format code: `gofmt -w .`

License: MIT (not included)
