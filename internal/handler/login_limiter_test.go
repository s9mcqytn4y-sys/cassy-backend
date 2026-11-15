package handler

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoginFailureLimiterSlidingWindow(t *testing.T) {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	l := newLoginFailureLimiter(3, 15*time.Minute)
	l.redisAddr = ""
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if l.Blocked("203.0.113.7") {
			t.Fatalf("blocked after %d failures", i)
		}
		l.RecordFailure("203.0.113.7")
	}
	if !l.Blocked("203.0.113.7") {
		t.Fatal("expected block at the limit")
	}
	if l.Blocked("198.51.100.9") {
		t.Fatal("limit must be per IP")
	}
	now = now.Add(15*time.Minute + time.Second)
	if l.Blocked("203.0.113.7") {
		t.Fatal("failures outside the window must expire")
	}
	if len(l.failures) != 0 {
		t.Fatalf("expired IPs must be dropped, have %d", len(l.failures))
	}
}

func TestLoginFailureLimiterRedisRESP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var mu sync.Mutex
	counts := map[string]int{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if !strings.HasPrefix(line, "*") {
						continue
					}
					var argc int
					fmt.Sscanf(strings.TrimSpace(line), "*%d", &argc)
					args := make([]string, 0, argc)
					for i := 0; i < argc; i++ {
						_, _ = r.ReadString('\n')
						val, _ := r.ReadString('\n')
						args = append(args, strings.TrimSpace(val))
					}
					if len(args) == 0 {
						continue
					}
					mu.Lock()
					switch strings.ToUpper(args[0]) {
					case "ZREMRANGEBYSCORE":
						_, _ = c.Write([]byte(":0\r\n"))
					case "ZADD":
						counts[args[1]]++
						_, _ = c.Write([]byte(":1\r\n"))
					case "PEXPIRE":
						_, _ = c.Write([]byte(":1\r\n"))
					case "ZCARD":
						_, _ = fmt.Fprintf(c, ":%d\r\n", counts[args[1]])
					}
					mu.Unlock()
				}
			}(conn)
		}
	}()

	l := newLoginFailureLimiter(2, 10*time.Minute)
	l.redisAddr = ln.Addr().String()
	if l.Blocked("192.0.2.10") {
		t.Fatal("must not be blocked initially")
	}
	l.RecordFailure("192.0.2.10")
	l.RecordFailure("192.0.2.10")
	if !l.Blocked("192.0.2.10") {
		t.Fatal("expected Redis ZSET count to block at limit 2")
	}
}
