package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeSite(t *testing.T) {
	cases := map[string]string{
		"acme.atlassian.net":                 "acme.atlassian.net",
		"https://acme.atlassian.net":         "acme.atlassian.net",
		"https://acme.atlassian.net/wiki":    "acme.atlassian.net",
		"http://acme.atlassian.net/wiki/foo": "acme.atlassian.net",
		"  acme.atlassian.net/  ":            "acme.atlassian.net",
		"":                                   "",
	}
	for in, want := range cases {
		if got := NormalizeSite(in); got != want {
			t.Fatalf("NormalizeSite(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadConfigEnvWins(t *testing.T) {
	env := map[string]string{
		"CONFLUENCE_EMAIL":         "env@example.com",
		"CONFLUENCE_API_TOKEN":     "env-token",
		"CONFLUENCE_SITE":          "https://env-site.atlassian.net/wiki",
		"CONFLUENCE_DEFAULT_SPACE": "ENG",
	}
	cfg, err := LoadConfig(env, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Email != "env@example.com" || cfg.APIToken != "env-token" {
		t.Fatalf("unexpected creds: %+v", cfg)
	}
	if cfg.Site != "env-site.atlassian.net" {
		t.Fatalf("site not normalized: %q", cfg.Site)
	}
	if cfg.DefaultSpace != "ENG" {
		t.Fatalf("unexpected default space: %q", cfg.DefaultSpace)
	}
}

func TestLoadConfigMissingRequiredFails(t *testing.T) {
	// No site provided.
	env := map[string]string{
		"CONFLUENCE_EMAIL":     "e@example.com",
		"CONFLUENCE_API_TOKEN": "t",
	}
	if _, err := LoadConfig(env, ""); err == nil {
		t.Fatal("expected error when site is missing")
	}
}

func TestFileConfigRoundTripAndRedaction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "confluence-cli.yaml")

	if err := SetFileValue(path, "email", "file@example.com"); err != nil {
		t.Fatalf("set email: %v", err)
	}
	if err := SetFileValue(path, "api_token", "secret-token"); err != nil {
		t.Fatalf("set token: %v", err)
	}
	if err := SetFileValue(path, "site", "https://file-site.atlassian.net/wiki"); err != nil {
		t.Fatalf("set site: %v", err)
	}

	fc, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if fc.Email != "file@example.com" || fc.APIToken != "secret-token" {
		t.Fatalf("unexpected file config: %+v", fc)
	}
	// site should have been normalized on write.
	if fc.Site != "file-site.atlassian.net" {
		t.Fatalf("site not normalized on set: %q", fc.Site)
	}

	// File must be 0600 since it holds a token.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config file perm = %o, want 600", perm)
	}
}

func TestResolveSpace(t *testing.T) {
	cfg := Config{DefaultSpace: "ENG"}
	if got, _ := ResolveSpace("DOCS", cfg); got != "DOCS" {
		t.Fatalf("explicit override failed: %q", got)
	}
	if got, _ := ResolveSpace("", cfg); got != "ENG" {
		t.Fatalf("default fallback failed: %q", got)
	}
	if _, err := ResolveSpace("", Config{}); err == nil {
		t.Fatal("expected error when no space available")
	}
}
