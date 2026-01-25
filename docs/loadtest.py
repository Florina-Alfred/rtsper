#!/usr/bin/env python3
"""
Small helper that uses the system ffmpeg and ffplay to publish a local
video file to an RTSP topic and (optionally) play it back. Intended as a
minimal starting point for building a loadtest script.

Usage examples:
  ./docs/loadtest.py                           # publish docs/test_footage.mp4 to topic1 on localhost
  ./docs/loadtest.py --topic camera42 --play    # also run ffplay to view the stream (subscribe port)

This script is intentionally small and depends on the `ffmpeg` and
`ffplay` executables being on PATH.
"""
from __future__ import annotations

import argparse
import os
import shutil
import signal
import subprocess
import sys
import time


def find_exe(name: str) -> str:
    p = shutil.which(name)
    if not p:
        raise SystemExit(f"{name} not found in PATH; please install ffmpeg")
    return p


def build_ffmpeg_cmd(ffmpeg: str, infile: str, rtsp_url: str, reencode: bool, loop: bool) -> list:
    cmd = [ffmpeg]
    # read input at natural rate
    cmd += ["-re"]
    if loop:
        # loop infinitely
        cmd += ["-stream_loop", "-1"]
    cmd += ["-i", infile]
    if reencode:
        # re-encode to H.264/AAC which is generally accepted by RTSP servers
        cmd += [
            "-c:v",
            "libx264",
            "-preset",
            "veryfast",
            "-tune",
            "zerolatency",
            "-c:a",
            "aac",
        ]
    else:
        # try copy by default (fast, no CPU overhead)
        cmd += ["-c", "copy"]
    cmd += ["-f", "rtsp", rtsp_url]
    return cmd


def build_ffplay_cmd(ffplay: str, rtsp_url: str) -> list:
    # use TCP transport for reliability
    return [ffplay, "-rtsp_transport", "tcp", rtsp_url]


def main() -> None:
    parser = argparse.ArgumentParser(description="Publish docs/test_footage.mp4 with ffmpeg")
    parser.add_argument("--host", default="127.0.0.1", help="RTSP server host to publish to")
    parser.add_argument("--pub-port", type=int, default=9191, help="RTSP publisher port")
    parser.add_argument("--sub-port", type=int, default=9192, help="RTSP subscriber port used by ffplay")
    parser.add_argument("--topic", default="topic1", help="Topic name (path) to publish to")
    parser.add_argument("--file", default=None, help="Video file to stream (defaults to docs/test_footage.mp4)")
    parser.add_argument("--reencode", action="store_true", help="Force re-encoding (libx264/aac) instead of -c copy")
    parser.add_argument("--no-loop", dest="loop", action="store_false", help="Do not loop the input file")
    parser.add_argument("--play", action="store_true", help="Also start ffplay to subscribe and view the stream")
    args = parser.parse_args()

    script_dir = os.path.dirname(os.path.realpath(__file__))
    default_file = os.path.join(script_dir, "test_footage.mp4")
    infile = args.file or default_file
    if not os.path.exists(infile):
        print(f"input file not found: {infile}")
        sys.exit(2)

    ffmpeg = find_exe("ffmpeg")
    ffplay = None
    if args.play:
        ffplay = find_exe("ffplay")

    pub_url = f"rtsp://{args.host}:{args.pub_port}/{args.topic}"
    sub_url = f"rtsp://{args.host}:{args.sub_port}/{args.topic}"

    ffmpeg_cmd = build_ffmpeg_cmd(ffmpeg, infile, pub_url, args.reencode, args.loop)

    print("Starting publisher:")
    print(" ", " ".join(ffmpeg_cmd))

    pub_proc = subprocess.Popen(ffmpeg_cmd)

    play_proc = None
    try:
        if args.play and ffplay:
            # wait briefly so the publisher has time to announce
            time.sleep(1.0)
            ffplay_cmd = build_ffplay_cmd(ffplay, sub_url)
            print("Starting player:")
            print(" ", " ".join(ffplay_cmd))
            play_proc = subprocess.Popen(ffplay_cmd)

        # wait for publisher to exit or until interrupted
        while True:
            ret = pub_proc.poll()
            if ret is not None:
                print(f"ffmpeg exited with code {ret}")
                break
            time.sleep(0.5)

    except KeyboardInterrupt:
        print("Interrupted, terminating child processes...")
    finally:
        for p in (play_proc, pub_proc):
            if p is None:
                continue
            try:
                p.terminate()
            except Exception:
                pass
        # give processes a moment then kill if still running
        time.sleep(0.5)
        for p in (play_proc, pub_proc):
            if p is None:
                continue
            if p.poll() is None:
                try:
                    p.kill()
                except Exception:
                    pass


if __name__ == "__main__":
    main()
