package handler

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/redispool"
)

// loginFailureLimiter counts failed staff logins or guest actions per trusted client key in a sliding
// window (ADR-003 D3). When REDIS_ADDR is configured, sliding windows are coordinated in Redis ZSETs
// across instances with automatic in-memory fallback when Redis is unset or unreachable.
type loginFailureLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	pool     *redispool.Pool
	prefix   string
	failures map[string][]time.Time
	now      func() time.Time
}

func newLoginFailureLimiter(limit int, window time.Duration) *loginFailureLimiter {
	addr := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	return &loginFailureLimiter{
		limit:    limit,
		window:   window,
		pool:     redispool.New(addr),
		prefix:   "cassy:rl:" + strconv.Itoa(limit) + ":" + strconv.Itoa(int(window.Seconds())) + ":",
		failures: map[string][]time.Time{},
		now:      time.Now,
	}
}

// Blocked reports whether ip already reached the failure limit inside the window.
func (l *loginFailureLimiter) Blocked(ip string) bool {
	now := l.now()
	if l.pool != nil {
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
	if l.pool != nil {
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
	if l.pool == nil {
		return 0, false
	}
	key := l.prefix + ip
	cutoff := strconv.FormatInt(now.Add(-l.window).UnixNano(), 10)
	cmd := redispool.Encode("ZREMRANGEBYSCORE", key, "-inf", cutoff) + redispool.Encode("ZCARD", key)
	lines, ok := l.pool.Pipeline(cmd, 2)
	if !ok || len(lines) < 2 || !strings.HasPrefix(lines[1], ":") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(lines[1][1:]))
	if err != nil {
		return 0, false
	}
	return n, true
}

func (l *loginFailureLimiter) redisRecord(ip string, now time.Time) {
	if l.pool == nil {
		return
	}
	key := l.prefix + ip
	score := strconv.FormatInt(now.UnixNano(), 10)
	cutoff := strconv.FormatInt(now.Add(-l.window).UnixNano(), 10)
	ttlMs := strconv.FormatInt(l.window.Milliseconds(), 10)
	cmd := redispool.Encode("ZREMRANGEBYSCORE", key, "-inf", cutoff) +
		redispool.Encode("ZADD", key, score, score) +
		redispool.Encode("PEXPIRE", key, ttlMs)
	_, _ = l.pool.Pipeline(cmd, 3)
}
