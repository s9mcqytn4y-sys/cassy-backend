package redispool

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

func TestPool_GetSetEXAndPipeline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				r := bufio.NewReader(c)
				store := map[string]string{}
				for {
					hdr, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if !strings.HasPrefix(hdr, "*") {
						return
					}
					var args []string
					n := 0
					for _, ch := range strings.TrimSpace(hdr[1:]) {
						n = n*10 + int(ch-'0')
					}
					for i := 0; i < n; i++ {
						_, _ = r.ReadString('\n')
						arg, _ := r.ReadString('\n')
						args = append(args, strings.TrimRight(arg, "\r\n"))
					}
					switch strings.ToUpper(args[0]) {
					case "SETEX":
						store[args[1]] = args[3]
						_, _ = c.Write([]byte("+OK\r\n"))
					case "GET":
						v, ok := store[args[1]]
						if !ok {
							_, _ = c.Write([]byte("$-1\r\n"))
						} else {
							_, _ = c.Write([]byte("$" + itoa(len(v)) + "\r\n" + v + "\r\n"))
						}
					case "ZREMRANGEBYSCORE", "ZCARD", "ZADD", "PEXPIRE", "DEL":
						_, _ = c.Write([]byte(":1\r\n"))
					}
				}
			}(conn)
		}
	}()

	p := New(ln.Addr().String())
	defer p.Close()

	if !p.SetEX("k1", 30, "hello") {
		t.Fatal("SetEX failed")
	}
	if v, ok := p.Get("k1"); !ok || v != "hello" {
		t.Fatalf("Get mismatch: %q %v", v, ok)
	}
	if v, ok := p.Get("missing"); ok || v != "" {
		t.Fatalf("expected miss, got %q %v", v, ok)
	}
	lines, ok := p.Pipeline(Encode("ZREMRANGEBYSCORE", "rl", "-inf", "0")+Encode("ZCARD", "rl"), 2)
	if !ok || len(lines) != 2 {
		t.Fatalf("Pipeline failed: %v %v", lines, ok)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [16]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
