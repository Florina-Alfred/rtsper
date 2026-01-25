package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Simplified loadtest: start publishers and subscribers using ffmpeg.
// This version focuses on clarity: it starts processes, writes simple per-stream
// logs, and performs a basic graceful shutdown on SIGINT/SIGTERM.

func main() {
	count := flag.Int("count", 100, "number of topics to create")
	filePath := flag.String("file", "docs/test_footage.mp4", "path to source media file")
	pubAddr := flag.String("publish", "localhost:9191", "publish server host:port")
	subAddr := flag.String("subscribe", "localhost:9192", "subscribe server host:port")
	delay := flag.Duration("delay", 50*time.Millisecond, "delay between starting each pair")
	outDir := flag.String("out", "loadtest-logs", "directory to write per-stream logs")
	duration := flag.Duration("duration", 0, "optional run duration; 0 = until interrupted")
	flag.Parse()

	if *count <= 0 {
		log.Fatalf("invalid count: %d", *count)
	}

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Fatalf("ffmpeg not found in PATH: %v", err)
	}

	// Find input file by checking a few likely paths (supports running from
	// repo root or docs/loadtest).
	*filePath = findFile(*filePath)
	log.Printf("using input file: %s", *filePath)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("cannot create out dir: %v", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	var procs []*exec.Cmd
	var mu sync.Mutex
	var started int32
	var failed int32

	start := time.Now()
	for i := 1; i <= *count; i++ {
		topic := "topic" + strconv.Itoa(i)

		// Publisher
		pubURL := fmt.Sprintf("rtsp://%s/%s", *pubAddr, topic)
		pubLog := filepath.Join(*outDir, "pub_"+topic+".log")
		pubCmd := exec.Command(ffmpeg,
			"-re", "-stream_loop", "-1", "-i", *filePath,
			"-c", "copy", "-f", "rtsp", "-rtsp_transport", "tcp", pubURL,
		)
		pubF, _ := os.Create(pubLog)
		pubCmd.Stdout = pubF
		pubCmd.Stderr = pubF

		// Subscriber
		subURL := fmt.Sprintf("rtsp://%s/%s", *subAddr, topic)
		subLog := filepath.Join(*outDir, "sub_"+topic+".log")
		subCmd := exec.Command(ffmpeg, "-rtsp_transport", "tcp", "-i", subURL, "-f", "null", "-")
		subF, _ := os.Create(subLog)
		subCmd.Stdout = subF
		subCmd.Stderr = subF

		if err := pubCmd.Start(); err != nil {
			log.Printf("publisher %s start failed: %v", topic, err)
			atomic.AddInt32(&failed, 1)
			pubF.Close()
			subF.Close()
			continue
		}

		if err := subCmd.Start(); err != nil {
			log.Printf("subscriber %s start failed: %v", topic, err)
			atomic.AddInt32(&failed, 1)
			_ = pubCmd.Process.Kill()
			pubF.Close()
			subF.Close()
			continue
		}

		mu.Lock()
		procs = append(procs, pubCmd, subCmd)
		mu.Unlock()

		atomic.AddInt32(&started, 2)
		wg.Add(2)
		go waitAndClose(pubCmd, pubF, &wg)
		go waitAndClose(subCmd, subF, &wg)

		time.Sleep(*delay)

		// If duration is set and elapsed, stop starting new pairs
		if *duration > 0 && time.Since(start) > *duration {
			break
		}
	}

	log.Printf("started %d processes, %d failures", started, failed)

	// Wait for signal or until all processes exit
	select {
	case <-sigs:
		log.Printf("signal received: shutting down")
	default:
		// continue to wait for wg below
	}

	// Graceful shutdown: give processes a short time, then kill remaining
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("all child processes exited")
	case <-time.After(5 * time.Second):
		log.Printf("timeout; killing remaining processes")
		mu.Lock()
		for _, c := range procs {
			if c.Process != nil {
				_ = c.Process.Kill()
			}
		}
		mu.Unlock()
	}

	log.Printf("finished: started=%d failed=%d", started, failed)
}

func waitAndClose(cmd *exec.Cmd, f *os.File, wg *sync.WaitGroup) {
	defer wg.Done()
	err := cmd.Wait()
	if err != nil {
		log.Printf("process %s exited: %v", cmd.String(), err)
	}
	f.Close()
}

// findFile looks for path in current and parent directories and returns the first match.
func findFile(p string) string {
	if _, err := os.Stat(p); err == nil {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	cwd, _ := os.Getwd()
	cur := cwd
	for i := 0; i < 6; i++ {
		cand := filepath.Join(cur, p)
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		cand = filepath.Join(cur, filepath.Base(p))
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	log.Fatalf("input file not found: %s", p)
	return p
}
