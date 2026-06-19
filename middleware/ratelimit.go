package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/chmenegatti/goxpress"
)

// sweepInterval bounds how often idle buckets are evicted and how long an idle
// bucket survives before removal, keeping the per-key map from growing without
// bound under churning client addresses.
const sweepInterval = 10 * time.Minute

// RateLimitConfig configures the RateLimit middleware. Limiting uses a token
// bucket per key: each bucket refills at Rate tokens per second up to Burst,
// and every request consumes one token.
type RateLimitConfig struct {
	// Rate is the sustained number of requests allowed per second per key. It
	// must be positive.
	Rate float64
	// Burst is the maximum number of tokens a bucket holds, i.e. the largest
	// instantaneous burst permitted. Defaults to ceil(Rate) when zero.
	Burst int
	// KeyFunc derives the bucket key from the request. Defaults to the client IP
	// parsed from RemoteAddr.
	KeyFunc func(c *goxpress.Context) string
}

// tokenBucket tracks the available tokens for one key and when they were last
// refilled.
type tokenBucket struct {
	tokens float64
	last   time.Time
}

// rateLimiter holds the per-key buckets behind a mutex.
type rateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*tokenBucket
	rate      float64
	burst     float64
	lastSweep time.Time
}

// allow reports whether a request for key is permitted at now, and when denied
// returns how long until a token is available.
func (l *rateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastSweep) >= sweepInterval {
		l.sweep(now)
		l.lastSweep = now
	}

	b := l.buckets[key]
	if b == nil {
		b = &tokenBucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	} else {
		b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
		b.last = now
	}

	if b.tokens < 1 {
		retry := time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
		return false, retry
	}
	b.tokens--
	return true, 0
}

// sweep removes buckets that have been idle for at least sweepInterval.
func (l *rateLimiter) sweep(now time.Time) {
	for k, b := range l.buckets {
		if now.Sub(b.last) >= sweepInterval {
			delete(l.buckets, k)
		}
	}
}

// RateLimit returns per-IP token-bucket rate-limiting middleware allowing rate
// requests per second.
func RateLimit(rate float64) goxpress.HandlerFunc {
	return RateLimitWithConfig(RateLimitConfig{Rate: rate})
}

// RateLimitWithConfig returns token-bucket rate-limiting middleware using cfg.
// Rejected requests receive a 429 with a Retry-After header.
func RateLimitWithConfig(cfg RateLimitConfig) goxpress.HandlerFunc {
	if cfg.Rate <= 0 {
		panic("goxpress/middleware: RateLimit requires a positive Rate")
	}
	burst := float64(cfg.Burst)
	if burst < 1 {
		burst = math.Max(1, math.Ceil(cfg.Rate))
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = ipKey
	}

	l := &rateLimiter{
		buckets:   make(map[string]*tokenBucket),
		rate:      cfg.Rate,
		burst:     burst,
		lastSweep: time.Now(),
	}

	return func(c *goxpress.Context) error {
		allowed, retry := l.allow(cfg.KeyFunc(c), time.Now())
		if !allowed {
			c.SetHeader("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
			return goxpress.NewHTTPError(http.StatusTooManyRequests)
		}
		return c.Next()
	}
}

// ipKey returns the client IP from RemoteAddr, falling back to the raw value
// when it carries no port.
func ipKey(c *goxpress.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}
