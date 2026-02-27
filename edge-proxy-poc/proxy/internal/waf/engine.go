package waf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/openflare/edge-proxy/internal/config"
	"github.com/openflare/edge-proxy/internal/observability"
)

var wafLogger = log.New(os.Stdout, "", 0)

// Decision represents a WAF decision for a request.
type Decision struct {
	RuleID  string
	Action  string // "allow", "block", "log", "bypass_cache"
	Matched bool
	Detail  string
}

// Engine is the WAF rule matching engine.
type Engine struct {
	mode    string // "shadow" or "enforce"
	rules   []compiledRule
	metrics *observability.Metrics
	maxBodyInspect int
}

type compiledRule struct {
	config     config.WAFRule
	conditions []compiledCondition
}

type compiledCondition struct {
	original config.WAFCondition
	pathRe   *regexp.Regexp
	queryRe  *regexp.Regexp
	bodyRe   *regexp.Regexp
	headerRe *regexp.Regexp
	uaRe     *regexp.Regexp
	ipNet    *net.IPNet
}

// NewEngine creates a new WAF engine from configuration.
func NewEngine(wafCfg *config.WAFConfig, metrics *observability.Metrics, maxBodyInspect int) (*Engine, error) {
	if wafCfg == nil {
		return &Engine{mode: "shadow", maxBodyInspect: maxBodyInspect, metrics: metrics}, nil
	}

	engine := &Engine{
		mode:           wafCfg.Mode,
		metrics:        metrics,
		maxBodyInspect: maxBodyInspect,
	}

	for _, rule := range wafCfg.Rules {
		if !rule.Enabled {
			continue
		}

		cr := compiledRule{config: rule}

		for _, cond := range rule.Match.Any {
			cc, err := compileCondition(cond)
			if err != nil {
				return nil, fmt.Errorf("compile rule %s: %w", rule.ID, err)
			}
			cr.conditions = append(cr.conditions, cc)
		}

		engine.rules = append(engine.rules, cr)
	}

	return engine, nil
}

func compileCondition(cond config.WAFCondition) (compiledCondition, error) {
	cc := compiledCondition{original: cond}
	var err error

	if cond.PathRegex != "" {
		cc.pathRe, err = regexp.Compile(cond.PathRegex)
		if err != nil {
			return cc, fmt.Errorf("compile path_regex %q: %w", cond.PathRegex, err)
		}
	}

	if cond.QueryRegex != "" {
		cc.queryRe, err = regexp.Compile(cond.QueryRegex)
		if err != nil {
			return cc, fmt.Errorf("compile query_regex %q: %w", cond.QueryRegex, err)
		}
	}

	if cond.BodyRegex != "" {
		cc.bodyRe, err = regexp.Compile(cond.BodyRegex)
		if err != nil {
			return cc, fmt.Errorf("compile body_regex %q: %w", cond.BodyRegex, err)
		}
	}

	if cond.HeaderRegex != "" {
		cc.headerRe, err = regexp.Compile(cond.HeaderRegex)
		if err != nil {
			return cc, fmt.Errorf("compile header_regex %q: %w", cond.HeaderRegex, err)
		}
	}

	if cond.UserAgentRegex != "" {
		cc.uaRe, err = regexp.Compile(cond.UserAgentRegex)
		if err != nil {
			return cc, fmt.Errorf("compile user_agent_regex %q: %w", cond.UserAgentRegex, err)
		}
	}

	if cond.IPCIDR != "" {
		_, cc.ipNet, err = net.ParseCIDR(cond.IPCIDR)
		if err != nil {
			return cc, fmt.Errorf("parse ip_cidr %q: %w", cond.IPCIDR, err)
		}
	}

	return cc, nil
}

// Evaluate evaluates all WAF rules against a request.
// Returns the most restrictive decision.
func (e *Engine) Evaluate(r *http.Request, body []byte) *Decision {
	if len(e.rules) == 0 {
		return &Decision{Action: "allow", Matched: false}
	}

	clientIP := extractIP(r.RemoteAddr)

	for _, rule := range e.rules {
		matched, detail := e.matchRule(rule, r, body, clientIP)
		if !matched {
			continue
		}

		decision := &Decision{
			RuleID:  rule.config.ID,
			Action:  rule.config.Action.Type,
			Matched: true,
			Detail:  detail,
		}

		// Log the decision
		if rule.config.Action.Log {
			e.logDecision(r, decision)
		}

		// Record metric
		if e.metrics != nil {
			e.metrics.WAFDecisions.WithLabelValues(rule.config.ID, rule.config.Action.Type).Inc()
		}

		switch rule.config.Action.Type {
		case "allow":
			return decision
		case "block":
			if e.mode == "enforce" {
				return decision
			}
			// Shadow mode: log but don't block
			decision.Action = "log"
			return decision
		case "log":
			// Log action: record but don't block
			return decision
		case "bypass_cache":
			return decision
		}
	}

	return &Decision{Action: "allow", Matched: false}
}

// HandleBlock writes a block response.
func (e *Engine) HandleBlock(w http.ResponseWriter, decision *Decision, rule config.WAFRule) {
	statusCode := rule.Action.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusForbidden
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "blocked by WAF",
		"rule_id": decision.RuleID,
	})
}

// GetBlockStatusCode returns the status code for a blocking rule.
func (e *Engine) GetBlockStatusCode(ruleID string) int {
	for _, rule := range e.rules {
		if rule.config.ID == ruleID {
			if rule.config.Action.StatusCode > 0 {
				return rule.config.Action.StatusCode
			}
			return http.StatusForbidden
		}
	}
	return http.StatusForbidden
}

// Mode returns the WAF mode.
func (e *Engine) Mode() string {
	return e.mode
}

func (e *Engine) matchRule(rule compiledRule, r *http.Request, body []byte, clientIP net.IP) (bool, string) {
	// ANY = OR between conditions
	for _, cond := range rule.conditions {
		if matched, detail := e.matchCondition(cond, r, body, clientIP); matched {
			return true, detail
		}
	}
	return false, ""
}

func (e *Engine) matchCondition(cond compiledCondition, r *http.Request, body []byte, clientIP net.IP) (bool, string) {
	if cond.pathRe != nil {
		if cond.pathRe.MatchString(r.URL.Path) {
			return true, "path_regex matched: " + r.URL.Path
		}
		// Also check raw (undecoded) path
		if cond.pathRe.MatchString(r.URL.RawPath) {
			return true, "raw_path_regex matched: " + r.URL.RawPath
		}
	}

	if cond.queryRe != nil {
		rawQuery := r.URL.RawQuery
		if cond.queryRe.MatchString(rawQuery) {
			return true, "query_regex matched"
		}
		// Also check URL-decoded query (handles + → space, %XX encoding)
		if decoded, err := url.QueryUnescape(rawQuery); err == nil && decoded != rawQuery {
			if cond.queryRe.MatchString(decoded) {
				return true, "query_regex matched"
			}
		}
	}

	if cond.bodyRe != nil && len(body) > 0 {
		inspectLen := len(body)
		if inspectLen > e.maxBodyInspect {
			inspectLen = e.maxBodyInspect
		}
		if cond.bodyRe.Match(body[:inspectLen]) {
			return true, "body_regex matched"
		}
	}

	if cond.original.Method != "" {
		if strings.EqualFold(r.Method, cond.original.Method) {
			return true, "method matched: " + r.Method
		}
	}

	if cond.uaRe != nil {
		ua := r.UserAgent()
		if cond.uaRe.MatchString(ua) {
			return true, "user_agent_regex matched"
		}
	}

	if cond.original.HeaderName != "" {
		val := r.Header.Get(cond.original.HeaderName)
		if val != "" {
			return true, "header_name matched: " + cond.original.HeaderName
		}
	}

	if cond.headerRe != nil {
		for name, values := range r.Header {
			for _, v := range values {
				combined := name + ":" + v
				if cond.headerRe.MatchString(combined) {
					return true, "header_regex matched: " + combined
				}
			}
		}
	}

	if cond.ipNet != nil && clientIP != nil {
		if cond.ipNet.Contains(clientIP) {
			return true, "ip_cidr matched: " + clientIP.String()
		}
	}

	if cond.original.ContentType != "" {
		ct := r.Header.Get("Content-Type")
		if strings.Contains(strings.ToLower(ct), strings.ToLower(cond.original.ContentType)) {
			return true, "content_type matched: " + ct
		}
	}

	return false, ""
}

func (e *Engine) logDecision(r *http.Request, decision *Decision) {
	entry := map[string]interface{}{
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"level":      "warn",
		"msg":        "waf_decision",
		"rule_id":    decision.RuleID,
		"action":     decision.Action,
		"detail":     decision.Detail,
		"method":     r.Method,
		"path":       r.URL.Path,
		"query":      r.URL.RawQuery,
		"client_ip":  r.RemoteAddr,
		"user_agent": r.UserAgent(),
	}
	data, _ := json.Marshal(entry)
	wafLogger.Println(string(data))
}

// ReadBodyForInspection reads the request body for WAF inspection,
// then restores it for downstream handlers.
func ReadBodyForInspection(r *http.Request, maxBytes int) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	limited := io.LimitReader(r.Body, int64(maxBytes)+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}

	// Restore body for downstream handlers
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Truncate if over limit
	if len(body) > maxBytes {
		body = body[:maxBytes]
	}

	return body, nil
}

func extractIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return net.ParseIP(remoteAddr)
	}
	return net.ParseIP(host)
}
