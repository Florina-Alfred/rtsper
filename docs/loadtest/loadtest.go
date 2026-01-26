package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/pion/rtp"
)

// This is a minimal Go-based load test publisher using gortsplib.
// It creates N publisher clients that announce topics and send dummy H264 RTP
// packets periodically. The goal is to simulate many publishers without
// requiring ffmpeg. Receivers can be added later.

func main() {
	var (
		host       = flag.String("host", "127.0.0.1", "RTSP server host")
		pubPort    = flag.Int("pub-port", 9191, "RTSP publisher port")
		streams    = flag.Int("streams", 1, "number of publisher streams to create")
		prefix     = flag.String("prefix", "topic", "topic name prefix")
		interval   = flag.Duration("interval", 50*time.Millisecond, "interval between RTP packets per stream")
		retries    = flag.Int("retries", 5, "number of times to retry StartPublishing before giving up (0 = no retries)")
		retryDelay = flag.Duration("retry-delay", 1*time.Second, "initial retry delay for StartPublishing failures")
		// ffmpeg publishing options
		useFFmpeg = flag.Bool("use-ffmpeg", true, "Use system ffmpeg to publish docs/test_footage.mp4 instead of synthetic RTP")
		infile    = flag.String("infile", "../test_footage.mp4", "Input file to publish (relative to docs/loadtest/) ")
		ffmpegBin = flag.String("ffmpeg", "ffmpeg", "Path to ffmpeg binary")
		staggerMs = flag.Int("stagger-ms", 200, "milliseconds to stagger starts when spawning ffmpeg publishers")
	)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	if *useFFmpeg {
		// spawn ffmpeg processes (one per stream). If infile is missing,
		// fall back to native synthetic RTP publishers.
		cmds := make([]*os.Process, 0, *streams)
		infileExists := false
		if _, err := os.Stat(*infile); err == nil {
			infileExists = true
		}

		for i := 1; i <= *streams; i++ {
			topic := fmt.Sprintf("%s%d", *prefix, i)
			url := fmt.Sprintf("rtsp://%s:%d/%s", *host, *pubPort, topic)

			if infileExists {
				// try ffmpeg with copy, fallback to re-encode on ANNOUNCE 400
				p := spawnFFmpegWithFallback(ctx, *ffmpegBin, *infile, url, *staggerMs)
				if p != nil {
					log.Printf("[%s] started ffmpeg pid=%d -> %s", topic, p.Pid, url)
					cmds = append(cmds, p)
					time.Sleep(time.Duration(*staggerMs) * time.Millisecond)
					continue
				}
				// otherwise fallback to native publisher
				log.Printf("[%s] falling back to native RTP publisher", topic)
			}

			// fallback: native synthetic RTP publisher
			wg.Add(1)
			go func(t string, id int) {
				defer wg.Done()
				runPublisher(ctx, *host, *pubPort, t, id, *interval, *retries, *retryDelay)
			}(topic, i)
			time.Sleep(20 * time.Millisecond)
		}

		// wait for signal
		<-sig
		log.Println("shutdown requested, stopping ffmpeg publishers")
		cancel()
		// give processes a moment to exit
		time.Sleep(500 * time.Millisecond)
		// kill any remaining
		for _, p := range cmds {
			if p == nil {
				continue
			}
			_ = p.Kill()
		}
		return
	}

	for i := 1; i <= *streams; i++ {
		wg.Add(1)
		topic := fmt.Sprintf("%s%d", *prefix, i)
		go func(t string, id int) {
			defer wg.Done()
			runPublisher(ctx, *host, *pubPort, t, id, *interval, *retries, *retryDelay)
		}(topic, i)
		// small stagger to avoid bursting
		time.Sleep(20 * time.Millisecond)
	}

	// wait for signal
	<-sig
	log.Println("shutdown requested")
	cancel()
	// wait for publishers to finish
	wg.Wait()
}

func runPublisher(ctx context.Context, host string, port int, topic string, id int, interval time.Duration, retries int, retryDelay time.Duration) {
	url := fmt.Sprintf("rtsp://%s:%d/%s", host, port, topic)
	// create a simple H264 track (payload type 96)
	track := &gortsplib.TrackH264{PayloadType: 96, PacketizationMode: 1}
	tracks := gortsplib.Tracks{track}

	c := gortsplib.Client{Transport: func() *gortsplib.Transport { v := gortsplib.TransportTCP; return &v }()}
	// try to start publishing with retries/backoff
	attempt := 0
	for {
		if err := c.StartPublishing(url, tracks); err != nil {
			attempt++
			if retries > 0 && attempt > retries {
				log.Printf("[%s] start publishing failed after %d attempts: %v", topic, attempt, err)
				return
			}
			log.Printf("[%s] start publishing attempt %d failed: %v; retrying in %s", topic, attempt, err, retryDelay)
			select {
			case <-time.After(retryDelay):
				// exponential backoff
				retryDelay *= 2
				continue
			case <-ctx.Done():
				return
			}
		}
		break
	}
	defer c.Close()

	log.Printf("[%s] publishing to %s", topic, url)

	seq := uint16(0)
	ts := uint32(0)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] stopping publisher", topic)
			return
		case <-ticker.C:
			pkt := &rtp.Packet{
				Header:  rtp.Header{Version: 2, PayloadType: 96, SequenceNumber: seq, Timestamp: ts, SSRC: uint32(id)},
				Payload: []byte{0x00, 0x00, 0x01, 0x09},
			}
			if err := c.WritePacketRTP(0, pkt); err != nil {
				log.Printf("[%s] write rtp error: %v", topic, err)
				return
			}
			seq++
			ts += 3000
		}
	}
}

// spawnFFmpegWithFallback tries to start ffmpeg to publish a file to the
// provided rtsp url. It first attempts a copy (-c copy) and if the ffmpeg
// stderr indicates a server rejection that likely requires re-encoding
// (e.g. ANNOUNCE 400), it will terminate that ffmpeg and restart with
// libx264/aac re-encode. Returns the started process or nil on error.
func spawnFFmpegWithFallback(ctx context.Context, ffmpegBin, infile, url string, staggerMs int) *os.Process {
	// try copy first
	args := []string{"-re", "-stream_loop", "-1", "-i", infile, "-c", "copy", "-f", "rtsp", url}
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	// capture stderr to inspect for errors
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil
	}
	if err := cmd.Start(); err != nil {
		return nil
	}

	// read a small window of stderr to see if it fails immediately
	r := bufio.NewReader(stderr)
	failed := false
	done := make(chan struct{})
	go func() {
		defer close(done)
		// read up to first 4KB or until newline errors
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		s := string(buf[:n])
		// simple heuristic checks
		if strings.Contains(s, "400 Bad Request") || strings.Contains(s, "Error opening input") || strings.Contains(s, "Could not write header") {
			failed = true
		}
	}()

	// wait briefly
	select {
	case <-done:
	case <-time.After(time.Duration(staggerMs) * time.Millisecond):
	}

	if !failed {
		return cmd.Process
	}

	// otherwise kill and restart with re-encode
	_ = cmd.Process.Kill()
	// build re-encode args
	reArgs := []string{"-re", "-stream_loop", "-1", "-i", infile, "-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", "-f", "rtsp", url}
	cmd2 := exec.CommandContext(ctx, ffmpegBin, reArgs...)
	if err := cmd2.Start(); err != nil {
		return nil
	}
	return cmd2.Process
}
