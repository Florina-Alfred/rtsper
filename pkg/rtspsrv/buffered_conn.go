package rtspsrv

import (
	"net"
	"time"
)

// bufferedConn wraps an existing net.Conn and provides initial bytes that were
// already read from the connection so the upper layers can consume them.
type bufferedConn struct {
	net.Conn
	buf []byte
}

func (b *bufferedConn) Read(p []byte) (int, error) {
	if len(b.buf) > 0 {
		n := copy(p, b.buf)
		b.buf = b.buf[n:]
		return n, nil
	}
	return b.Conn.Read(p)
}

// ensure Close delegates
func (b *bufferedConn) Close() error                       { return b.Conn.Close() }
func (b *bufferedConn) Write(p []byte) (int, error)        { return b.Conn.Write(p) }
func (b *bufferedConn) LocalAddr() net.Addr                { return b.Conn.LocalAddr() }
func (b *bufferedConn) RemoteAddr() net.Addr               { return b.Conn.RemoteAddr() }
func (b *bufferedConn) SetDeadline(t time.Time) error      { return b.Conn.SetDeadline(t) }
func (b *bufferedConn) SetReadDeadline(t time.Time) error  { return b.Conn.SetReadDeadline(t) }
func (b *bufferedConn) SetWriteDeadline(t time.Time) error { return b.Conn.SetWriteDeadline(t) }
