package cache

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/openflare/edge-proxy/internal/config"
)

// Policy handles cache eligibility decisions.
type Policy struct {
	cfg       config.CacheConfig
	rules     []config.CacheRule
	compiled  []compiledCacheRule
}

type compiledCacheRule struct {
	rule       config.CacheRule
	pathRegex  *regexp.Regexp
}

// NewPolicy creates a new cache policy.
func NewPolicy(cfg config.CacheConfig, rules []config.CacheRule) *Policy {
	p := &Policy{
		cfg:   cfg,
		rules: rules,
	}

	for _, r := range rules {
		cr := compiledCacheRule{rule: r}
		if r.PathRegex != "" {
			cr.pathRegex, _ = regexp.Compile(r.PathRegex)
		}
		p.compiled = append(p.compiled, cr)
	}

	return p
}

// CacheStatus represents the cache decision.
type CacheStatus string

const (
	StatusHIT     CacheStatus = "HIT"
	StatusMISS    CacheStatus = "MISS"
	StatusBYPASS  CacheStatus = "BYPASS"
	StatusSTORE   CacheStatus = "STORE"
	StatusEXPIRED CacheStatus = "EXPIRED"
)

// IsRequestCacheable checks if a request is eligible for caching.
func (p *Policy) IsRequestCacheable(r *http.Request) (bool, string) {
	// Only GET and HEAD
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false, "method_not_cacheable"
	}

	// Bypass on Authorization header
	if p.cfg.BypassOnAuthorization && r.Header.Get("Authorization") != "" {
		return false, "authorization_header"
	}

	// Bypass on Cookie header
	if p.cfg.BypassOnCookie && r.Header.Get("Cookie") != "" {
		return false, "cookie_header"
	}

	// Check cache rules
	for _, cr := range p.compiled {
		if p.matchRule(cr, r) {
			if !cr.rule.Cache.Eligible {
				return false, "rule_" + cr.rule.ID
			}
			// Rule says eligible, continue
			return true, ""
		}
	}

	// Default: eligible (for GET/HEAD without cookies/auth)
	return true, ""
}

// IsResponseStorable checks if a response should be stored in cache.
func (p *Policy) IsResponseStorable(statusCode int, headers http.Header) (bool, string) {
	// Only 2xx responses
	if statusCode < 200 || statusCode >= 300 {
		return false, "non_2xx_status"
	}

	cc := headers.Get("Cache-Control")
	ccLower := strings.ToLower(cc)

	// no-store
	if strings.Contains(ccLower, "no-store") {
		return false, "cache_control_no_store"
	}

	// private
	if strings.Contains(ccLower, "private") {
		return false, "cache_control_private"
	}

	// no-cache (v1: treated as not storable for simplicity)
	if strings.Contains(ccLower, "no-cache") {
		return false, "cache_control_no_cache"
	}

	// Set-Cookie
	if p.cfg.NeverStoreOnSetCookie && headers.Get("Set-Cookie") != "" {
		return false, "set_cookie_header"
	}

	return true, ""
}

// DetermineTTL calculates the TTL for a response.
func (p *Policy) DetermineTTL(reqPath string, headers http.Header) time.Duration {
	// Priority 1: Matching cache rule with explicit TTL
	for _, cr := range p.compiled {
		if p.matchRulePath(cr, reqPath) && cr.rule.Cache.TTLSeconds > 0 {
			return cr.rule.Cache.TTL()
		}
	}

	// Priority 2: Origin Cache-Control s-maxage
	if p.cfg.RespectOriginCC {
		cc := headers.Get("Cache-Control")
		if ttl := parseSMaxAge(cc); ttl > 0 {
			return ttl
		}
		// Priority 3: Origin Cache-Control max-age
		if ttl := parseMaxAge(cc); ttl > 0 {
			return ttl
		}
	}

	// Priority 4: Default TTL
	return p.cfg.DefaultTTL()
}

func (p *Policy) matchRule(cr compiledCacheRule, r *http.Request) bool {
	return p.matchRulePath(cr, r.URL.Path) && p.matchRuleMethod(cr, r.Method)
}

func (p *Policy) matchRulePath(cr compiledCacheRule, path string) bool {
	rule := cr.rule

	if rule.PathExact != "" {
		return path == rule.PathExact
	}

	if rule.PathPrefix != "" {
		return strings.HasPrefix(path, rule.PathPrefix)
	}

	if cr.pathRegex != nil {
		return cr.pathRegex.MatchString(path)
	}

	return false
}

func (p *Policy) matchRuleMethod(cr compiledCacheRule, method string) bool {
	if len(cr.rule.Methods) == 0 {
		return true
	}
	for _, m := range cr.rule.Methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// parseSMaxAge extracts s-maxage value from Cache-Control header.
func parseSMaxAge(cc string) time.Duration {
	return parseCCDirective(cc, "s-maxage=")
}

// parseMaxAge extracts max-age value from Cache-Control header.
func parseMaxAge(cc string) time.Duration {
	return parseCCDirective(cc, "max-age=")
}

func parseCCDirective(cc, directive string) time.Duration {
	lower := strings.ToLower(cc)
	idx := strings.Index(lower, directive)
	if idx == -1 {
		return 0
	}
	val := lower[idx+len(directive):]
	end := strings.IndexAny(val, ", ")
	if end != -1 {
		val = val[:end]
	}
	var seconds int
	for _, c := range val {
		if c >= '0' && c <= '9' {
			seconds = seconds*10 + int(c-'0')
		} else {
			break
		}
	}
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
