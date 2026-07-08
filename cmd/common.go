package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/dtonair/confluence-cli/config"
	"github.com/dtonair/confluence-cli/confluence"
	"github.com/dtonair/confluence-cli/output"

	"github.com/spf13/cobra"
)

// testTransport, when non-nil, is injected into every client. Tests set this
// to a stub RoundTripper; it is always nil in production.
var testTransport http.RoundTripper

// envMap snapshots the process environment into a map for config loading.
func envMap() map[string]string {
	env := os.Environ()
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

// loadConfig resolves config from the environment and the YAML config file.
func loadConfig() (config.Config, error) {
	env := envMap()
	return config.LoadConfig(env, config.DefaultConfigPath(env))
}

// newClient loads config and builds a Confluence client.
func newClient() (config.Config, *confluence.Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return config.Config{}, nil, err
	}
	var opts []confluence.Option
	if testTransport != nil {
		opts = append(opts, confluence.WithHTTPClient(&http.Client{Transport: testTransport}))
	}
	return cfg, confluence.NewClient(cfg, opts...), nil
}

// fail renders a structured error to stderr and returns it so Execute exits 1.
func fail(err error) error {
	output.WriteError(os.Stderr, err)
	return err
}

// ctx returns the command's context (carries cancellation on Ctrl-C).
func ctx(cmd *cobra.Command) context.Context {
	return cmd.Context()
}

// isNumericID reports whether s is a non-empty run of ASCII digits.
func isNumericID(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseID validates a positional numeric id argument (page, comment, space id).
// Confluence ids can exceed int range, so it is kept as a string.
func parseID(label, arg string) (string, error) {
	s := strings.TrimSpace(arg)
	if !isNumericID(s) {
		return "", fmt.Errorf("invalid %s %q: must be a positive integer", label, arg)
	}
	return s, nil
}

// resolveSpaceID turns a --space value (or the configured default), which may be
// a space key or a numeric id, into a numeric space id. Keys are resolved via
// GET /spaces?keys={key}.
func resolveSpaceID(cmd *cobra.Command, client *confluence.Client, cfg config.Config) (string, error) {
	raw, err := config.ResolveSpace(flagSpace, cfg)
	if err != nil {
		return "", err
	}
	return spaceIDFor(cmd, client, raw)
}

// spaceIDFor resolves a single space key-or-id to a numeric id.
func spaceIDFor(cmd *cobra.Command, client *confluence.Client, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if isNumericID(raw) {
		return raw, nil
	}
	path := fmt.Sprintf("/spaces?keys=%s&limit=1", url.QueryEscape(raw))
	var resp struct {
		Results []map[string]any `json:"results"`
	}
	if err := client.Request(ctx(cmd), path, confluence.RequestOptions{}, &resp); err != nil {
		return "", err
	}
	if len(resp.Results) == 0 {
		return "", fmt.Errorf("no space found with key %q", raw)
	}
	if id, ok := resp.Results[0]["id"].(string); ok && id != "" {
		return id, nil
	}
	return "", fmt.Errorf("space %q has no id in the API response", raw)
}

// emitObject renders a single decoded JSON object as JSON, or its pretty
// summary when --pretty is set.
func emitObject(raw json.RawMessage, summary func(map[string]any) string) error {
	if flagPretty {
		m, err := toMap(raw)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, summary(m))
		return err
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	return output.RenderJSON(os.Stdout, v)
}

// emitList renders a list of raw JSON objects as a JSON array, or one summary
// line per item when --pretty is set.
func emitList(values []json.RawMessage, summary func(map[string]any) string, emptyMsg string) error {
	if flagPretty {
		lines := make([]string, 0, len(values))
		for _, raw := range values {
			m, err := toMap(raw)
			if err != nil {
				return err
			}
			lines = append(lines, summary(m))
		}
		return output.RenderLines(os.Stdout, lines, emptyMsg)
	}
	items := make([]any, 0, len(values))
	for _, raw := range values {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		items = append(items, v)
	}
	return output.RenderJSON(os.Stdout, items)
}

func toMap(raw json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}
