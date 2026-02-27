package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the proxy.
type Metrics struct {
	RequestsTotal          *prometheus.CounterVec
	RequestDuration        *prometheus.HistogramVec
	CacheOperations        *prometheus.CounterVec
	CacheSizeBytes         prometheus.Gauge
	CacheEntries           prometheus.Gauge
	WAFDecisions           *prometheus.CounterVec
	RateLimitTotal         *prometheus.CounterVec
	UpstreamErrors         *prometheus.CounterVec
	UpstreamDuration       *prometheus.HistogramVec
}

// NewMetrics creates and registers all metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "openflare_requests_total",
				Help: "Total number of HTTP requests processed",
			},
			[]string{"method", "status", "route"},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "openflare_request_duration_seconds",
				Help:    "Request processing duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route"},
		),
		CacheOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "openflare_cache_operations_total",
				Help: "Total cache operations by type",
			},
			[]string{"operation"},
		),
		CacheSizeBytes: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "openflare_cache_size_bytes",
				Help: "Current cache size in bytes",
			},
		),
		CacheEntries: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "openflare_cache_entries",
				Help: "Current number of cache entries",
			},
		),
		WAFDecisions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "openflare_waf_decisions_total",
				Help: "Total WAF decisions by rule and action",
			},
			[]string{"rule_id", "action"},
		),
		RateLimitTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "openflare_ratelimit_total",
				Help: "Total rate limit decisions",
			},
			[]string{"rule_id", "decision"},
		),
		UpstreamErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "openflare_upstream_errors_total",
				Help: "Total upstream errors",
			},
			[]string{"upstream", "error_type"},
		),
		UpstreamDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "openflare_upstream_duration_seconds",
				Help:    "Upstream request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"upstream"},
		),
	}
}
