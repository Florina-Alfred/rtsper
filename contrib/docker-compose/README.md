rtsper demo (Docker Compose)

This directory contains a small demo stack for local testing: `rtsper`, Prometheus, Grafana and a small HLS proxy.

Quick start

1. Start the demo using the published image on GitHub Container Registry:

   ```sh
   cd contrib/docker-compose
   docker compose -f docker-compose-multi.yml up
   ```

Services (demo)

- rtsper: publisher 9191, subscriber 9192, admin/metrics 8080
- rtsper1: publisher 9191, subscriber 9192, admin/metrics 8080
- rtsper2: publisher 9193 (host), subscriber 9194 (host), admin/metrics 8081 (host)
- rtsper3: publisher 9195 (host), subscriber 9196 (host), admin/metrics 8082 (host)
 
Note: The compose file pulls `ghcr.io/florina-alfred/rtsper:latest`. Ensure the image is available and that your Docker can access GHCR (public image or authenticated as needed).
- Prometheus UI: http://localhost:9090/
- Grafana UI: http://localhost:3000/ (default admin/admin in demo)
- HLS proxy: http://localhost:8088/ (serves `./hls`)

Grafana provisioning

- Grafana is pre-configured to add a Prometheus datasource (`http://prometheus:9090`) and to import the `contrib/grafana/dashboard.json` dashboard. If provisioning did not complete, import the dashboard manually via Dashboards -> Import and select `contrib/grafana/dashboard.json`.

Notes

- The compose stack is intended for local development only. Review and harden before using in production.
- To change Grafana plugins, set `GF_INSTALL_PLUGINS` in `docker-compose.yml`.

Cluster demo notes

- Each rtsper instance is configured using `CLUSTER_NODES` and `NODE_NAME` environment variables. For this demo the two services are bootstrapped statically:

  - `CLUSTER_NODES=rtsper1,rtsper2,rtsper3`
  - `NODE_NAME` is set per container (rtsper1 / rtsper2)

- The demo maps the second instance's RTSP/admin ports to different host ports to avoid collisions. Clients connecting within the Docker network should use service names (e.g., `rtsper1:9191`).

- For cross-node routing to work, clients must use RTSP over TCP (RTP-over-TCP). UDP transports will not be proxied across nodes; if a client attempts to negotiate UDP and the owner is remote, the server will return an error and recommend TCP transport.

- To test routing locally with ffmpeg/ffplay, connect publishers/subscribers to the appropriate host-mapped ports for each instance, or use `docker compose exec` to run clients inside the `rtsnet` network.

Step-by-step: Multi-server quick test
-----------------------------------

1) Start the 3-node compose stack

   ```sh
   cd contrib/docker-compose
   docker compose -f docker-compose-multi.yml up
   ```

   Each rtsper instance is reachable on the host via the following mapped ports:
   - rtsper1: publish `localhost:9191`, subscribe `localhost:9192`, admin `localhost:8080`
   - rtsper2: publish `localhost:9193`, subscribe `localhost:9194`, admin `localhost:8081`
   - rtsper3: publish `localhost:9195`, subscribe `localhost:9196`, admin `localhost:8082`

2) Verify cluster and status

   ```sh
   curl http://localhost:8080/cluster
   curl http://localhost:8080/status
   curl http://localhost:8081/cluster
   curl http://localhost:8082/cluster
   ```

3) Stream a webcam to a topic (topic name: `webcam1`)

   - If you have a Linux webcam device `/dev/video0`:

     ```sh
     ffmpeg -f v4l2 -framerate 30 -video_size 640x480 -i /dev/video0 \
       -f rtsp -rtsp_transport tcp rtsp://localhost:9191/webcam1
     ```

   - If you don't have a webcam, publish a test video source (synthetic test pattern):

     ```sh
     ffmpeg -f lavfi -i testsrc=size=640x480:rate=30 -f rtsp -rtsp_transport tcp rtsp://localhost:9191/webcam1
     ```

   You may publish the same topic from any server; the node you connect to will proxy the TCP connection to the owner. We use `rtsper1`'s publish port in the examples above.

4) Stream a video file to another topic (topic name: `movie1`)

   ```sh
   ffmpeg -re -i /path/to/movie.mp4 -f rtsp -rtsp_transport tcp rtsp://localhost:9193/movie1
   ```

   The example publishes to `rtsper2`'s publish port. The server will accept and proxy as needed.

5) Play either topic from any server (use TCP interleaved)

   - Play `webcam1` by connecting to `rtsper3`'s subscribe port (host-mapped):

     ```sh
     ffplay -rtsp_transport tcp rtsp://localhost:9196/webcam1
     ```

   - Play `movie1` by connecting to `rtsper1`'s subscribe port:

     ```sh
     ffplay -rtsp_transport tcp rtsp://localhost:9192/movie1
     ```

   The subscriber node will transparently proxy the TCP connection to the topic owner if it is remote.

6) Notes and troubleshooting

   - If you see `RTSP/1.0 461 Unsupported Transport` when SETUP is attempted, the client tried to negotiate UDP. Re-run the client with `-rtsp_transport tcp` to force interleaved RTP-over-TCP.
   - Check admin endpoints for cluster state and topics: `/cluster`, `/status` and see `/metrics` for forwarding counters (e.g., `rtsper_forwarded_connections_total`).
   - If publishing from a remote machine to the host running the compose stack, use the host IP and mapped ports (e.g., `rtsp://host-ip:9193/movie1`). Ensure any firewall allows those ports.
   - For in-network testing (avoid host mapping), run clients inside the compose network and use service names (`rtsper1:9191`, `rtsper2:9191`, `rtsper3:9191`) — e.g. `docker compose -f docker-compose-multi.yml exec rtsper1 ffmpeg ... rtsp://rtsper1:9191/webcam1`.

7) Stopping the demo

   ```sh
   docker compose -f docker-compose-multi.yml down
   ```

This quick flow lets you publish different sources to different topics and subscribe from any server; the cluster routing and proxying handle directing clients to the topic owner.
