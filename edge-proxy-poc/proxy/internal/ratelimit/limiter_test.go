package ratelimit

import (
	"net/http"
	"testing"

	"github.com/openflare/edge-proxy/internal/config"
)

func TestTokenBucketAllow(t *testing.T) {
	cfg := &config.RateLimitConfig{
		Rules: []config.RateLimitRule{
			{ID: "test", Key: "ip", Rate: 10, Burst: 20, Enabled: true},
		},
	}

	limiter := NewLimiter(cfg, nil)

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	allowed, _, _ := limiter.Allow(req)
	if !allowed {
		t.Error("first request should be allowed")
	}
}

func TestTokenBucketDeny(t *testing.T) {
	cfg := &config.RateLimitConfig{
		Rules: []config.RateLimitRule{
			{ID: "test", Key: "ip", Rate: 1, Burst: 2, Enabled: true},
		},
	}

	limiter := NewLimiter(cfg, nil)

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// Send burst
	for i := 0; i < 2; i++ {
		limiter.Allow(req)
	}

	// Next should be denied
	allowed, ruleID, _ := limiter.Allow(req)
	if allowed {
		t.Error("request should be denied after exceeding burst")
	}
	if ruleID != "test" {
		t.Errorf("expected rule_id 'test', got '%s'", ruleID)
	}
}

func TestBurstCapacity(t *testing.T) {
	cfg := &config.RateLimitConfig{
		Rules: []config.RateLimitRule{
			{ID: "test", Key: "ip", Rate: 1, Burst: 5, Enabled: true},
		},
	}

	limiter := NewLimiter(cfg, nil)

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	// Should allow burst
	allowed := 0
	for i := 0; i < 10; i++ {
		ok, _, _ := limiter.Allow(req)
		if ok {
			allowed++
		}
	}

	if allowed < 3 || allowed > 6 {
		t.Errorf("expected around 5 allowed in burst, got %d", allowed)
	}
}

func TestPathPrefixMatching(t *testing.T) {
	cfg := &config.RateLimitConfig{
		Rules: []config.RateLimitRule{
			{ID: "api", Key: "ip", PathPrefix: "/api/", Rate: 1, Burst: 1, Enabled: true},
		},
	}

	limiter := NewLimiter(cfg, nil)

	// API request should be rate limited
	apiReq, _ := http.NewRequest("GET", "http://example.com/api/users", nil)
	apiReq.RemoteAddr = "10.0.0.1:12345"

	// Non-API request should not trigger API rule
	otherReq, _ := http.NewRequest("GET", "http://example.com/static/app.js", nil)
	otherReq.RemoteAddr = "10.0.0.1:12345"

	// Exhaust API limit
	limiter.Allow(apiReq)

	// API should be denied
	allowed, _, _ := limiter.Allow(apiReq)
	if allowed {
		t.Error("API request should be denied")
	}

	// Non-API should be allowed (no matching rule)
	allowed, _, _ = limiter.Allow(otherReq)
	if !allowed {
		t.Error("non-API request should be allowed")
	}
}

func TestExemption(t *testing.T) {
	cfg := &config.RateLimitConfig{
		Rules: []config.RateLimitRule{
			{ID: "test", Key: "ip", Rate: 1, Burst: 1, Enabled: true},
		},
		Exemptions: config.RateLimitExemptions{
			Paths: []string{"/healthz", "/metrics"},
		},
	}

	limiter := NewLimiter(cfg, nil)

	// Exempt path
	req, _ := http.NewRequest("GET", "http://example.com/healthz", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	for i := 0; i < 10; i++ {
		allowed, _, _ := limiter.Allow(req)
		if !allowed {
			t.Error("exempt path should always be allowed")
		}
	}
}
