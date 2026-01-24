rtsper demo (Docker Compose)

This directory contains a small demo stack for local testing: `rtsper`, Prometheus, Grafana and a small HLS proxy.

Quick start

1. From the repository root, start the demo using the prebuilt local image (preferred when you already have `rtsper:local`):

   ```sh
   cd contrib/docker-compose
   docker compose up --no-build
   ```

2. If you do not have the local image, build it once and start the stack:

   ```sh
   cd contrib/docker-compose
   docker compose up --build
   ```

Services (demo)

- rtsper: publisher 9191, subscriber 9192, admin/metrics 8080
 - rtsper1: publisher 9191, subscriber 9192, admin/metrics 8080
 - rtsper2: publisher 9193 (host), subscriber 9194 (host), admin/metrics 8081 (host)
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

  - `CLUSTER_NODES=rtsper1,rtsper2`
  - `NODE_NAME` is set per container (rtsper1 / rtsper2)

- The demo maps the second instance's RTSP/admin ports to different host ports to avoid collisions. Clients connecting within the Docker network should use service names (e.g., `rtsper1:9191`).

- For cross-node routing to work, clients must use RTSP over TCP (RTP-over-TCP). UDP transports will not be proxied across nodes; if a client attempts to negotiate UDP and the owner is remote, the server will return an error and recommend TCP transport.

- To test routing locally with ffmpeg/ffplay, connect publishers/subscribers to the appropriate host-mapped ports for each instance, or use `docker compose exec` to run clients inside the `rtsnet` network.
