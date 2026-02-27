package normalize

import (
	"net/url"
	"path"
	"sort"
	"strings"
)

// Path normalizes an HTTP path:
// - Resolves dot segments (., ..)
// - Collapses double slashes
// - Ensures leading slash
func Path(p string) string {
	if p == "" {
		return "/"
	}

	// Decode percent-encoded characters for normalization
	decoded, err := url.PathUnescape(p)
	if err != nil {
		decoded = p
	}

	// Use path.Clean to resolve dots and double slashes
	cleaned := path.Clean(decoded)

	// Ensure leading slash
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}

	// Preserve trailing slash if original had one
	if strings.HasSuffix(p, "/") && !strings.HasSuffix(cleaned, "/") && cleaned != "/" {
		cleaned += "/"
	}

	return cleaned
}

// Query normalizes a query string by sorting parameters alphabetically.
func Query(q string) string {
	if q == "" {
		return ""
	}

	// Remove leading ?
	q = strings.TrimPrefix(q, "?")

	values, err := url.ParseQuery(q)
	if err != nil {
		return q
	}

	// Sort keys
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Rebuild sorted query string
	var parts []string
	for _, k := range keys {
		vals := values[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	return strings.Join(parts, "&")
}

// AcceptEncoding normalizes Accept-Encoding header to a stable form.
func AcceptEncoding(ae string) string {
	if ae == "" {
		return "identity"
	}

	ae = strings.ToLower(ae)
	parts := strings.Split(ae, ",")

	var encodings []string
	for _, p := range parts {
		enc := strings.TrimSpace(strings.Split(p, ";")[0])
		if enc != "" {
			encodings = append(encodings, enc)
		}
	}

	sort.Strings(encodings)
	return strings.Join(encodings, ",")
}

// CacheKey builds a normalized cache key from method, host, path, query and accept-encoding.
func CacheKey(method, host, reqPath, query, acceptEncoding string) string {
	m := strings.ToUpper(method)
	// Treat HEAD as GET for cache key purposes
	if m == "HEAD" {
		m = "GET"
	}
	h := strings.ToLower(host)
	p := Path(reqPath)
	q := Query(query)
	ae := AcceptEncoding(acceptEncoding)

	return m + "|" + h + "|" + p + "|" + q + "|" + ae
}
