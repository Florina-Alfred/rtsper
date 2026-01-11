#!/bin/sh
# Start a simple Python HTTP server to serve the HLS directory.
# Usage: ./rtsp-to-hls-python-server.sh /path/to/hls 8088

if [ "$#" -lt 2 ]; then
  echo "Usage: $0 <hls-dir> <port>"
  exit 1
fi

HLS_DIR="$1"
PORT="$2"

cd "$HLS_DIR" || exit 1
python3 -m http.server "$PORT"
