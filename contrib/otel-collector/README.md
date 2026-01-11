This folder provides a minimal local stack to test OTLP metrics from rtsper.

Services:
- OpenTelemetry Collector (receives OTLP/gRPC on :4317 and exposes a Prometheus metrics endpoint on :8888)
- Prometheus (scrapes the Collector on :8888)
- Grafana (for visualization)

Quick start:
1. cd contrib/otel-collector
2. docker compose up -d
3. Start rtsper on the host and point it to the Collector: `./rtsper -otel-endpoint=host.docker.internal:4317` (macOS/Windows) or `./rtsper -otel-endpoint=localhost:4317` on Linux when running docker compose on the same host and ports are mapped.
4. Open Prometheus: http://localhost:9090 and Grafana: http://localhost:3000 (default admin:admin)

Notes:
- On macOS/Windows, use `host.docker.internal` for the OTLP endpoint when rtsper runs on the host and Collector runs in Docker.
- On Linux, you can also run the Collector with `network_mode: host` in docker-compose to simplify networking, or use `localhost:4317` if ports are mapped.
- This is a minimal example intended for local testing only.
