#!/bin/sh
# Simple RTSP -> HLS proxy using ffmpeg.
# Usage: ./rtsp-to-hls.sh rtsp://<host>:<port>/<topic> /path/to/output_dir

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <rtsp-url> <output-dir>"
  exit 1
fi

RTSP_URL="$1"
OUT_DIR="$2"
PORTAL_PORT=8088

mkdir -p "$OUT_DIR"
# Run ffmpeg to produce HLS (low-latency-ish, small segment size)
ffmpeg -y -rtsp_transport tcp -i "$RTSP_URL" -c:v copy -c:a copy -f hls -hls_time 1 -hls_list_size 5 -hls_flags delete_segments "$OUT_DIR/index.m3u8" &
FFMPEG_PID=$!

# Serve the HLS directory via a simple Python HTTP server (for demo only)
cd "$OUT_DIR"
python3 -m http.server $PORTAL_PORT &
PY_PID=$!

echo "Started ffmpeg (PID=$FFMPEG_PID) -> producing HLS at http://localhost:$PORTAL_PORT/index.m3u8"

echo "To stop: kill $FFMPEG_PID $PY_PID"

wait $FFMPEG_PID
