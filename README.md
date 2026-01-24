
# rtsper — Simple RTSP relay / distributor (MVP)

Minimal RTSP relay server focused on relaying RTSP streams from a single publisher per topic to multiple subscribers.

## Architecture

The server accepts one publisher per topic and fans out the stream to many subscribers. Typical ports:

- Publisher (ingest): :9191
- Subscriber (playback): :9192
- Admin / Metrics: :8080

ASCII overview:

```
                        RTSP (publish)                         RTSP (play)
  +-------------+   ------------------------------>   +----------------------+   ------------------------------>
  |  Publisher  |   rtsp://host:9191/<topic>         |        rtsper        |   rtsp://host:9192/<topic>
  |  (ffmpeg)   |                                       |   (relay & mux)     |
  +-------------+                                       +----------+-----------+
                                                                  |
                                                                  |  fans out
                                                                  v
                             +----------------+   +----------------+   +----------------+
                             |  Subscriber 1  |   |  Subscriber 2  |   |  Subscriber N  |
                             |   (ffplay)     |   |   (ffplay)     |   |   (ffplay)     |
                             +----------------+   +----------------+   +----------------+

Notes:
- rtsper accepts one publisher per topic and relays to many subscribers.
- Default ingest port: 9191, playback port: 9192, admin/metrics: 8080.
```

## Key points

- Accepts RTSP publishers on `:9191` and serves subscribers on `:9192`.
- Default transport: TCP interleaved (best for NAT/traversal). UDP is optional and supported when needed.
- Topic names: only characters matching `^[A-Za-z0-9_-]+$` are allowed.

## Run options

### Local (build & run)

- Build the binary:

  ```sh
  go build -o rtsper ./cmd/rtsper
  ```

- Run the server (defaults to TCP interleaved):

  ```sh
  ./rtsper
  ```

- Example publish (TCP interleaved):

  ```sh
  ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 \
    -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p \
    -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1
  ```

- Example play (TCP):

  ```sh
  ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1
  ```

### UDP (optional)

- Enable UDP in the server if you need lower-latency RTP over UDP. For production-like setups use the allocator which pre-binds even/odd RTP/RTCP port pairs from a configured range to avoid bind races:

  ```sh
  ./rtsper -enable-udp -udp-port-start 5000 -udp-port-end 5999
  ```

- Example publish/play commands (UDP):

  ```sh
  # Publisher
  ffmpeg ... -f rtsp -rtsp_transport udp rtsp://localhost:9191/topic1

  # Subscriber
  ffplay -rtsp_transport udp rtsp://localhost:9192/topic1
  ```

### Containerized (docker run)

You can run the published image or a locally-built image with `docker run`. The image includes an entrypoint so flags can be passed directly to the server.

Usage (pull the latest published image):

```sh
docker run --rm \
  -p 9191:9191 \
  -p 9192:9192 \
  -p 8080:8080 \
  ghcr.io/florina-alfred/rtsper:latest \
  --publish-port=9191 --subscribe-port=9192
```

Or, build locally from `contrib/docker-compose/` and run the local image:

```sh
cd contrib/docker-compose
docker build -t rtsper:local .
docker run --rm -p 9191:9191 -p 9192:9192 -p 8080:8080 rtsper:local --publish-port=9191 --subscribe-port=9192
```

Clustered mode (Docker Compose)

- The repository includes a `contrib/docker-compose` demo that starts two rtsper instances with static cluster membership configured via `CLUSTER_NODES` and `NODE_NAME` environment variables.
- Each instance decides ownership of topics using rendezvous hashing. The demo maps the second instance's ports to alternate host ports so both can run locally without collision.
- Only RTSP-over-TCP (RTP-over-TCP) is supported for cross-node routing; UDP transports are intentionally unsupported across nodes in this release.

Example (run compose demo):

    cd contrib/docker-compose
    docker compose up --build

In the demo, the services are `rtsper1` and `rtsper2`. Use `docker compose exec` to run ffmpeg/ffplay within the `rtsnet` network if you want to connect using service names (recommended for routing tests).

Notes:

- The container maps the RTSP publisher (ingest) port 9191, the subscriber (play) port 9192, and the admin/metrics port 8080 to the host. When running on the same machine you can use `localhost` to reach these ports.
- If you run the container on a remote host, replace `localhost` in the examples below with the host IP or hostname.

Publish a webcam (v4l2) from the host to the server's publish port (host-side):

```sh
ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 \
  -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p \
  -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1
```

Play the relayed stream from the subscriber port:

```sh
ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1
```

If you prefer the demo compose stack (includes Prometheus/Grafana and a small HLS proxy) use the compose files in `contrib/docker-compose/` as documented there.

## Configuration file

- Pass a JSON config file using `-config /path/to/config.json`.
- Example `config.json`:

```json
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
```

## Development

- Run tests: `go test ./... -v`
- Format code: `gofmt -w .`

<!-- License removed from repository -->
