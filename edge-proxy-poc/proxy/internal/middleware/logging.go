package middleware

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/openflare/edge-proxy/internal/observability"
)

var logger = log.New(os.Stdout, "", 0)

// statusWriter wraps http.ResponseWriter to capture status code.
type statusWriter struct {
	http.ResponseWriter
	status int
	written bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.status = http.StatusOK
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// AccessLog is a middleware that logs each request in JSON format with timing.
func AccessLog(metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)
			reqID := GetRequestID(r.Context())

			// Build structured log entry
			entry := map[string]interface{}{
				"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
				"level":       "info",
				"msg":         "access",
				"request_id":  reqID,
				"method":      r.Method,
				"path":        r.URL.Path,
				"query":       r.URL.RawQuery,
				"status":      sw.status,
				"duration_ms": fmt.Sprintf("%.2f", float64(duration.Microseconds())/1000.0),
				"client_ip":   r.RemoteAddr,
				"user_agent":  r.UserAgent(),
				"cache_status": w.Header().Get("X-Edge-Cache"),
			}

			data, _ := json.Marshal(entry)
			logger.Println(string(data))

			// Record metrics
			route := r.URL.Path
			statusStr := strconv.Itoa(sw.status)
			metrics.RequestsTotal.WithLabelValues(r.Method, statusStr, route).Inc()
			metrics.RequestDuration.WithLabelValues(r.Method, route).Observe(duration.Seconds())
		})
	}
}
