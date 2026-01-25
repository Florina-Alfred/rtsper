
# rtsper — Simple RTSP relay / distributor (MVP)

Minimal RTSP relay server focused on relaying RTSP streams from a single publisher per topic to multiple subscribers.

## Architecture

The server accepts one publisher per topic and fans out the stream to many subscribers. Typical ports:

- Publisher (ingest): :9191
- Subscriber (playback): :9192
- Admin / Metrics: :8080

ASCII overview:

```
                         RTSP (publish)                      RTSP (play)

  +-------------+   ----------------------------->  +----------------------+ 
  |  Publisher  |   rtsp://host:9191/<topic>        |        rtsper        |
  |  (ffmpeg)   |                                   |   (relay & mux)      |
  +-------------+                                   +----------------------+ 
                                                           |
                                                           |  rtsp://host:9192/<topic>
                                                           |  fans out
                                    ------------------------------------------
                                    |                    |                    |
                                    |                    |                    |
                                    v                    v                    v
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

1) Run the published image (minimum required):

```sh
docker run --rm \
  -p 9191:9191 \
  -p 9192:9192 \
  -p 8080:8080 \
  ghcr.io/florina-alfred/rtsper:latest \
  --publish-port=9191 --subscribe-port=9192
```

2) Publish a webcam to a topic (TCP interleaved):

```sh
ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 \
  -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1
```

3) Play the relayed stream from the subscriber port:

```sh
ffplay -rtsp_transport tcp rtsp://localhost:9192/topic1
```

Use `localhost` when running locally.

Clustered mode note

If you use the compose demos, see `contrib/docker-compose/README.md` for details. The quick multi-server example above is the recommended starting point for local testing.

Quick multi-server compose example
---------------------------------

1) Start the 3-node compose demo (pulls `ghcr.io/florina-alfred/rtsper:latest`):

```sh
cd contrib/docker-compose
docker compose -f docker-compose-multi.yml up
```

2) Verify cluster and status (optional):

```sh
curl http://localhost:8080/cluster
curl http://localhost:8080/status
```

3) Stream a webcam to `topic1` (publish to rtsper1):

```sh
ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 \
  -f rtsp -rtsp_transport tcp rtsp://localhost:9191/topic1
```

4) Stream a video file to `topic2` (publish to rtsper2):

```sh
ffmpeg -re -i /path/to/video.mp4 -f rtsp -rtsp_transport tcp rtsp://localhost:9193/topic2
```

5) Play either topic from any server (example uses rtsper3 subscribe mapping):

```sh
ffplay -rtsp_transport tcp rtsp://localhost:9196/topic1
ffplay -rtsp_transport tcp rtsp://localhost:9196/topic2
```

6) Stop the demo:

```sh
docker compose -f docker-compose-multi.yml down
```

Use `localhost` when running locally.

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
