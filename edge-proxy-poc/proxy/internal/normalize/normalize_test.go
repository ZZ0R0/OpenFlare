package normalize

import "testing"

func TestPathNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/", "/"},
		{"", "/"},
		{"/foo/bar", "/foo/bar"},
		{"/foo//bar", "/foo/bar"},
		{"/foo/./bar", "/foo/bar"},
		{"/foo/../bar", "/bar"},
		{"/foo/bar/", "/foo/bar/"},
		{"foo/bar", "/foo/bar"},
		{"/static/app%2Ev1.js", "/static/app.v1.js"},
		{"/../etc/passwd", "/etc/passwd"},
		{"/foo/%2e%2e/bar", "/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Path(tt.input)
			if got != tt.expected {
				t.Errorf("Path(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestQueryNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"a=1&b=2", "a=1&b=2"},
		{"b=2&a=1", "a=1&b=2"},
		{"c=3&a=1&b=2", "a=1&b=2&c=3"},
		{"a=2&a=1", "a=1&a=2"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Query(tt.input)
			if got != tt.expected {
				t.Errorf("Query(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAcceptEncoding(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "identity"},
		{"gzip", "gzip"},
		{"gzip, br", "br,gzip"},
		{"br;q=0.8, gzip;q=1.0", "br,gzip"},
		{"GZIP, BR, IDENTITY", "br,gzip,identity"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := AcceptEncoding(tt.input)
			if got != tt.expected {
				t.Errorf("AcceptEncoding(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCacheKey(t *testing.T) {
	tests := []struct {
		method, host, path, query, ae string
		expected                       string
	}{
		{"GET", "example.com", "/static/app.js", "v=1", "gzip",
			"GET|example.com|/static/app.js|v=1|gzip"},
		{"HEAD", "example.com", "/static/app.js", "v=1", "gzip",
			"GET|example.com|/static/app.js|v=1|gzip"},
		{"GET", "Example.COM", "/foo//bar", "b=2&a=1", "gzip, br",
			"GET|example.com|/foo/bar|a=1&b=2|br,gzip"},
	}

	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			got := CacheKey(tt.method, tt.host, tt.path, tt.query, tt.ae)
			if got != tt.expected {
				t.Errorf("CacheKey = %q, want %q", got, tt.expected)
			}
		})
	}
}
