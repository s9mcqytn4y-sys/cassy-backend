package redispool

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPoolSize = 8
	dialTimeout     = 120 * time.Millisecond
	cmdTimeout      = 150 * time.Millisecond
)

// Pool maintains a bounded pool of lightweight RESP TCP connections without external dependencies.
type Pool struct {
	addr   string
	mu     sync.Mutex
	closed bool
	idle   chan net.Conn
}

// New creates a Pool for addr (host:port). Returns nil if addr is empty.
func New(addr string) *Pool {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return nil
	}
	return &Pool{
		addr: trimmed,
		idle: make(chan net.Conn, defaultPoolSize),
	}
}

func (p *Pool) acquire() (net.Conn, bool, error) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil, false, net.ErrClosed
	}
	select {
	case conn := <-p.idle:
		return conn, true, nil
	default:
		conn, err := net.DialTimeout("tcp", p.addr, dialTimeout)
		return conn, false, err
	}
}

func (p *Pool) release(conn net.Conn, ok bool) {
	if conn == nil {
		return
	}
	if !ok {
		_ = conn.Close()
		return
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		_ = conn.Close()
		return
	}
	select {
	case p.idle <- conn:
	default:
		_ = conn.Close()
	}
}

// Close closes all idle pooled sockets.
func (p *Pool) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	close(p.idle)
	for conn := range p.idle {
		_ = conn.Close()
	}
}

// Encode encodes one RESP array command.
func Encode(args ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return b.String()
}

// Get fetches a string key from Redis. Returns ("", false) on miss or error.
func (p *Pool) Get(key string) (string, bool) {
	if p == nil || key == "" {
		return "", false
	}
	for attempt := 0; attempt < 2; attempt++ {
		conn, reused, err := p.acquire()
		if err != nil {
			return "", false
		}
		_ = conn.SetDeadline(time.Now().Add(cmdTimeout))
		if _, err := conn.Write([]byte(Encode("GET", key))); err != nil {
			p.release(conn, false)
			if reused {
				continue
			}
			return "", false
		}
		r := bufio.NewReader(conn)
		hdr, err := r.ReadString('\n')
		if err != nil {
			p.release(conn, false)
			if reused {
				continue
			}
			return "", false
		}
		if strings.HasPrefix(hdr, "$-1") {
			p.release(conn, true)
			return "", false
		}
		if !strings.HasPrefix(hdr, "$") {
			p.release(conn, false)
			return "", false
		}
		length, err := strconv.Atoi(strings.TrimSpace(hdr[1:]))
		if err != nil || length < 0 {
			p.release(conn, false)
			return "", false
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			p.release(conn, false)
			return "", false
		}
		p.release(conn, true)
		return string(buf[:length]), true
	}
	return "", false
}

// SetEX writes a key with integer second TTL cleanly on a pooled socket.
func (p *Pool) SetEX(key string, ttlSeconds int64, val string) bool {
	if p == nil || key == "" || ttlSeconds <= 0 {
		return false
	}
	return p.ExecStatus(Encode("SETEX", key, strconv.FormatInt(ttlSeconds, 10), val))
}

// Del deletes one or more keys.
func (p *Pool) Del(keys ...string) bool {
	if p == nil || len(keys) == 0 {
		return false
	}
	args := make([]string, 0, len(keys)+1)
	args = append(args, "DEL")
	args = append(args, keys...)
	return p.ExecStatus(Encode(args...))
}

// ExecStatus writes raw RESP payload and verifies one non-error reply line, discarding stale sockets once.
func (p *Pool) ExecStatus(raw string) bool {
	if p == nil || raw == "" {
		return false
	}
	for attempt := 0; attempt < 2; attempt++ {
		conn, reused, err := p.acquire()
		if err != nil {
			return false
		}
		_ = conn.SetDeadline(time.Now().Add(cmdTimeout))
		if _, err := conn.Write([]byte(raw)); err != nil {
			p.release(conn, false)
			if reused {
				continue
			}
			return false
		}
		r := bufio.NewReader(conn)
		line, err := r.ReadString('\n')
		if err != nil || strings.HasPrefix(line, "-") {
			p.release(conn, false)
			if err != nil && reused {
				continue
			}
			return false
		}
		p.release(conn, true)
		return true
	}
	return false
}

// Pipeline executes n RESP commands expecting exactly n simple/integer replies and returns lines.
func (p *Pool) Pipeline(raw string, replies int) ([]string, bool) {
	if p == nil || raw == "" || replies <= 0 {
		return nil, false
	}
	for attempt := 0; attempt < 2; attempt++ {
		conn, reused, err := p.acquire()
		if err != nil {
			return nil, false
		}
		_ = conn.SetDeadline(time.Now().Add(cmdTimeout))
		if _, err := conn.Write([]byte(raw)); err != nil {
			p.release(conn, false)
			if reused {
				continue
			}
			return nil, false
		}
		r := bufio.NewReader(conn)
		out := make([]string, 0, replies)
		broken := false
		for i := 0; i < replies; i++ {
			line, err := r.ReadString('\n')
			if err != nil || strings.HasPrefix(line, "-") {
				broken = true
				break
			}
			out = append(out, strings.TrimSpace(line))
		}
		if broken {
			p.release(conn, false)
			if reused {
				continue
			}
			return nil, false
		}
		p.release(conn, true)
		return out, true
	}
	return nil, false
}
