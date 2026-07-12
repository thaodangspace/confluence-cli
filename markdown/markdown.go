// Package markdown converts Confluence rendered-view HTML into GitHub-Flavored
// Markdown and, optionally, prepends a YAML frontmatter block. It is a pure-Go,
// network-free core so it can be unit-tested independently of the CLI.
package markdown

import (
	"fmt"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"gopkg.in/yaml.v3"
)

// FromHTML converts an HTML fragment (Confluence's rendered `view` body) into
// GitHub-Flavored Markdown. The GFM plugin adds tables, strikethrough, and task
// lists on top of the base rules. Empty input yields an empty string (an empty
// page is not an error). The returned Markdown is trimmed of surrounding
// whitespace; callers own the final newline policy.
func FromHTML(html string) (string, error) {
	if strings.TrimSpace(html) == "" {
		return "", nil
	}
	conv := md.NewConverter("", true, nil)
	conv.Use(plugin.GitHubFlavored())
	out, err := conv.ConvertString(html)
	if err != nil {
		return "", fmt.Errorf("convert html to markdown: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// Render composes an optional YAML frontmatter block with a Markdown body. When
// fm is empty, body is returned unchanged. Otherwise keys are emitted in the
// given order; keys absent from fm or holding an empty/nil value are skipped.
// Values are marshaled via yaml.v3 so quoting and escaping are correct.
func Render(body string, fm map[string]any, order []string) string {
	lines := make([]string, 0, len(order))
	for _, k := range order {
		v, ok := fm[k]
		if !ok || isEmpty(v) {
			continue
		}
		lines = append(lines, renderField(k, v))
	}
	if len(lines) == 0 {
		return body
	}
	var b strings.Builder
	b.WriteString("---\n")
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("---\n\n")
	b.WriteString(body)
	return b.String()
}

// renderField marshals a single "key: value" line, delegating scalar quoting to
// yaml.v3 by marshaling a one-key map and trimming the trailing newline.
func renderField(key string, value any) string {
	out, err := yaml.Marshal(map[string]any{key: value})
	if err != nil {
		// yaml.Marshal only fails on unrepresentable types; fall back to a
		// plain rendering rather than dropping the field silently.
		return fmt.Sprintf("%s: %v", key, value)
	}
	return strings.TrimRight(string(out), "\n")
}

// isEmpty reports whether a frontmatter value should be omitted: nil or an empty
// string. Numeric zero and false are considered present (a page can legitimately
// be version 0 in theory, and callers only pass meaningful values).
func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}
