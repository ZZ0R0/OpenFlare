package server

import (
	"net/http"

	"github.com/openflare/edge-proxy/internal/admin"
	"github.com/openflare/edge-proxy/internal/config"
	"github.com/openflare/edge-proxy/internal/middleware"
	"github.com/openflare/edge-proxy/internal/observability"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewPublicServer creates the public-facing HTTP server.
func NewPublicServer(cfg config.ServerConfig, handler http.Handler, metrics *observability.Metrics) *http.Server {
	// Build middleware chain
	chain := middleware.RequestID(
		middleware.AccessLog(metrics)(
			middleware.Recovery(
				handler,
			),
		),
	)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", chain)

	return &http.Server{
		Addr:           cfg.ListenAddr,
		Handler:        mux,
		ReadTimeout:    cfg.ReadTimeout(),
		WriteTimeout:   cfg.WriteTimeout(),
		IdleTimeout:    cfg.IdleTimeout(),
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}
}

// NewAdminServer creates the admin API HTTP server.
func NewAdminServer(cfg config.ServerConfig, adminHandler *admin.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/admin/", adminHandler)
	mux.Handle("/metrics", promhttp.Handler())

	// Also serve health on admin port
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	return &http.Server{
		Addr:           cfg.AdminListenAddr,
		Handler:        mux,
		ReadTimeout:    cfg.ReadTimeout(),
		WriteTimeout:   cfg.WriteTimeout(),
		IdleTimeout:    cfg.IdleTimeout(),
		MaxHeaderBytes: cfg.MaxHeaderBytes,
	}
}
