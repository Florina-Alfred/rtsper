package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	pubsubDelay := flag.Duration("pubsub-delay", 200*time.Millisecond, "delay between starting publisher and subscriber for the same topic")
	phase := flag.String("phase", "both", "which phase to run: publish, subscribe, both")
	graceAfterPublish := flag.Duration("grace-after-publish", 2*time.Second, "grace period after publishing before starting subscribers when phase=both")
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

	_ = time.Now()

	// Two-phase mode: optionally start all publishers first, wait, then start subscribers.
	// Prepare command lists for publishers and subscribers.
	type pair struct {
		topic  string
		pubCmd *exec.Cmd
		pubF   *os.File
		subCmd *exec.Cmd
		subF   *os.File
	}

	pairs := make([]*pair, 0, *count)
	for i := 1; i <= *count; i++ {
		topic := "topic" + strconv.Itoa(i)

		pubURL := fmt.Sprintf("rtsp://%s/%s", *pubAddr, topic)
		pubLog := filepath.Join(*outDir, "pub_"+topic+".log")
		pubCmd := exec.Command(ffmpeg,
			"-re", "-stream_loop", "-1", "-i", *filePath,
			"-c", "copy", "-f", "rtsp", "-rtsp_transport", "tcp", pubURL,
		)
		pubF, _ := os.Create(pubLog)

		subURL := fmt.Sprintf("rtsp://%s/%s", *subAddr, topic)
		subLog := filepath.Join(*outDir, "sub_"+topic+".log")
		subCmd := exec.Command(ffmpeg, "-rtsp_transport", "tcp", "-i", subURL, "-f", "null", "-")
		subF, _ := os.Create(subLog)

		pairs = append(pairs, &pair{topic: topic, pubCmd: pubCmd, pubF: pubF, subCmd: subCmd, subF: subF})
	}

	// Helper to start a list of commands with delay and track them
	startList := func(cmds []*exec.Cmd, files []*os.File) {
		for idx, c := range cmds {
			if c == nil {
				continue
			}
			if err := c.Start(); err != nil {
				log.Printf("start failed: %v", err)
				atomic.AddInt32(&failed, 1)
				files[idx].Close()
				continue
			}
			mu.Lock()
			procs = append(procs, c)
			mu.Unlock()
			atomic.AddInt32(&started, 1)
			wg.Add(1)
			go func(cmd *exec.Cmd, f *os.File) {
				defer wg.Done()
				err := cmd.Wait()
				if err != nil {
					log.Printf("process %s exited: %v", cmd.String(), err)
				}
				f.Close()
			}(c, files[idx])
			time.Sleep(*delay)
		}
	}

	// If phase is publish or both, start all publishers first
	if *phase == "publish" || *phase == "both" {
		pubCmds := make([]*exec.Cmd, 0, len(pairs))
		pubFiles := make([]*os.File, 0, len(pairs))
		for _, p := range pairs {
			pubCmds = append(pubCmds, p.pubCmd)
			pubFiles = append(pubFiles, p.pubF)
		}
		log.Printf("starting %d publishers", len(pubCmds))
		startList(pubCmds, pubFiles)
	}

	// If both, wait a grace period before starting subscribers
	if *phase == "both" {
		log.Printf("waiting %s before starting subscribers", pubsubDelay)
		time.Sleep(*graceAfterPublish)
	}

	// If phase is subscribe or both, start subscribers now
	if *phase == "subscribe" || *phase == "both" {
		subCmds := make([]*exec.Cmd, 0, len(pairs))
		subFiles := make([]*os.File, 0, len(pairs))
		for _, p := range pairs {
			subCmds = append(subCmds, p.subCmd)
			subFiles = append(subFiles, p.subF)
		}

		// If duration was set, wait for that duration then proceed to shutdown
		if *duration > 0 {
			log.Printf("running for %s", *duration)
			time.Sleep(*duration)
		}
		log.Printf("starting %d subscribers", len(subCmds))
		startList(subCmds, subFiles)
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

	// Summarize common ffmpeg error lines from logs
	summarizeLogs(*outDir, 10)
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

// summarizeLogs scans log files in outDir for common error lines and prints a small report.
func summarizeLogs(outDir string, top int) {
	files, err := filepath.Glob(filepath.Join(outDir, "*.log"))
	if err != nil || len(files) == 0 {
		log.Printf("no log files found in %s to summarize", outDir)
		return
	}

	counts := map[string]int{}
	totalErrors := 0
	filesScanned := 0

	for _, f := range files {
		fi, err := os.Open(f)
		if err != nil {
			continue
		}
		filesScanned++
		r := bufio.NewReader(fi)
		for {
			line, err := r.ReadString('\n')
			if err != nil && err != io.EOF {
				break
			}
			l := strings.TrimSpace(line)
			low := strings.ToLower(l)
			if strings.Contains(low, "error") || strings.Contains(low, "failed") || strings.Contains(low, "refused") || strings.Contains(low, "not found") {
				// Normalize by removing variable parts (timestamps) - crude but helpful
				// Keep the whole line for now
				counts[l]++
				totalErrors++
			}
			if err == io.EOF {
				break
			}
		}
		fi.Close()
	}

	if totalErrors == 0 {
		log.Printf("no error lines found in %d log files", filesScanned)
		return
	}

	// Sort by count
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(counts))
	for k, v := range counts {
		arr = append(arr, kv{k, v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].v > arr[j].v })

	log.Printf("log summary: scanned=%d total_error_lines=%d unique=%d top=%d", filesScanned, totalErrors, len(counts), top)
	limit := top
	if limit > len(arr) {
		limit = len(arr)
	}
	for i := 0; i < limit; i++ {
		log.Printf("[%d] %d occurrences: %s", i+1, arr[i].v, arr[i].k)
	}
	log.Printf("inspect %s for per-stream details", outDir)
}
