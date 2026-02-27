package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Upstreams []UpstreamConfig `yaml:"upstreams"`
	Routing  RoutingConfig  `yaml:"routing"`
	Security SecurityConfig `yaml:"security"`
	Cache    CacheConfig    `yaml:"cache"`
	AdminAPI AdminAPIConfig `yaml:"admin_api"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	ListenAddr              string `yaml:"listen_addr"`
	AdminListenAddr         string `yaml:"admin_listen_addr"`
	ReadTimeoutMS           int    `yaml:"read_timeout_ms"`
	WriteTimeoutMS          int    `yaml:"write_timeout_ms"`
	IdleTimeoutMS           int    `yaml:"idle_timeout_ms"`
	MaxHeaderBytes          int    `yaml:"max_header_bytes"`
	GracefulShutdownTimeoutMS int  `yaml:"graceful_shutdown_timeout_ms"`
}

func (s ServerConfig) ReadTimeout() time.Duration {
	return time.Duration(s.ReadTimeoutMS) * time.Millisecond
}

func (s ServerConfig) WriteTimeout() time.Duration {
	return time.Duration(s.WriteTimeoutMS) * time.Millisecond
}

func (s ServerConfig) IdleTimeout() time.Duration {
	return time.Duration(s.IdleTimeoutMS) * time.Millisecond
}

func (s ServerConfig) GracefulShutdownTimeout() time.Duration {
	return time.Duration(s.GracefulShutdownTimeoutMS) * time.Millisecond
}

// UpstreamConfig holds origin server settings.
type UpstreamConfig struct {
	Name                    string `yaml:"name"`
	BaseURL                 string `yaml:"base_url"`
	ConnectTimeoutMS        int    `yaml:"connect_timeout_ms"`
	ResponseHeaderTimeoutMS int    `yaml:"response_header_timeout_ms"`
	MaxIdleConns            int    `yaml:"max_idle_conns"`
	MaxIdleConnsPerHost     int    `yaml:"max_idle_conns_per_host"`
}

func (u UpstreamConfig) ConnectTimeout() time.Duration {
	return time.Duration(u.ConnectTimeoutMS) * time.Millisecond
}

func (u UpstreamConfig) ResponseHeaderTimeout() time.Duration {
	return time.Duration(u.ResponseHeaderTimeoutMS) * time.Millisecond
}

// RoutingConfig holds routing settings.
type RoutingConfig struct {
	DefaultUpstream string `yaml:"default_upstream"`
}

// SecurityConfig holds security settings.
type SecurityConfig struct {
	TrustXForwardedFor  bool `yaml:"trust_x_forwarded_for"`
	MaxBodyInspectBytes int  `yaml:"max_body_inspect_bytes"`
	MaxRequestBodyBytes int  `yaml:"max_request_body_bytes"`
}

// CacheConfig holds cache settings.
type CacheConfig struct {
	Enabled                bool `yaml:"enabled"`
	MaxEntries             int  `yaml:"max_entries"`
	MaxBytes               int  `yaml:"max_bytes"`
	DefaultTTLSeconds      int  `yaml:"default_ttl_seconds"`
	RespectOriginCC        bool `yaml:"respect_origin_cache_control"`
	BypassOnCookie         bool `yaml:"bypass_on_cookie"`
	BypassOnAuthorization  bool `yaml:"bypass_on_authorization"`
	NeverStoreOnSetCookie  bool `yaml:"never_store_on_set_cookie"`
	DebugHeaders           bool `yaml:"debug_headers"`
}

func (c CacheConfig) DefaultTTL() time.Duration {
	return time.Duration(c.DefaultTTLSeconds) * time.Second
}

// AdminAPIConfig holds admin API settings.
type AdminAPIConfig struct {
	Enabled   bool   `yaml:"enabled"`
	AuthToken string `yaml:"auth_token"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// WAFConfig holds WAF rules configuration.
type WAFConfig struct {
	Mode  string    `yaml:"mode"`
	Rules []WAFRule `yaml:"rules"`
}

// WAFRule represents a single WAF rule.
type WAFRule struct {
	ID          string    `yaml:"id"`
	Enabled     bool      `yaml:"enabled"`
	Phase       string    `yaml:"phase"`
	Description string    `yaml:"description"`
	Match       WAFMatch  `yaml:"match"`
	Action      WAFAction `yaml:"action"`
}

// WAFMatch holds matching conditions.
type WAFMatch struct {
	Any []WAFCondition `yaml:"any"`
}

// WAFCondition is a single match condition.
type WAFCondition struct {
	PathRegex      string `yaml:"path_regex,omitempty"`
	QueryRegex     string `yaml:"query_regex,omitempty"`
	BodyRegex      string `yaml:"body_regex,omitempty"`
	HeaderName     string `yaml:"header_name,omitempty"`
	HeaderRegex    string `yaml:"header_regex,omitempty"`
	IPCIDR         string `yaml:"ip_cidr,omitempty"`
	Method         string `yaml:"method,omitempty"`
	UserAgentRegex string `yaml:"user_agent_regex,omitempty"`
	ContentType    string `yaml:"content_type,omitempty"`
}

// WAFAction holds the action for a matched rule.
type WAFAction struct {
	Type       string `yaml:"type"`
	StatusCode int    `yaml:"status_code,omitempty"`
	Log        bool   `yaml:"log"`
}

// CacheRulesConfig holds cache rules.
type CacheRulesConfig struct {
	Rules []CacheRule `yaml:"rules"`
}

// CacheRule is a single cache rule.
type CacheRule struct {
	ID         string         `yaml:"id"`
	PathPrefix string         `yaml:"path_prefix,omitempty"`
	PathExact  string         `yaml:"path_exact,omitempty"`
	PathRegex  string         `yaml:"path_regex,omitempty"`
	Methods    []string       `yaml:"methods,omitempty"`
	Cache      CacheRuleCache `yaml:"cache"`
}

// CacheRuleCache holds per-rule cache settings.
type CacheRuleCache struct {
	Eligible    bool `yaml:"eligible"`
	TTLSeconds  int  `yaml:"ttl_seconds,omitempty"`
}

func (c CacheRuleCache) TTL() time.Duration {
	return time.Duration(c.TTLSeconds) * time.Second
}

// RateLimitConfig holds rate limit rules.
type RateLimitConfig struct {
	Rules      []RateLimitRule      `yaml:"rules"`
	Exemptions RateLimitExemptions  `yaml:"exemptions"`
}

// RateLimitRule is a single rate limit rule.
type RateLimitRule struct {
	ID         string `yaml:"id"`
	Key        string `yaml:"key"`
	PathPrefix string `yaml:"path_prefix,omitempty"`
	Rate       int    `yaml:"rate"`
	Burst      int    `yaml:"burst"`
	Enabled    bool   `yaml:"enabled"`
}

// RateLimitExemptions holds exemption lists.
type RateLimitExemptions struct {
	CIDRs []string `yaml:"cidrs"`
	Paths []string `yaml:"paths"`
}

// Load loads the main config from a file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}
	applyDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

// LoadWAFConfig loads WAF rules from a file path.
func LoadWAFConfig(path string) (*WAFConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read WAF config %s: %w", path, err)
	}
	var cfg WAFConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse WAF config %s: %w", path, err)
	}
	return &cfg, nil
}

// LoadCacheRulesConfig loads cache rules from a file path.
func LoadCacheRulesConfig(path string) (*CacheRulesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cache rules %s: %w", path, err)
	}
	var cfg CacheRulesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse cache rules %s: %w", path, err)
	}
	return &cfg, nil
}

// LoadRateLimitConfig loads rate limit rules from a file path.
func LoadRateLimitConfig(path string) (*RateLimitConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rate limit config %s: %w", path, err)
	}
	var cfg RateLimitConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse rate limit config %s: %w", path, err)
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = ":8080"
	}
	if cfg.Server.AdminListenAddr == "" {
		cfg.Server.AdminListenAddr = ":8081"
	}
	if cfg.Server.ReadTimeoutMS == 0 {
		cfg.Server.ReadTimeoutMS = 5000
	}
	if cfg.Server.WriteTimeoutMS == 0 {
		cfg.Server.WriteTimeoutMS = 10000
	}
	if cfg.Server.IdleTimeoutMS == 0 {
		cfg.Server.IdleTimeoutMS = 60000
	}
	if cfg.Server.MaxHeaderBytes == 0 {
		cfg.Server.MaxHeaderBytes = 1 << 20 // 1 MiB
	}
	if cfg.Server.GracefulShutdownTimeoutMS == 0 {
		cfg.Server.GracefulShutdownTimeoutMS = 15000
	}
	if cfg.Security.MaxBodyInspectBytes == 0 {
		cfg.Security.MaxBodyInspectBytes = 65536
	}
	if cfg.Security.MaxRequestBodyBytes == 0 {
		cfg.Security.MaxRequestBodyBytes = 10 << 20 // 10 MiB
	}
	if cfg.Cache.MaxEntries == 0 {
		cfg.Cache.MaxEntries = 5000
	}
	if cfg.Cache.MaxBytes == 0 {
		cfg.Cache.MaxBytes = 256 << 20 // 256 MiB
	}
	if cfg.Cache.DefaultTTLSeconds == 0 {
		cfg.Cache.DefaultTTLSeconds = 30
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
}

func validate(cfg *Config) error {
	if len(cfg.Upstreams) == 0 {
		return fmt.Errorf("at least one upstream must be configured")
	}
	if cfg.Routing.DefaultUpstream == "" {
		return fmt.Errorf("routing.default_upstream is required")
	}
	found := false
	for _, u := range cfg.Upstreams {
		if u.Name == "" {
			return fmt.Errorf("upstream name is required")
		}
		if u.BaseURL == "" {
			return fmt.Errorf("upstream %s: base_url is required", u.Name)
		}
		if u.Name == cfg.Routing.DefaultUpstream {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("default upstream %q not found in upstreams", cfg.Routing.DefaultUpstream)
	}
	if cfg.AdminAPI.Enabled && cfg.AdminAPI.AuthToken == "" {
		return fmt.Errorf("admin_api.auth_token is required when admin_api is enabled")
	}
	return nil
}
