package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Loadtest tool (duplicate source in docs for convenience).
// See docs/loadtest/README.md for usage.

func main() {
	var (
		count      = flag.Int("count", 100, "number of topics to create")
		filePath   = flag.String("file", "docs/test_footage.mp4", "path to source media file")
		pubAddr    = flag.String("publish", "localhost:9191", "publish server host:port (no scheme)")
		subAddr    = flag.String("subscribe", "localhost:9192", "subscribe server host:port (no scheme)")
		startDelay = flag.Duration("delay", 50*time.Millisecond, "delay between starting each pair")
		outDir     = flag.String("out", "loadtest-logs", "directory to write per-stream logs")
		duration   = flag.Duration("duration", 0, "duration to run the test; 0 means run until interrupted")
	)
	flag.Parse()

	if *count <= 0 {
		log.Fatalf("invalid count: %d", *count)
	}

	// Ensure ffmpeg exists
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Fatalf("ffmpeg not found in PATH: %v", err)
	}

	// Resolve input file path by walking up from the current working
	// directory and testing several candidate locations. This lets the
	// program be run from the repo root or from docs/loadtest.
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("cannot get working dir: %v", err)
	}

	tried := []string{}
	found := ""

	// Also consider absolute/relative as provided first
	candidates := []string{*filePath}
	if abs, err := filepath.Abs(*filePath); err == nil {
		candidates = append(candidates, abs)
	}

	// Walk up a few directory levels and try both the provided path and the basename
	cur := cwd
	for i := 0; i < 6; i++ {
		candidates = append(candidates, filepath.Join(cur, *filePath))
		candidates = append(candidates, filepath.Join(cur, filepath.Base(*filePath)))
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	for _, c := range candidates {
		if c == "" {
			continue
		}
		tried = append(tried, c)
		if _, err := os.Stat(c); err == nil {
			found = c
			break
		}
	}

	if found == "" {
		log.Fatalf("input file not found (%s); tried: %v", *filePath, tried)
	}
	*filePath = found
	log.Printf("using input file: %s", *filePath)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("cannot create out dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	procs := make([]*exec.Cmd, 0, *count*2)
	mu := sync.Mutex{}

	startPair := func(i int) {
		topic := "topic" + strconv.Itoa(i)

		// Publisher: loop the input file and push to publish endpoint
		pubURL := fmt.Sprintf("rtsp://%s/%s", *pubAddr, topic)
		pubLog := filepath.Join(*outDir, fmt.Sprintf("pub_%s.log", topic))
		pubCmd := exec.CommandContext(ctx, ffmpegPath,
			"-re",
			"-stream_loop", "-1",
			"-i", *filePath,
			"-c", "copy",
			"-f", "rtsp",
			"-rtsp_transport", "tcp",
			pubURL,
		)
		pubF, _ := os.Create(pubLog)
		pubCmd.Stdout = pubF
		pubCmd.Stderr = pubF

		// Subscriber: pull from subscribe endpoint and discard
		subURL := fmt.Sprintf("rtsp://%s/%s", *subAddr, topic)
		subLog := filepath.Join(*outDir, fmt.Sprintf("sub_%s.log", topic))
		subCmd := exec.CommandContext(ctx, ffmpegPath,
			"-rtsp_transport", "tcp",
			"-i", subURL,
			"-f", "null",
			"-",
		)
		subF, _ := os.Create(subLog)
		subCmd.Stdout = subF
		subCmd.Stderr = subF

		// Start publisher
		if err := pubCmd.Start(); err != nil {
			log.Printf("failed to start publisher for %s: %v", topic, err)
			return
		}

		// Start subscriber
		if err := subCmd.Start(); err != nil {
			log.Printf("failed to start subscriber for %s: %v", topic, err)
			// try to kill publisher we just started
			_ = pubCmd.Process.Kill()
			return
		}

		mu.Lock()
		procs = append(procs, pubCmd, subCmd)
		mu.Unlock()

		wg.Add(2)
		go func(cmd *exec.Cmd, f *os.File) {
			defer wg.Done()
			err := cmd.Wait()
			if err != nil {
				log.Printf("process exited with error: %v", err)
			}
			f.Close()
		}(pubCmd, pubF)

		go func(cmd *exec.Cmd, f *os.File) {
			defer wg.Done()
			err := cmd.Wait()
			if err != nil {
				log.Printf("process exited with error: %v", err)
			}
			f.Close()
		}(subCmd, subF)
	}

	log.Printf("starting %d publisher+subscriber pairs (publishing to %s, subscribing from %s)", *count, *pubAddr, *subAddr)

	// Start pairs with small delay to avoid hammering scheduler instantly
	for i := 1; i <= *count; i++ {
		select {
		case <-ctx.Done():
			break
		default:
		}
		startPair(i)
		time.Sleep(*startDelay)
	}

	// Optionally stop after duration
	if *duration > 0 {
		go func() {
			select {
			case <-time.After(*duration):
				log.Printf("duration elapsed: cancelling")
				cancel()
			case <-sigs:
				cancel()
			}
		}()
	}

	// Wait for signal
	select {
	case <-sigs:
		log.Printf("signal received: shutting down")
		cancel()
	case <-ctx.Done():
	}

	// Give processes a moment to exit gracefully, then kill any remaining
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("all processes exited")
	case <-time.After(5 * time.Second):
		log.Printf("timeout waiting for processes, killing remaining")
		mu.Lock()
		for _, cmd := range procs {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
		mu.Unlock()
	}

	log.Printf("loadtest finished")
}
