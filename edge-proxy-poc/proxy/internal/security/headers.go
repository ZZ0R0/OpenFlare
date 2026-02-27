package security

import (
	"net"
	"net/http"
)

// SetForwardedHeaders sets appropriate forwarding headers on the request.
func SetForwardedHeaders(r *http.Request, trustXFF bool) {
	clientIP := extractIP(r.RemoteAddr)

	if !trustXFF {
		// Override any existing X-Forwarded-For
		r.Header.Set("X-Forwarded-For", clientIP)
	} else {
		// Append to existing
		existing := r.Header.Get("X-Forwarded-For")
		if existing != "" {
			r.Header.Set("X-Forwarded-For", existing+", "+clientIP)
		} else {
			r.Header.Set("X-Forwarded-For", clientIP)
		}
	}

	// Set X-Forwarded-Proto
	if r.TLS != nil {
		r.Header.Set("X-Forwarded-Proto", "https")
	} else {
		r.Header.Set("X-Forwarded-Proto", "http")
	}
}

func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
