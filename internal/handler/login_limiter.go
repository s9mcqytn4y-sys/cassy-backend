package handler

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// loginFailureLimiter counts failed staff logins or guest actions per trusted client key in a sliding
// window (ADR-003 D3). When REDIS_ADDR is configured, sliding windows are coordinated in Redis ZSETs
// across instances with automatic in-memory fallback when Redis is unset or unreachable.
type loginFailureLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	redisAddr string
	prefix    string
	failures  map[string][]time.Time
	now       func() time.Time
}

func newLoginFailureLimiter(limit int, window time.Duration) *loginFailureLimiter {
	return &loginFailureLimiter{
		limit:     limit,
		window:    window,
		redisAddr: strings.TrimSpace(os.Getenv("REDIS_ADDR")),
		prefix:    fmt.Sprintf("cassy:rl:%d:%d:", limit, int(window.Seconds())),
		failures:  map[string][]time.Time{},
		now:       time.Now,
	}
}

// Blocked reports whether ip already reached the failure limit inside the window.
func (l *loginFailureLimiter) Blocked(ip string) bool {
	now := l.now()
	if l.redisAddr != "" {
		if count, ok := l.redisCount(ip, now); ok {
			return count >= l.limit
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.prune(ip)) >= l.limit
}

func (l *loginFailureLimiter) RecordFailure(ip string) {
	now := l.now()
	if l.redisAddr != "" {
		l.redisRecord(ip, now)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[ip] = append(l.prune(ip), now)
	if len(l.failures) > 10000 {
		l.sweep()
	}
}

func (l *loginFailureLimiter) prune(ip string) []time.Time {
	cutoff := l.now().Add(-l.window)
	kept := l.failures[ip][:0]
	for _, t := range l.failures[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, ip)
		return nil
	}
	l.failures[ip] = kept
	return kept
}

// sweep drops idle IPs so a scan from many addresses cannot grow the map without bound.
func (l *loginFailureLimiter) sweep() {
	for ip := range l.failures {
		l.prune(ip)
	}
}

func (l *loginFailureLimiter) redisCount(ip string, now time.Time) (int, bool) {
	conn, err := net.DialTimeout("tcp", l.redisAddr, 150*time.Millisecond)
	if err != nil {
		return 0, false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(150 * time.Millisecond))
	key := l.prefix + ip
	cutoff := strconv.FormatInt(now.Add(-l.window).UnixNano(), 10)
	cmd := respCmd("ZREMRANGEBYSCORE", key, "-inf", cutoff) + respCmd("ZCARD", key)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return 0, false
	}
	r := bufio.NewReader(conn)
	if _, err := r.ReadString('\n'); err != nil {
		return 0, false
	}
	line, err := r.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, ":") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return 0, false
	}
	return n, true
}

func (l *loginFailureLimiter) redisRecord(ip string, now time.Time) {
	conn, err := net.DialTimeout("tcp", l.redisAddr, 150*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(150 * time.Millisecond))
	key := l.prefix + ip
	score := strconv.FormatInt(now.UnixNano(), 10)
	cutoff := strconv.FormatInt(now.Add(-l.window).UnixNano(), 10)
	ttlMs := strconv.FormatInt(l.window.Milliseconds(), 10)
	cmd := respCmd("ZREMRANGEBYSCORE", key, "-inf", cutoff) +
		respCmd("ZADD", key, score, score) +
		respCmd("PEXPIRE", key, ttlMs)
	_, _ = conn.Write([]byte(cmd))
}

func respCmd(args ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return b.String()
}
