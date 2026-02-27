package config

import (
	"os"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	content := `
server:
  listen_addr: ":8080"
  admin_listen_addr: ":8081"
upstreams:
  - name: testsite
    base_url: "http://localhost:8000"
routing:
  default_upstream: "testsite"
admin_api:
  enabled: true
  auth_token: "test-token"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
	if cfg.Server.ListenAddr != ":8080" {
		t.Errorf("expected :8080, got %s", cfg.Server.ListenAddr)
	}
	if len(cfg.Upstreams) != 1 {
		t.Errorf("expected 1 upstream, got %d", len(cfg.Upstreams))
	}
}

func TestLoadInvalidConfigMissingUpstream(t *testing.T) {
	content := `
server:
  listen_addr: ":8080"
routing:
  default_upstream: "testsite"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	_, err = Load(f.Name())
	if err == nil {
		t.Fatal("expected error for missing upstream")
	}
}

func TestDefaultValues(t *testing.T) {
	content := `
upstreams:
  - name: origin
    base_url: "http://origin:8000"
routing:
  default_upstream: "origin"
admin_api:
  enabled: false
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.ListenAddr != ":8080" {
		t.Errorf("expected default :8080, got %s", cfg.Server.ListenAddr)
	}
	if cfg.Server.ReadTimeoutMS != 5000 {
		t.Errorf("expected default 5000, got %d", cfg.Server.ReadTimeoutMS)
	}
	if cfg.Cache.MaxEntries != 5000 {
		t.Errorf("expected default 5000, got %d", cfg.Cache.MaxEntries)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default info, got %s", cfg.Logging.Level)
	}
}

func TestMissingDefaultUpstream(t *testing.T) {
	content := `
upstreams:
  - name: origin
    base_url: "http://origin:8000"
routing:
  default_upstream: "nonexistent"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	_, err = Load(f.Name())
	if err == nil {
		t.Fatal("expected error for nonexistent default upstream")
	}
}
