package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/openflare/edge-proxy/internal/cache"
)

// Handler handles admin API requests.
type Handler struct {
	cacheBackend cache.Backend
	authToken    string
}

// NewHandler creates a new admin handler.
func NewHandler(cacheBackend cache.Backend, authToken string) *Handler {
	return &Handler{
		cacheBackend: cacheBackend,
		authToken:    authToken,
	}
}

// ServeHTTP routes admin API requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Auth check
	if !h.authenticate(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	path := r.URL.Path

	switch {
	case path == "/admin/cache/stats" && r.Method == http.MethodGet:
		h.handleCacheStats(w, r)
	case path == "/admin/cache/purge" && r.Method == http.MethodPost:
		h.handleCachePurge(w, r)
	case path == "/admin/cache/purge-prefix" && r.Method == http.MethodPost:
		h.handleCachePurgePrefix(w, r)
	case path == "/admin/cache/purge-all" && r.Method == http.MethodPost:
		h.handleCachePurgeAll(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}
}

func (h *Handler) authenticate(r *http.Request) bool {
	if h.authToken == "" {
		return true
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	// Support "Bearer <token>" format
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1] == h.authToken
	}

	return auth == h.authToken
}

func (h *Handler) handleCacheStats(w http.ResponseWriter, _ *http.Request) {
	stats := h.cacheBackend.Stats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handleCachePurge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	key := req.Key
	if key == "" {
		key = req.URL
	}

	// Try exact key first
	deleted := h.cacheBackend.Delete(key)
	// If exact key didn't match and key looks like a URL path, try path-based deletion
	if !deleted && len(key) > 0 && key[0] == '/' {
		count := h.cacheBackend.DeleteByPath(key)
		deleted = count > 0
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": deleted,
		"key":     key,
	})
}

func (h *Handler) handleCachePurgePrefix(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Prefix string `json:"prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	count := h.cacheBackend.DeleteByPrefix(req.Prefix)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted_count": count,
		"prefix":        req.Prefix,
	})
}

func (h *Handler) handleCachePurgeAll(w http.ResponseWriter, _ *http.Request) {
	count := h.cacheBackend.Purge()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"purged_count": count,
	})
}
