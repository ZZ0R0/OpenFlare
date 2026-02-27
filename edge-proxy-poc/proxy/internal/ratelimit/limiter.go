package ratelimit

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openflare/edge-proxy/internal/config"
	"github.com/openflare/edge-proxy/internal/observability"
	"golang.org/x/time/rate"
)

// Limiter implements token-bucket rate limiting.
type Limiter struct {
	rules      []config.RateLimitRule
	exemptions config.RateLimitExemptions
	limiters   map[string]*rateLimiterEntry
	mu         sync.RWMutex
	metrics    *observability.Metrics
	cleanupInterval time.Duration
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewLimiter creates a new rate limiter from configuration.
func NewLimiter(cfg *config.RateLimitConfig, metrics *observability.Metrics) *Limiter {
	l := &Limiter{
		limiters:        make(map[string]*rateLimiterEntry),
		metrics:         metrics,
		cleanupInterval: 5 * time.Minute,
	}

	if cfg != nil {
		l.rules = cfg.Rules
		l.exemptions = cfg.Exemptions
	}

	// Start cleanup goroutine
	go l.cleanup()

	return l
}

// Allow checks if a request is allowed by any matching rate limit rule.
func (l *Limiter) Allow(r *http.Request) (bool, string, *config.RateLimitRule) {
	clientIP := extractClientIP(r.RemoteAddr)

	// Check exemptions
	if l.isExempt(clientIP, r.URL.Path) {
		return true, "", nil
	}

	// Check each rule
	for i := range l.rules {
		rule := &l.rules[i]
		if !rule.Enabled {
			continue
		}

		// Check if rule path prefix matches
		if rule.PathPrefix != "" && !strings.HasPrefix(r.URL.Path, rule.PathPrefix) {
			continue
		}

		// Build key
		key := l.buildKey(rule, r, clientIP)

		// Get or create limiter
		limiter := l.getLimiter(key, rule)

		if !limiter.Allow() {
			if l.metrics != nil {
				l.metrics.RateLimitTotal.WithLabelValues(rule.ID, "deny").Inc()
			}
			return false, rule.ID, rule
		}

		if l.metrics != nil {
			l.metrics.RateLimitTotal.WithLabelValues(rule.ID, "allow").Inc()
		}
	}

	return true, "", nil
}

// HandleDeny writes a 429 response.
func HandleDeny(w http.ResponseWriter, ruleID string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "rate limit exceeded",
		"rule_id": ruleID,
	})
}

func (l *Limiter) buildKey(rule *config.RateLimitRule, r *http.Request, clientIP string) string {
	switch rule.Key {
	case "ip":
		return fmt.Sprintf("rl:%s:ip:%s", rule.ID, clientIP)
	case "ip_path":
		return fmt.Sprintf("rl:%s:ip_path:%s:%s", rule.ID, clientIP, rule.PathPrefix)
	case "api_key":
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = "anonymous"
		}
		return fmt.Sprintf("rl:%s:api_key:%s", rule.ID, apiKey)
	default:
		return fmt.Sprintf("rl:%s:ip:%s", rule.ID, clientIP)
	}
}

func (l *Limiter) getLimiter(key string, rule *config.RateLimitRule) *rate.Limiter {
	l.mu.RLock()
	entry, ok := l.limiters[key]
	l.mu.RUnlock()

	if ok {
		l.mu.Lock()
		entry.lastSeen = time.Now()
		l.mu.Unlock()
		return entry.limiter
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, ok := l.limiters[key]; ok {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(rate.Limit(rule.Rate), rule.Burst)
	l.limiters[key] = &rateLimiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	return limiter
}

func (l *Limiter) isExempt(clientIP, path string) bool {
	// Check path exemptions
	for _, exemptPath := range l.exemptions.Paths {
		if strings.HasPrefix(path, exemptPath) {
			return true
		}
	}

	// Check CIDR exemptions
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	for _, cidr := range l.exemptions.CIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		threshold := time.Now().Add(-l.cleanupInterval)
		for key, entry := range l.limiters {
			if entry.lastSeen.Before(threshold) {
				delete(l.limiters, key)
			}
		}
		l.mu.Unlock()
	}
}

func extractClientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
