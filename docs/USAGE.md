rtsper — Usage

Quick start

- Build locally:
  - `go build -o rtsper ./cmd/rtsper`
- Run:
  - `./rtsper`
- Publish (TCP interleaved):
  - `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1`
- Play (TCP):
  - `ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1`

UDP (optional)

- Enable UDP allocator for production-like setups:
  - `./rtsper -enable-udp -udp-port-start 5000 -udp-port-end 5999`

Demo compose

- `cd contrib/docker-compose && docker compose up --no-build`

Configuration

- `-config /path/to/config.json` supports the JSON fields described in the README.
