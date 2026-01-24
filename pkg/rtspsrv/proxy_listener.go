package rtspsrv

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	plog "redalf.de/rtsper/pkg/log"
	"redalf.de/rtsper/pkg/metrics"
)

type proxyListener struct {
	ln          net.Listener
	server      *Server
	isPublisher bool
}

func (p *proxyListener) Accept() (net.Conn, error) {
	for {
		nconn, err := p.ln.Accept()
		if err != nil {
			return nil, err
		}

		// peek initial request bytes (up to 8KB or until blank line)
		nconn.SetReadDeadline(time.Now().Add(2 * time.Second))
		reader := bufio.NewReader(nconn)
		var buf bytes.Buffer
		// read until double CRLF or up to limit
		for buf.Len() < 8192 {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				break
			}
			buf.Write(line)
			if bytes.HasSuffix(buf.Bytes(), []byte("\r\n\r\n")) {
				break
			}
		}
		// clear deadline
		nconn.SetReadDeadline(time.Time{})

		// parse first request line to extract path
		b := buf.Bytes()
		// fallback: if empty, just return connection to be handled locally
		if len(b) == 0 {
			return &bufferedConn{Conn: nconn, buf: buf.Bytes()}, nil
		}
		// first line
		firstLineEnd := bytes.IndexByte(b, '\n')
		if firstLineEnd < 0 {
			firstLineEnd = len(b)
		}
		firstLine := strings.TrimSpace(string(b[:firstLineEnd]))
		parts := strings.SplitN(firstLine, " ", 3)
		path := ""
		if len(parts) >= 2 {
			// parts[1] may be absolute URL or absolute path
			u := parts[1]
			if strings.HasPrefix(u, "rtsp://") {
				if parsed, err := url.Parse(u); err == nil {
					path = parsed.Path
				}
			} else {
				// may be "/topic"
				path = u
			}
		}
		topic := strings.TrimPrefix(path, "/")

		// determine owner
		owner := ""
		if p.server != nil && p.server.cluster != nil {
			owner = p.server.cluster.Owner(topic)
		}
		// if owner is self or cluster not configured, hand over connection to local server
		if owner == "" || p.server.cluster == nil || p.server.cluster.IsSelf(owner) {
			return &bufferedConn{Conn: nconn, buf: buf.Bytes()}, nil
		}

		// else proxy to owner
		port := p.server.pubPort
		if !p.isPublisher {
			port = p.server.subPort
		}
		targetAddr := fmt.Sprintf("%s:%d", owner, port)
		dialer := net.Dialer{Timeout: p.server.proxyDialTimeout}
		targetConn, err := dialer.Dial("tcp", targetAddr)
		if err != nil {
			plog.Info("failed to dial owner %s: %v", targetAddr, err)
			metrics.IncForwardFailed()
			// respond with RTSP 503 Service Unavailable
			msg := "RTSP/1.0 503 Service Unavailable\r\nServer: rtsper-proxy\r\n\r\n"
			nconn.Write([]byte(msg))
			nconn.Close()
			continue // accept next connection
		}

		// write buffered bytes to target then start bidirectional copy
		if len(b) > 0 {
			targetConn.Write(b)
		}
		metrics.IncForwardedConnections()

		// copy both ways
		go func() {
			n, _ := io.Copy(targetConn, nconn)
			metrics.AddForwardedBytes(n)
			targetConn.Close()
			nconn.Close()
		}()
		go func() {
			n, _ := io.Copy(nconn, targetConn)
			metrics.AddForwardedBytes(n)
			targetConn.Close()
			nconn.Close()
		}()
		// continue loop to accept next connection
	}
}

func (p *proxyListener) Close() error   { return p.ln.Close() }
func (p *proxyListener) Addr() net.Addr { return p.ln.Addr() }
