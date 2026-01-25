Loadtest tool
==============

This folder documents the simple load testing tool included at `cmd/loadtest`.

Overview
--------

The loadtest program starts N publisher+subscriber pairs that use `ffmpeg` to push and pull RTSP streams against your rtsper cluster. It is intended for quick local load tests and functional smoke tests (not for precise benchmarking).

Prerequisites
-------------

- `ffmpeg` available in your `PATH` (used to publish and pull streams)
- Go toolchain to build the loadtest binary (`go 1.24` or compatible)
- A running rtsper stack (see `contrib/docker-compose/docker-compose-multi.yml`)

Files
-----

`docs/loadtest` — the Go program (binary entrypoint). A self-contained copy of the tool is included in this folder for convenience.

Usage
-----

Build the tool:

```sh
go build -o bin/loadtest ./docs/loadtest
```

Basic run (100 pairs, default):

```sh
./bin/loadtest
```

Common flags

- `-count` (int) — number of publisher+subscriber pairs to create (default: 100)
- `-file` (string) — path to source media file to publish (default: `docs/test_footage.mp4`)
- `-publish` (string) — publish server host:port (default: `localhost:9191`)
- `-subscribe` (string) — subscribe server host:port (default: `localhost:9192`)
- `-delay` (duration) — delay between starting each pair (default: `50ms`)
- `-out` (string) — output directory for per-stream logs (default: `loadtest-logs`)
- `-duration` (duration) — optional duration to run the test (0 means until interrupted)

Examples
--------

Start 100 pairs against the local compose demo and run for 30 seconds:

```sh
./bin/loadtest -count 100 -file docs/test_footage.mp4 -publish localhost:9191 -subscribe localhost:9192 -duration 30s
```

Run 200 pairs with a small stagger to avoid spikes:

```sh
./bin/loadtest -count 200 -delay 100ms
```

Logs
----

Per-stream stdout/stderr are written to the `-out` directory (default `loadtest-logs`) as `pub_topicX.log` and `sub_topicX.log`. These files help diagnose failures (connectivity, codec errors, server rejections).

Notes & tips
------------

- The tool uses `ffmpeg` with `-stream_loop -1` for publishers to continuously stream the provided file. Publishers use `-c copy` where possible to reduce CPU overhead.
- Subscribers pull and direct output to `-f null -` to discard received media while still exercising the protocol paths.
- For higher scale tests, ensure your host has enough file descriptors (`ulimit -n`) and CPU — each ffmpeg process consumes resources.
- You can run the tool from any machine that can reach the rtsper publish/subscribe endpoints.
- This tool is not a drop-in performance benchmark. For more controlled load testing consider using a purpose-built load generator or a GStreamer-based variant.

Stopping
--------

Use CTRL+C to stop the tool. It will attempt graceful shutdown of child processes and kill any remaining ones after a short timeout.

Further improvements
--------------------

- Add a GStreamer backend (if you prefer gst pipelines instead of ffmpeg).
- Add metrics (e.g., Prometheus) to measure RTT, CPU and memory per process.
- Add concurrency control and backoff when servers return errors.
