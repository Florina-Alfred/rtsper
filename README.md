rtsper — Simple RTSP relay / distributor (MVP)

Overview
- rtsper accepts RTSP publishers and relays streams to RTSP subscribers.
- TCP-interleaved is the default transport (MVP). UDP is supported optionally.
- Topic names are tokenized: only characters matching `^[A-Za-z0-9_-]+$`.
- Single active publisher per topic. Multiple subscribers allowed (configurable).

Quick start
1. Build:
   - `go build -o rtsper ./cmd/rtsper`
2. Run (TCP only):
   - `./rtsper`
3. Publish (TCP interleaved):
   - `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1`
4. Play (TCP):
   - `ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1`

Enable UDP (example)
- Start server with UDP enabled and base ports:
  - `./rtsper -enable-udp -publisher-udp-base 5004 -subscriber-udp-base 6004`
- Publish over UDP:
  - `ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p -f rtsp -rtsp_transport udp rtsp://localhost:9191/topic1`
- Play over UDP:
  - `ffplay -rtsp_transport udp rtsp://localhost:9192/topic1`

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