rtsper demo (Docker Compose)

This directory contains a small Docker Compose demo stack for local testing:

- rtsper: the RTSP relay (built from the repo)
- prometheus: Prometheus server configured to scrape rtsper metrics
- grafana: Grafana with provisioning to auto-add Prometheus datasource and import the rtsper dashboard
- hls-proxy: a tiny Python-based static server to serve HLS files produced by `ffmpeg`

Quick start

1. From the repository root, start the demo:

   ```sh
   cd contrib/docker-compose
   docker compose up --build
   ```

2. Services

   - rtsper (publisher port 9191, subscriber port 9192, admin/metrics 8080)
   - prometheus UI: http://localhost:9090/
   - grafana UI: http://localhost:3000/ (admin/admin) — change the password after first login
   - hls-proxy: http://localhost:8088/ (serves the `./hls` directory)

Grafana provisioning

- Grafana is pre-configured to add a Prometheus datasource (`Prometheus` -> `http://prometheus:9090`) and to import the `contrib/grafana/dashboard.json` dashboard automatically.
- If provisioning did not complete, you can import the dashboard manually via Dashboards -> Import and selecting the `contrib/grafana/dashboard.json` file.

Plugins

- The compose file attempts to install example plugins via the `GF_INSTALL_PLUGINS` environment variable. The example includes `marcusolsson-picture-panel` and `grafana-image-renderer` as placeholders; replace or extend with an RTSP-capable video plugin of your choice.

HLS proxy usage

- The `hls-proxy` service serves files from the `./hls` directory on the host. Use the provided script `contrib/proxy/rtsp-to-hls.sh` to produce HLS segments into that directory and view them at `http://localhost:8088/index.m3u8`.

Notes

- The compose stack is for local development and demos only. Review and harden before using in production.
- If you want Grafana to install a specific plugin, set the plugin id(s) in `docker-compose.yml` under `GF_INSTALL_PLUGINS`.
