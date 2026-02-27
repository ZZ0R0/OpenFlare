package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/openflare/edge-proxy/internal/cache"
	"github.com/openflare/edge-proxy/internal/config"
	"github.com/openflare/edge-proxy/internal/middleware"
	"github.com/openflare/edge-proxy/internal/normalize"
	"github.com/openflare/edge-proxy/internal/observability"
	"github.com/openflare/edge-proxy/internal/ratelimit"
	"github.com/openflare/edge-proxy/internal/security"
	"github.com/openflare/edge-proxy/internal/waf"
	"golang.org/x/sync/singleflight"
)

// Handler is the main proxy handler that integrates cache, WAF, and rate limiting.
type Handler struct {
	cfg          *config.Config
	cacheBackend *cache.MemoryBackend
	cachePolicy  *cache.Policy
	wafEngine    *waf.Engine
	rateLimiter  *ratelimit.Limiter
	metrics      *observability.Metrics
	reverseProxy *httputil.ReverseProxy
	targetURL    *url.URL
	sfGroup      singleflight.Group
}

// NewHandler creates a new proxy handler.
func NewHandler(
	cfg *config.Config,
	cacheBackend *cache.MemoryBackend,
	cachePolicy *cache.Policy,
	wafEngine *waf.Engine,
	rateLimiter *ratelimit.Limiter,
	metrics *observability.Metrics,
	transport http.RoundTripper,
) (*Handler, error) {
	// Find default upstream
	var upstream *config.UpstreamConfig
	for i := range cfg.Upstreams {
		if cfg.Upstreams[i].Name == cfg.Routing.DefaultUpstream {
			upstream = &cfg.Upstreams[i]
			break
		}
	}
	if upstream == nil {
		return nil, http.ErrAbortHandler
	}

	targetURL, err := url.Parse(upstream.BaseURL)
	if err != nil {
		return nil, err
	}

	rp := httputil.NewSingleHostReverseProxy(targetURL)
	rp.Transport = transport
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		reqID := middleware.GetRequestID(r.Context())
		metrics.UpstreamErrors.WithLabelValues(upstream.Name, "proxy_error").Inc()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", reqID)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"bad gateway","detail":"upstream unreachable"}`))
	}

	h := &Handler{
		cfg:          cfg,
		cacheBackend: cacheBackend,
		cachePolicy:  cachePolicy,
		wafEngine:    wafEngine,
		rateLimiter:  rateLimiter,
		metrics:      metrics,
		reverseProxy: rp,
		targetURL:    targetURL,
	}

	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle built-in endpoints
	switch r.URL.Path {
	case "/healthz":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
		return
	case "/readyz":
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
		return
	case "/metrics":
		// Prometheus metrics are served by the metrics handler registered separately
		// This is a fallback if not separately mounted
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# metrics served via /metrics endpoint"))
		return
	}

	// 1. Read body for WAF inspection (if needed)
	var body []byte
	if r.Body != nil && r.ContentLength != 0 {
		var err error
		body, err = waf.ReadBodyForInspection(r, h.cfg.Security.MaxBodyInspectBytes)
		if err != nil {
			http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
			return
		}
	}

	// 2. Normalize path and query
	r.URL.Path = normalize.Path(r.URL.Path)
	if r.URL.RawQuery != "" {
		r.URL.RawQuery = normalize.Query(r.URL.RawQuery)
	}

	// 3. WAF inspection
	if h.wafEngine != nil {
		decision := h.wafEngine.Evaluate(r, body)
		if decision.Matched && decision.Action == "block" {
			statusCode := h.wafEngine.GetBlockStatusCode(decision.RuleID)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Request-ID", middleware.GetRequestID(r.Context()))
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"error":"blocked by WAF","rule_id":"` + decision.RuleID + `"}`))
			return
		}
		if decision.Matched && decision.Action == "bypass_cache" {
			r.Header.Set("X-OpenFlare-No-Cache", "1")
		}
	}

	// 4. Rate limiting
	if h.rateLimiter != nil {
		allowed, ruleID, _ := h.rateLimiter.Allow(r)
		if !allowed {
			ratelimit.HandleDeny(w, ruleID)
			return
		}
	}

	// 5. Set forwarded headers
	security.SetForwardedHeaders(r, h.cfg.Security.TrustXForwardedFor)

	// 6. Cache handling
	if h.cfg.Cache.Enabled {
		h.handleWithCache(w, r)
		return
	}

	// No cache - just proxy
	h.proxyToOrigin(w, r)
}

func (h *Handler) handleWithCache(w http.ResponseWriter, r *http.Request) {
	// Check request eligibility
	cacheable, reason := h.cachePolicy.IsRequestCacheable(r)
	if !cacheable || r.Header.Get("X-OpenFlare-No-Cache") == "1" {
		if reason == "" {
			reason = "waf_bypass"
		}
		h.cacheBackend.IncrementBypasses()
		h.metrics.CacheOperations.WithLabelValues("bypass").Inc()
		h.setDebugHeaders(w, string(cache.StatusBYPASS))
		h.proxyToOrigin(w, r)
		return
	}

	// Build cache key
	cacheKey := normalize.CacheKey(
		r.Method,
		r.Host,
		r.URL.Path,
		r.URL.RawQuery,
		r.Header.Get("Accept-Encoding"),
	)

	// Cache lookup
	if entry, found := h.cacheBackend.Get(cacheKey); found {
		h.metrics.CacheOperations.WithLabelValues("hit").Inc()
		h.serveCached(w, r, entry)
		return
	}

	// Cache MISS - fetch via singleflight
	h.metrics.CacheOperations.WithLabelValues("miss").Inc()

	type sfResult struct {
		entry *cache.Entry
		err   error
	}

	result, _, _ := h.sfGroup.Do(cacheKey, func() (interface{}, error) {
		// Double-check cache (another goroutine may have populated it)
		if entry, found := h.cacheBackend.Get(cacheKey); found {
			return &sfResult{entry: entry}, nil
		}

		// Fetch from origin
		entry, err := h.fetchFromOrigin(r)
		if err != nil {
			return &sfResult{err: err}, nil
		}

		// Store decision
		if entry != nil {
			storable, _ := h.cachePolicy.IsResponseStorable(entry.StatusCode, entry.Headers)
			if storable {
				ttl := h.cachePolicy.DetermineTTL(r.URL.Path, entry.Headers)
				entry.Key = cacheKey
				entry.TTL = ttl
				entry.StoredAt = time.Now()
				h.cacheBackend.Set(cacheKey, entry)
				h.metrics.CacheOperations.WithLabelValues("store").Inc()

				// Update cache metrics
				stats := h.cacheBackend.Stats()
				h.metrics.CacheSizeBytes.Set(float64(stats.SizeBytes))
				h.metrics.CacheEntries.Set(float64(stats.Entries))
			}
		}

		return &sfResult{entry: entry}, nil
	})

	sfRes := result.(*sfResult)
	if sfRes.err != nil {
		h.metrics.UpstreamErrors.WithLabelValues(h.cfg.Routing.DefaultUpstream, "fetch_error").Inc()
		w.Header().Set("X-Request-ID", middleware.GetRequestID(r.Context()))
		http.Error(w, `{"error":"upstream error"}`, http.StatusBadGateway)
		return
	}

	if sfRes.entry != nil {
		h.setDebugHeaders(w, string(cache.StatusMISS))
		h.writeEntry(w, sfRes.entry)
		return
	}

	// Fallback: direct proxy
	h.setDebugHeaders(w, string(cache.StatusMISS))
	h.proxyToOrigin(w, r)
}

func (h *Handler) serveCached(w http.ResponseWriter, _ *http.Request, entry *cache.Entry) {
	h.setDebugHeaders(w, string(cache.StatusHIT))
	h.writeEntry(w, entry)
}

func (h *Handler) writeEntry(w http.ResponseWriter, entry *cache.Entry) {
	// Copy cached headers
	for k, vals := range entry.Headers {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(entry.StatusCode)
	w.Write(entry.Body)
}

func (h *Handler) setDebugHeaders(w http.ResponseWriter, status string) {
	if h.cfg.Cache.DebugHeaders {
		w.Header().Set("X-Edge-Cache", status)
	}
}

func (h *Handler) fetchFromOrigin(r *http.Request) (*cache.Entry, error) {
	// Create a new request to origin
	originURL := h.targetURL.ResolveReference(r.URL)
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, originURL.String(), r.Body)
	if err != nil {
		return nil, err
	}

	// Copy relevant headers
	for k, v := range r.Header {
		outReq.Header[k] = v
	}

	start := time.Now()
	resp, err := h.reverseProxy.Transport.RoundTrip(outReq)
	duration := time.Since(start)

	h.metrics.UpstreamDuration.WithLabelValues(h.cfg.Routing.DefaultUpstream).Observe(duration.Seconds())

	if err != nil {
		h.metrics.UpstreamErrors.WithLabelValues(h.cfg.Routing.DefaultUpstream, "roundtrip_error").Inc()
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(h.cfg.Security.MaxRequestBodyBytes)))
	if err != nil {
		return nil, err
	}

	entry := &cache.Entry{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       bodyBytes,
		Size:       int64(len(bodyBytes)),
	}

	return entry, nil
}

func (h *Handler) proxyToOrigin(w http.ResponseWriter, r *http.Request) {
	// Restore body if consumed during WAF inspection
	if r.Body != nil {
		if body, err := io.ReadAll(r.Body); err == nil && len(body) > 0 {
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	start := time.Now()
	h.reverseProxy.ServeHTTP(w, r)
	duration := time.Since(start)
	h.metrics.UpstreamDuration.WithLabelValues(h.cfg.Routing.DefaultUpstream).Observe(duration.Seconds())
}
