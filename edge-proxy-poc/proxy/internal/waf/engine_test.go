package waf

import (
	"net/http"
	"testing"

	"github.com/openflare/edge-proxy/internal/config"
)

func makeWAFConfig(mode string, rules ...config.WAFRule) *config.WAFConfig {
	return &config.WAFConfig{
		Mode:  mode,
		Rules: rules,
	}
}

func TestPathTraversalMatch(t *testing.T) {
	wafCfg := makeWAFConfig("enforce", config.WAFRule{
		ID:      "block-path-traversal",
		Enabled: true,
		Phase:   "request",
		Match: config.WAFMatch{
			Any: []config.WAFCondition{
				{PathRegex: `(?i)(\.\./|%2e%2e%2f|%2e%2e/|\.\.%2f)`},
			},
		},
		Action: config.WAFAction{Type: "block", StatusCode: 403, Log: true},
	})

	engine, err := NewEngine(wafCfg, nil, 65536)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	tests := []struct {
		path    string
		matched bool
	}{
		{"/../etc/passwd", true},
		{"/foo/../bar", true},
		{"/foo/%2e%2e/bar", true},
		{"/normal/path", false},
		{"/static/app.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com"+tt.path, nil)
			decision := engine.Evaluate(req, nil)
			if decision.Matched != tt.matched {
				t.Errorf("path %s: matched=%v, want %v", tt.path, decision.Matched, tt.matched)
			}
		})
	}
}

func TestSQLiMatch(t *testing.T) {
	wafCfg := makeWAFConfig("enforce", config.WAFRule{
		ID:      "block-sqli",
		Enabled: true,
		Phase:   "request",
		Match: config.WAFMatch{
			Any: []config.WAFCondition{
				{QueryRegex: `(?i)(union\s+select|or\s+1\s*=\s*1|drop\s+table)`},
			},
		},
		Action: config.WAFAction{Type: "block", StatusCode: 403, Log: true},
	})

	engine, err := NewEngine(wafCfg, nil, 65536)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	tests := []struct {
		query   string
		matched bool
	}{
		{"q=1+UNION+SELECT+*+FROM+users", true},
		{"q=1+OR+1=1", true},
		{"q=DROP+TABLE+users", true},
		{"q=normal+search", false},
		{"q=hello+world", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com/search?"+tt.query, nil)
			decision := engine.Evaluate(req, nil)
			if decision.Matched != tt.matched {
				t.Errorf("query %s: matched=%v, want %v", tt.query, decision.Matched, tt.matched)
			}
		})
	}
}

func TestXSSMatch(t *testing.T) {
	wafCfg := makeWAFConfig("enforce", config.WAFRule{
		ID:      "shadow-xss",
		Enabled: true,
		Phase:   "request",
		Match: config.WAFMatch{
			Any: []config.WAFCondition{
				{QueryRegex: `(?i)(<script|javascript:|onerror\s*=)`},
			},
		},
		Action: config.WAFAction{Type: "log", Log: true},
	})

	engine, err := NewEngine(wafCfg, nil, 65536)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com/search?q=<script>alert(1)</script>", nil)
	decision := engine.Evaluate(req, nil)
	if !decision.Matched {
		t.Error("expected XSS pattern to match")
	}
}

func TestNormalRequestNoMatch(t *testing.T) {
	wafCfg := makeWAFConfig("enforce",
		config.WAFRule{
			ID:      "block-path-traversal",
			Enabled: true,
			Phase:   "request",
			Match: config.WAFMatch{
				Any: []config.WAFCondition{
					{PathRegex: `(?i)(\.\./|%2e%2e%2f)`},
				},
			},
			Action: config.WAFAction{Type: "block", StatusCode: 403, Log: true},
		},
		config.WAFRule{
			ID:      "block-sqli",
			Enabled: true,
			Phase:   "request",
			Match: config.WAFMatch{
				Any: []config.WAFCondition{
					{QueryRegex: `(?i)(union\s+select|or\s+1\s*=\s*1)`},
				},
			},
			Action: config.WAFAction{Type: "block", StatusCode: 403, Log: true},
		},
	)

	engine, err := NewEngine(wafCfg, nil, 65536)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com/api/users?page=1&limit=10", nil)
	decision := engine.Evaluate(req, nil)
	if decision.Matched {
		t.Errorf("expected no match for normal request, got rule=%s", decision.RuleID)
	}
}

func TestShadowModeNoBlock(t *testing.T) {
	wafCfg := makeWAFConfig("shadow", config.WAFRule{
		ID:      "block-sqli",
		Enabled: true,
		Phase:   "request",
		Match: config.WAFMatch{
			Any: []config.WAFCondition{
				{QueryRegex: `(?i)(union\s+select)`},
			},
		},
		Action: config.WAFAction{Type: "block", StatusCode: 403, Log: true},
	})

	engine, err := NewEngine(wafCfg, nil, 65536)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com/search?q=UNION+SELECT", nil)
	decision := engine.Evaluate(req, nil)
	if !decision.Matched {
		t.Error("expected match")
	}
	// In shadow mode, block becomes log
	if decision.Action != "log" {
		t.Errorf("expected action 'log' in shadow mode, got %s", decision.Action)
	}
}

func TestDisabledRuleSkipped(t *testing.T) {
	wafCfg := makeWAFConfig("enforce", config.WAFRule{
		ID:      "disabled-rule",
		Enabled: false,
		Phase:   "request",
		Match: config.WAFMatch{
			Any: []config.WAFCondition{
				{PathRegex: ".*"},
			},
		},
		Action: config.WAFAction{Type: "block", StatusCode: 403, Log: true},
	})

	engine, err := NewEngine(wafCfg, nil, 65536)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req, _ := http.NewRequest("GET", "http://example.com/anything", nil)
	decision := engine.Evaluate(req, nil)
	if decision.Matched {
		t.Error("disabled rule should not match")
	}
}

func TestBodyInspectionLimit(t *testing.T) {
	wafCfg := makeWAFConfig("enforce", config.WAFRule{
		ID:      "body-check",
		Enabled: true,
		Phase:   "request",
		Match: config.WAFMatch{
			Any: []config.WAFCondition{
				{BodyRegex: `(?i)(union\s+select)`},
			},
		},
		Action: config.WAFAction{Type: "block", StatusCode: 403, Log: true},
	})

	// Max inspect 10 bytes
	engine, err := NewEngine(wafCfg, nil, 10)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Body with payload beyond inspection limit
	body := make([]byte, 100)
	copy(body[50:], []byte("UNION SELECT"))

	req, _ := http.NewRequest("POST", "http://example.com/submit", nil)
	decision := engine.Evaluate(req, body)
	// Should NOT match because payload is beyond 10-byte inspection limit
	if decision.Matched {
		t.Error("should not match beyond inspection limit")
	}
}
