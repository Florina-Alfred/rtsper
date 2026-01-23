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
- Prometheus UI: http://localhost:9090/
- Grafana UI: http://localhost:3000/ (default admin/admin in demo)
- HLS proxy: http://localhost:8088/ (serves `./hls`)

Grafana provisioning

- Grafana is pre-configured to add a Prometheus datasource (`http://prometheus:9090`) and to import the `contrib/grafana/dashboard.json` dashboard. If provisioning did not complete, import the dashboard manually via Dashboards -> Import and select `contrib/grafana/dashboard.json`.

Notes

- The compose stack is intended for local development only. Review and harden before using in production.
- To change Grafana plugins, set `GF_INSTALL_PLUGINS` in `docker-compose.yml`.
