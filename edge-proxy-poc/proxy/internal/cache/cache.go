package cache

import (
	"container/list"
	"net/http"
	"sync"
	"time"
)

// Entry represents a cached HTTP response.
type Entry struct {
	Key        string
	StatusCode int
	Headers    http.Header
	Body       []byte
	StoredAt   time.Time
	TTL        time.Duration
	Size       int64
}

// IsExpired returns true if the entry has expired.
func (e *Entry) IsExpired() bool {
	return time.Since(e.StoredAt) > e.TTL
}

// Stats holds cache statistics.
type Stats struct {
	Entries    int   `json:"entries"`
	SizeBytes  int64 `json:"size_bytes"`
	MaxEntries int   `json:"max_entries"`
	MaxBytes   int   `json:"max_bytes"`
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Bypasses   int64 `json:"bypasses"`
	Stores     int64 `json:"stores"`
	Evictions  int64 `json:"evictions"`
}

// Backend is the interface for cache storage backends.
type Backend interface {
	Get(key string) (*Entry, bool)
	Set(key string, entry *Entry)
	Delete(key string) bool
	DeleteByPrefix(prefix string) int
	DeleteByPath(path string) int
	Purge() int
	Stats() Stats
	Len() int
}

// MemoryBackend is an in-memory LRU cache backend.
type MemoryBackend struct {
	mu         sync.RWMutex
	maxEntries int
	maxBytes   int
	items      map[string]*list.Element
	evictList  *list.List
	sizeBytes  int64
	hits       int64
	misses     int64
	bypasses   int64
	stores     int64
	evictions  int64
}

type lruItem struct {
	key   string
	entry *Entry
}

// NewMemoryBackend creates a new in-memory LRU cache.
func NewMemoryBackend(maxEntries, maxBytes int) *MemoryBackend {
	return &MemoryBackend{
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
		items:      make(map[string]*list.Element),
		evictList:  list.New(),
	}
}

// Get retrieves an entry from the cache.
func (m *MemoryBackend) Get(key string) (*Entry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	elem, ok := m.items[key]
	if !ok {
		m.misses++
		return nil, false
	}

	item := elem.Value.(*lruItem)
	if item.entry.IsExpired() {
		m.removeElement(elem)
		m.misses++
		return nil, false
	}

	// Move to front (most recently used)
	m.evictList.MoveToFront(elem)
	m.hits++
	return item.entry, true
}

// Set stores an entry in the cache.
func (m *MemoryBackend) Set(key string, entry *Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If updating existing entry, remove old one first
	if elem, ok := m.items[key]; ok {
		m.removeElement(elem)
	}

	// Evict until we have space
	for m.evictList.Len() >= m.maxEntries || (m.sizeBytes+entry.Size > int64(m.maxBytes) && m.evictList.Len() > 0) {
		m.evictOldest()
	}

	item := &lruItem{key: key, entry: entry}
	elem := m.evictList.PushFront(item)
	m.items[key] = elem
	m.sizeBytes += entry.Size
	m.stores++
}

// Delete removes an entry by exact key.
func (m *MemoryBackend) Delete(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	elem, ok := m.items[key]
	if !ok {
		return false
	}
	m.removeElement(elem)
	return true
}

// DeleteByPrefix removes all entries whose key contains the given prefix path.
func (m *MemoryBackend) DeleteByPrefix(prefix string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	var toRemove []*list.Element
	for _, elem := range m.items {
		item := elem.Value.(*lruItem)
		// Check if key contains the prefix (cache key format: METHOD|host|path|query|ae)
		if containsPrefix(item.key, prefix) {
			toRemove = append(toRemove, elem)
		}
	}
	for _, elem := range toRemove {
		m.removeElement(elem)
		count++
	}
	return count
}

// DeleteByPath removes all entries whose cache key path segment matches exactly.
func (m *MemoryBackend) DeleteByPath(path string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	var toRemove []*list.Element
	for _, elem := range m.items {
		item := elem.Value.(*lruItem)
		parts := splitCacheKey(item.key)
		if len(parts) >= 3 && parts[2] == path {
			toRemove = append(toRemove, elem)
		}
	}
	for _, elem := range toRemove {
		m.removeElement(elem)
		count++
	}
	return count
}

// Purge removes all entries.
func (m *MemoryBackend) Purge() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := m.evictList.Len()
	m.items = make(map[string]*list.Element)
	m.evictList.Init()
	m.sizeBytes = 0
	return count
}

// Stats returns current cache statistics.
func (m *MemoryBackend) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Stats{
		Entries:    m.evictList.Len(),
		SizeBytes:  m.sizeBytes,
		MaxEntries: m.maxEntries,
		MaxBytes:   m.maxBytes,
		Hits:       m.hits,
		Misses:     m.misses,
		Bypasses:   m.bypasses,
		Stores:     m.stores,
		Evictions:  m.evictions,
	}
}

// Len returns the number of entries in the cache.
func (m *MemoryBackend) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.evictList.Len()
}

// IncrementBypasses increments the bypass counter.
func (m *MemoryBackend) IncrementBypasses() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bypasses++
}

func (m *MemoryBackend) removeElement(elem *list.Element) {
	item := elem.Value.(*lruItem)
	delete(m.items, item.key)
	m.evictList.Remove(elem)
	m.sizeBytes -= item.entry.Size
	if m.sizeBytes < 0 {
		m.sizeBytes = 0
	}
}

func (m *MemoryBackend) evictOldest() {
	elem := m.evictList.Back()
	if elem != nil {
		m.removeElement(elem)
		m.evictions++
	}
}

// containsPrefix checks if a cache key contains a path prefix.
// Cache key format: METHOD|host|path|query|ae
func containsPrefix(key, prefix string) bool {
	// Find the path segment in the cache key
	parts := splitCacheKey(key)
	if len(parts) < 3 {
		return false
	}
	path := parts[2]
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

func splitCacheKey(key string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			parts = append(parts, key[start:i])
			start = i + 1
		}
	}
	parts = append(parts, key[start:])
	return parts
}
