rtsper — Simple RTSP relay / distributor (MVP)

Minimal RTSP relay server focused on relaying RTSP streams from a single publisher per topic to multiple subscribers.

Key points
- Accepts RTSP publishers on :9191 and serves subscribers on :9192.
- Default transport: TCP interleaved (best for NAT/traversal). UDP is optional and supported when needed.
- Topic names: only characters matching `^[A-Za-z0-9_-]+$` are allowed.

Run options

- Local (build & run)
  - Build the binary:
    - `go build -o rtsper ./cmd/rtsper`
  - Run the server (defaults to TCP interleaved):
    - `./rtsper`
  - Example publish (TCP interleaved):
    - `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1`
  - Example play (TCP):
    - `ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1`

- UDP (optional)
  - Enable UDP in the server if you need lower-latency RTP over UDP. For production-like setups use the allocator which pre-binds even/odd RTP/RTCP port pairs from a configured range to avoid bind races:
    - `./rtsper -enable-udp -udp-port-start 5000 -udp-port-end 5999`
  - Example publish/play commands (UDP):
    - Publisher: `ffmpeg ... -f rtsp -rtsp_transport udp rtsp://localhost:9191/topic1`
    - Subscriber: `ffplay -rtsp_transport udp rtsp://localhost:9192/topic1`

- Containerized (demo compose)
  - The repository includes a demo docker-compose stack at `contrib/docker-compose/` that runs rtsper, Prometheus and Grafana for local testing.
  - If you already have the image available locally (tagged `rtsper:local`), prefer using it instead of rebuilding:
    1) Start the demo without rebuilding the image:
       - `cd contrib/docker-compose && docker compose up --no-build`
    2) If you need to build the image locally (first time):
       - `cd contrib/docker-compose && docker compose up --build`
  - Demo service ports:
    - rtsper: publisher 9191, subscriber 9192, admin/metrics 8080
    - Prometheus UI: http://localhost:9090
    - Grafana UI: http://localhost:3000 (default admin/admin in demo)

Configuration file
- Pass a JSON config file using `-config /path/to/config.json`.
- Example `config.json`:

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

Development
- Run tests: `go test ./... -v`
- Format code: `gofmt -w .`

License: MIT
