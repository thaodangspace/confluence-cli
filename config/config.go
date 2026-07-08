// Package config resolves Confluence Cloud credentials and defaults from, in
// order of precedence: environment variables, then a YAML config file
// (~/.config/confluence-cli.yaml by default). Unlike bitbucket-cli there is no
// git-remote auto-detection: Confluence is addressed by an explicit site.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds resolved Confluence credentials, the target site, and an
// optional default space.
type Config struct {
	Email        string
	APIToken     string
	Site         string // normalized host, e.g. "your-site.atlassian.net"
	DefaultSpace string // space key or numeric id
}

// FileConfig mirrors the YAML config file. All fields are optional and act as
// fallbacks for the corresponding environment variables.
type FileConfig struct {
	Email        string `yaml:"email,omitempty"`
	APIToken     string `yaml:"api_token,omitempty"`
	Site         string `yaml:"site,omitempty"`
	DefaultSpace string `yaml:"default_space,omitempty"`
}

// FileKeys are the keys settable in the config file, in display order.
var FileKeys = []string{"email", "api_token", "site", "default_space"}

func (fc *FileConfig) field(key string) (*string, error) {
	switch key {
	case "email":
		return &fc.Email, nil
	case "api_token":
		return &fc.APIToken, nil
	case "site":
		return &fc.Site, nil
	case "default_space":
		return &fc.DefaultSpace, nil
	default:
		return nil, fmt.Errorf("unknown config key %q (valid keys: %s)", key, strings.Join(FileKeys, ", "))
	}
}

// Get returns the stored value for key.
func (fc FileConfig) Get(key string) (string, error) {
	p, err := (&fc).field(key)
	if err != nil {
		return "", err
	}
	return *p, nil
}

// Set assigns value (trimmed) to key. The site value is normalized to a host.
func (fc *FileConfig) Set(key, value string) error {
	p, err := fc.field(key)
	if err != nil {
		return err
	}
	value = strings.TrimSpace(value)
	if key == "site" {
		value = NormalizeSite(value)
	}
	*p = value
	return nil
}

// NormalizeSite reduces a site value to a bare host. It accepts a full URL
// ("https://your-site.atlassian.net/wiki"), a host with a path, or a bare host,
// and returns just the host ("your-site.atlassian.net").
func NormalizeSite(site string) string {
	s := strings.TrimSpace(site)
	if s == "" {
		return ""
	}
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	// Drop any path component (e.g. "/wiki").
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// DefaultConfigPath returns the config file location, honoring CONFLUENCE_CONFIG
// from env, then XDG_CONFIG_HOME, falling back to ~/.config/confluence-cli.yaml.
func DefaultConfigPath(env map[string]string) string {
	if p := trimmed(env, "CONFLUENCE_CONFIG"); p != "" {
		return p
	}
	if dir := trimmed(env, "XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "confluence-cli.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "confluence-cli.yaml")
}

// LoadFileConfig reads and parses the YAML config at path. A missing file
// yields an empty FileConfig and no error; malformed YAML is an error. An empty
// path returns an empty FileConfig.
func LoadFileConfig(path string) (FileConfig, error) {
	if strings.TrimSpace(path) == "" {
		return FileConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, nil
		}
		return FileConfig{}, fmt.Errorf("read config file %s: %w", path, err)
	}
	var fc FileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return FileConfig{}, fmt.Errorf("parse config file %s: %w", path, err)
	}
	return fc, nil
}

// WriteFileConfig writes fc as YAML to path, creating parent directories. The
// file is written with 0600 permissions since it may hold an API token.
func WriteFileConfig(path string, fc FileConfig) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("no config file path")
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}
	data, err := yaml.Marshal(fc)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config file %s: %w", path, err)
	}
	return nil
}

// SetFileValue loads the config at path, sets key to value, and writes it back,
// preserving the file's other values.
func SetFileValue(path, key, value string) error {
	fc, err := LoadFileConfig(path)
	if err != nil {
		return err
	}
	if err := fc.Set(key, value); err != nil {
		return err
	}
	return WriteFileConfig(path, fc)
}

func trimmed(env map[string]string, key string) string {
	return strings.TrimSpace(env[key])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// LoadConfig builds a Config from the given environment map and the YAML config
// file at configPath (pass "" to skip). Precedence is env > file. Email, API
// token, and site are required; the default space is optional.
func LoadConfig(env map[string]string, configPath string) (Config, error) {
	file, err := LoadFileConfig(configPath)
	if err != nil {
		return Config{}, err
	}

	email := firstNonEmpty(env["CONFLUENCE_EMAIL"], file.Email)
	apiToken := firstNonEmpty(env["CONFLUENCE_API_TOKEN"], file.APIToken)
	site := NormalizeSite(firstNonEmpty(env["CONFLUENCE_SITE"], file.Site))

	if email == "" || apiToken == "" || site == "" {
		return Config{}, fmt.Errorf("Set CONFLUENCE_EMAIL, CONFLUENCE_API_TOKEN, and CONFLUENCE_SITE (via environment or %s) before using confluence-cli.", configHint(configPath))
	}

	space := firstNonEmpty(env["CONFLUENCE_DEFAULT_SPACE"], file.DefaultSpace)

	return Config{
		Email:        email,
		APIToken:     apiToken,
		Site:         site,
		DefaultSpace: space,
	}, nil
}

func configHint(path string) string {
	if strings.TrimSpace(path) == "" {
		return "a config file"
	}
	return path
}

// ResolveSpace returns the explicit space input if non-empty, otherwise the
// configured default. It errors when neither yields a value.
func ResolveSpace(input string, cfg Config) (string, error) {
	space := strings.TrimSpace(input)
	if space == "" {
		space = strings.TrimSpace(cfg.DefaultSpace)
	}
	if space == "" {
		return "", fmt.Errorf("Provide a space via --space, or set CONFLUENCE_DEFAULT_SPACE.")
	}
	return space, nil
}
