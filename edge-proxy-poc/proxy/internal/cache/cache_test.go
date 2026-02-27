package cache

import (
	"net/http"
	"testing"
	"time"

	"github.com/openflare/edge-proxy/internal/config"
)

func TestMemoryBackendGetSet(t *testing.T) {
	mb := NewMemoryBackend(100, 1<<20)

	entry := &Entry{
		Key:        "test-key",
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": {"text/html"}},
		Body:       []byte("hello"),
		StoredAt:   time.Now(),
		TTL:        10 * time.Second,
		Size:       5,
	}

	mb.Set("test-key", entry)

	got, ok := mb.Get("test-key")
	if !ok {
		t.Fatal("expected to find entry")
	}
	if string(got.Body) != "hello" {
		t.Errorf("expected body 'hello', got '%s'", string(got.Body))
	}
}

func TestMemoryBackendExpiration(t *testing.T) {
	mb := NewMemoryBackend(100, 1<<20)

	entry := &Entry{
		Key:      "expired",
		Body:     []byte("old"),
		StoredAt: time.Now().Add(-10 * time.Second),
		TTL:      5 * time.Second,
		Size:     3,
	}

	mb.Set("expired", entry)

	_, ok := mb.Get("expired")
	if ok {
		t.Error("expected expired entry to not be found")
	}
}

func TestMemoryBackendLRUEviction(t *testing.T) {
	mb := NewMemoryBackend(3, 1<<20) // Max 3 entries

	for i := 0; i < 5; i++ {
		entry := &Entry{
			Key:      "key",
			Body:     []byte("data"),
			StoredAt: time.Now(),
			TTL:      1 * time.Minute,
			Size:     4,
		}
		key := "key-" + string(rune('a'+i))
		mb.Set(key, entry)
	}

	if mb.Len() != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", mb.Len())
	}

	// First entries should be evicted
	_, ok := mb.Get("key-a")
	if ok {
		t.Error("expected key-a to be evicted")
	}
	_, ok = mb.Get("key-b")
	if ok {
		t.Error("expected key-b to be evicted")
	}
}

func TestMemoryBackendDelete(t *testing.T) {
	mb := NewMemoryBackend(100, 1<<20)

	entry := &Entry{
		Body:     []byte("hello"),
		StoredAt: time.Now(),
		TTL:      1 * time.Minute,
		Size:     5,
	}
	mb.Set("to-delete", entry)

	ok := mb.Delete("to-delete")
	if !ok {
		t.Error("expected Delete to return true")
	}

	_, found := mb.Get("to-delete")
	if found {
		t.Error("expected entry to be deleted")
	}
}

func TestMemoryBackendPurge(t *testing.T) {
	mb := NewMemoryBackend(100, 1<<20)

	for i := 0; i < 10; i++ {
		entry := &Entry{
			Body:     []byte("data"),
			StoredAt: time.Now(),
			TTL:      1 * time.Minute,
			Size:     4,
		}
		mb.Set("key-"+string(rune('a'+i)), entry)
	}

	count := mb.Purge()
	if count != 10 {
		t.Errorf("expected 10 purged, got %d", count)
	}
	if mb.Len() != 0 {
		t.Errorf("expected 0 entries after purge, got %d", mb.Len())
	}
}

func TestMemoryBackendDeleteByPrefix(t *testing.T) {
	mb := NewMemoryBackend(100, 1<<20)

	// Cache key format: METHOD|host|path|query|ae
	entries := []struct {
		key string
	}{
		{"GET|example.com|/static/app.js||gzip"},
		{"GET|example.com|/static/style.css||gzip"},
		{"GET|example.com|/api/time||gzip"},
	}

	for _, e := range entries {
		entry := &Entry{
			Body:     []byte("data"),
			StoredAt: time.Now(),
			TTL:      1 * time.Minute,
			Size:     4,
		}
		mb.Set(e.key, entry)
	}

	count := mb.DeleteByPrefix("/static/")
	if count != 2 {
		t.Errorf("expected 2 deleted by prefix, got %d", count)
	}
	if mb.Len() != 1 {
		t.Errorf("expected 1 entry remaining, got %d", mb.Len())
	}
}

func TestMemoryBackendStats(t *testing.T) {
	mb := NewMemoryBackend(100, 1<<20)

	entry := &Entry{
		Body:     []byte("hello"),
		StoredAt: time.Now(),
		TTL:      1 * time.Minute,
		Size:     5,
	}
	mb.Set("key1", entry)
	mb.Get("key1") // hit
	mb.Get("key2") // miss

	stats := mb.Stats()
	if stats.Entries != 1 {
		t.Errorf("expected 1 entry, got %d", stats.Entries)
	}
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Stores != 1 {
		t.Errorf("expected 1 store, got %d", stats.Stores)
	}
}

func TestPolicyRequestCacheable(t *testing.T) {
	cfg := config.CacheConfig{
		Enabled:               true,
		BypassOnCookie:        true,
		BypassOnAuthorization: true,
	}
	policy := NewPolicy(cfg, nil)

	tests := []struct {
		method  string
		headers map[string]string
		want    bool
	}{
		{"GET", nil, true},
		{"HEAD", nil, true},
		{"POST", nil, false},
		{"GET", map[string]string{"Cookie": "session=abc"}, false},
		{"GET", map[string]string{"Authorization": "Bearer token"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, "http://example.com/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			got, _ := policy.IsRequestCacheable(req)
			if got != tt.want {
				t.Errorf("IsRequestCacheable(%s) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestPolicyResponseStorable(t *testing.T) {
	cfg := config.CacheConfig{
		NeverStoreOnSetCookie: true,
	}
	policy := NewPolicy(cfg, nil)

	tests := []struct {
		name    string
		status  int
		headers http.Header
		want    bool
	}{
		{"200 OK", 200, http.Header{}, true},
		{"404 Not Found", 404, http.Header{}, false},
		{"500 Error", 500, http.Header{}, false},
		{"no-store", 200, http.Header{"Cache-Control": {"no-store"}}, false},
		{"private", 200, http.Header{"Cache-Control": {"private"}}, false},
		{"Set-Cookie", 200, http.Header{"Set-Cookie": {"session=abc"}}, false},
		{"public max-age", 200, http.Header{"Cache-Control": {"public, max-age=60"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := policy.IsResponseStorable(tt.status, tt.headers)
			if got != tt.want {
				t.Errorf("IsResponseStorable = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyDetermineTTL(t *testing.T) {
	cfg := config.CacheConfig{
		DefaultTTLSeconds:  30,
		RespectOriginCC:    true,
	}
	rules := []config.CacheRule{
		{
			ID:         "static",
			PathPrefix: "/static/",
			Cache:      config.CacheRuleCache{Eligible: true, TTLSeconds: 3600},
		},
	}
	policy := NewPolicy(cfg, rules)

	// Rule TTL
	ttl := policy.DetermineTTL("/static/app.js", http.Header{})
	if ttl != 3600*time.Second {
		t.Errorf("expected 3600s for /static/, got %v", ttl)
	}

	// s-maxage
	ttl = policy.DetermineTTL("/other", http.Header{"Cache-Control": {"s-maxage=120"}})
	if ttl != 120*time.Second {
		t.Errorf("expected 120s for s-maxage, got %v", ttl)
	}

	// max-age
	ttl = policy.DetermineTTL("/other", http.Header{"Cache-Control": {"max-age=60"}})
	if ttl != 60*time.Second {
		t.Errorf("expected 60s for max-age, got %v", ttl)
	}

	// default
	ttl = policy.DetermineTTL("/other", http.Header{})
	if ttl != 30*time.Second {
		t.Errorf("expected 30s default, got %v", ttl)
	}
}
