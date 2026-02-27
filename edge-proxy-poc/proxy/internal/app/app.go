package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openflare/edge-proxy/internal/admin"
	"github.com/openflare/edge-proxy/internal/cache"
	"github.com/openflare/edge-proxy/internal/config"
	"github.com/openflare/edge-proxy/internal/observability"
	"github.com/openflare/edge-proxy/internal/proxy"
	"github.com/openflare/edge-proxy/internal/ratelimit"
	"github.com/openflare/edge-proxy/internal/server"
	"github.com/openflare/edge-proxy/internal/upstream"
	"github.com/openflare/edge-proxy/internal/waf"
)

var appLogger = log.New(os.Stdout, "", 0)

func logJSON(level, msg string, fields map[string]interface{}) {
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"level":     level,
		"msg":       msg,
	}
	for k, v := range fields {
		entry[k] = v
	}
	data, _ := json.Marshal(entry)
	appLogger.Println(string(data))
}

// Run is the main application entrypoint.
func Run() error {
	// Load config
	configPath := os.Getenv("OPENFLARE_CONFIG")
	if configPath == "" {
		configPath = "configs/proxy.dev.yaml"
	}

	logJSON("info", "loading configuration", map[string]interface{}{"path": configPath})

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Load WAF config
	wafConfigPath := os.Getenv("OPENFLARE_WAF_RULES")
	if wafConfigPath == "" {
		wafConfigPath = "configs/rules.waf.yaml"
	}

	var wafCfg *config.WAFConfig
	wafCfg, err = config.LoadWAFConfig(wafConfigPath)
	if err != nil {
		logJSON("warn", "WAF config not loaded, WAF disabled", map[string]interface{}{"error": err.Error()})
	}

	// Load cache rules
	cacheRulesPath := os.Getenv("OPENFLARE_CACHE_RULES")
	if cacheRulesPath == "" {
		cacheRulesPath = "configs/cache-rules.yaml"
	}

	var cacheRulesCfg *config.CacheRulesConfig
	cacheRulesCfg, err = config.LoadCacheRulesConfig(cacheRulesPath)
	if err != nil {
		logJSON("warn", "cache rules not loaded, using defaults", map[string]interface{}{"error": err.Error()})
		cacheRulesCfg = &config.CacheRulesConfig{}
	}

	// Load rate limit config
	rlConfigPath := os.Getenv("OPENFLARE_RATE_LIMIT_RULES")
	if rlConfigPath == "" {
		rlConfigPath = "configs/rate-limit.yaml"
	}

	var rlCfg *config.RateLimitConfig
	rlCfg, err = config.LoadRateLimitConfig(rlConfigPath)
	if err != nil {
		logJSON("warn", "rate limit config not loaded", map[string]interface{}{"error": err.Error()})
	}

	// Initialize metrics
	metrics := observability.NewMetrics()

	// Initialize cache
	cacheBackend := cache.NewMemoryBackend(cfg.Cache.MaxEntries, cfg.Cache.MaxBytes)
	cachePolicy := cache.NewPolicy(cfg.Cache, cacheRulesCfg.Rules)

	// Initialize WAF
	wafEngine, err := waf.NewEngine(wafCfg, metrics, cfg.Security.MaxBodyInspectBytes)
	if err != nil {
		return fmt.Errorf("init WAF engine: %w", err)
	}
	logJSON("info", "WAF initialized", map[string]interface{}{"mode": wafEngine.Mode()})

	// Initialize rate limiter
	rateLimiter := ratelimit.NewLimiter(rlCfg, metrics)

	// Initialize upstream transport
	var upstreamCfg config.UpstreamConfig
	for _, u := range cfg.Upstreams {
		if u.Name == cfg.Routing.DefaultUpstream {
			upstreamCfg = u
			break
		}
	}
	transport := upstream.NewTransport(upstreamCfg)

	// Initialize proxy handler
	proxyHandler, err := proxy.NewHandler(cfg, cacheBackend, cachePolicy, wafEngine, rateLimiter, metrics, transport)
	if err != nil {
		return fmt.Errorf("init proxy handler: %w", err)
	}

	// Initialize admin handler
	adminHandler := admin.NewHandler(cacheBackend, cfg.AdminAPI.AuthToken)

	// Create servers
	publicServer := server.NewPublicServer(cfg.Server, proxyHandler, metrics)
	adminServer := server.NewAdminServer(cfg.Server, adminHandler)

	// Start servers
	errCh := make(chan error, 2)

	go func() {
		logJSON("info", "starting public server", map[string]interface{}{"addr": cfg.Server.ListenAddr})
		if err := publicServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("public server: %w", err)
		}
	}()

	if cfg.AdminAPI.Enabled {
		go func() {
			logJSON("info", "starting admin server", map[string]interface{}{"addr": cfg.Server.AdminListenAddr})
			if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("admin server: %w", err)
			}
		}()
	}

	logJSON("info", "OpenFlare Edge Proxy started", map[string]interface{}{
		"public_addr": cfg.Server.ListenAddr,
		"admin_addr":  cfg.Server.AdminListenAddr,
		"cache":       cfg.Cache.Enabled,
		"waf_mode":    wafEngine.Mode(),
	})

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logJSON("info", "shutdown signal received", map[string]interface{}{"signal": sig.String()})
	case err := <-errCh:
		return err
	}

	// Graceful shutdown
	shutdownTimeout := cfg.Server.GracefulShutdownTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logJSON("info", "graceful shutdown", map[string]interface{}{"timeout_ms": cfg.Server.GracefulShutdownTimeoutMS})

	if err := publicServer.Shutdown(ctx); err != nil {
		logJSON("error", "public server shutdown error", map[string]interface{}{"error": err.Error()})
	}

	if cfg.AdminAPI.Enabled {
		if err := adminServer.Shutdown(ctx); err != nil {
			logJSON("error", "admin server shutdown error", map[string]interface{}{"error": err.Error()})
		}
	}

	logJSON("info", "shutdown complete", nil)
	return nil
}
